package services

import (
	"bufio"
	"context"
	"encoding/base64"
	"mime"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSMTP is a minimal SMTP server on localhost that records one message.
type fakeSMTP struct {
	addr     string
	mu       sync.Mutex
	auth     string
	from     string
	rcpt     []string
	data     string
	rejectTo bool
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &fakeSMTP{addr: ln.Addr().String()}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	tp.PrintfLine("220 fake ready")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			tp.PrintfLine("250-fake")
			tp.PrintfLine("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			f.mu.Lock()
			f.auth = strings.TrimSpace(line[len("AUTH PLAIN"):])
			f.mu.Unlock()
			tp.PrintfLine("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			f.mu.Lock()
			f.from = line
			f.mu.Unlock()
			tp.PrintfLine("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			if f.rejectTo {
				tp.PrintfLine("550 no such user")
				continue
			}
			f.mu.Lock()
			f.rcpt = append(f.rcpt, line)
			f.mu.Unlock()
			tp.PrintfLine("250 ok")
		case cmd == "DATA":
			tp.PrintfLine("354 go")
			r := bufio.NewReader(tp.DotReader())
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				b.WriteString(l)
				if err != nil {
					break
				}
			}
			f.mu.Lock()
			f.data = b.String()
			f.mu.Unlock()
			tp.PrintfLine("250 queued")
		case cmd == "QUIT":
			tp.PrintfLine("221 bye")
			return
		default:
			tp.PrintfLine("250 ok")
		}
	}
}

func TestSMTPMailerSends(t *testing.T) {
	srv := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, User: "app@gmail.com", Password: "app-pass", From: "app@gmail.com"})

	err := m.Send(context.Background(), EmailMessage{
		To: "me@example.com", Subject: "Напомняне: Газ — чака 3 дни", Text: "Здравей,\nplain body", HTML: "<p>html body</p>",
	})
	require.NoError(t, err)

	srv.mu.Lock()
	defer srv.mu.Unlock()
	creds, _ := base64.StdEncoding.DecodeString(srv.auth)
	assert.Equal(t, "\x00app@gmail.com\x00app-pass", string(creds), "logs in with the app password")
	assert.Contains(t, srv.from, "<app@gmail.com>")
	assert.Equal(t, []string{"RCPT TO:<me@example.com>"}, srv.rcpt)

	assert.Contains(t, srv.data, "From: app@gmail.com")
	assert.Contains(t, srv.data, "To: me@example.com")
	assert.Contains(t, srv.data, "multipart/alternative")
	assert.Contains(t, srv.data, "text/plain; charset=UTF-8")
	assert.Contains(t, srv.data, "text/html; charset=UTF-8")
	var subject string
	for _, l := range strings.Split(srv.data, "\n") { // DotReader normalises line endings
		if strings.HasPrefix(l, "Subject: ") {
			subject = strings.TrimPrefix(l, "Subject: ")
		}
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(subject)
	require.NoError(t, err)
	assert.Equal(t, "Напомняне: Газ — чака 3 дни", decoded, "non-ASCII subject is encoded")
}

func TestSMTPMailerReportsRejection(t *testing.T) {
	srv := startFakeSMTP(t)
	srv.rejectTo = true
	host, port, _ := net.SplitHostPort(srv.addr)
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, User: "app@gmail.com", Password: "x", From: "app@gmail.com"})
	err := m.Send(context.Background(), EmailMessage{To: "nobody@example.com", Subject: "s", Text: "t", HTML: "h"})
	assert.ErrorContains(t, err, "550")
}

func TestSMTPMailerTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() { // accepts but never greets
		c, err := ln.Accept()
		if err == nil {
			time.Sleep(2 * time.Second)
			c.Close()
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "a@b.co"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = m.Send(ctx, EmailMessage{To: "me@example.com", Subject: "s", Text: "t", HTML: "h"})
	assert.Error(t, err)
	assert.Less(t, time.Since(start), time.Second, "a stuck server must not hang the scheduler")
}

func TestSMTPMailerFromWithName(t *testing.T) {
	srv := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(srv.addr)
	m := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "Expenser <app@gmail.com>"})
	require.NoError(t, m.Send(context.Background(), EmailMessage{To: "me@example.com", Subject: "s", Text: "t", HTML: "h"}))
	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Equal(t, "MAIL FROM:<app@gmail.com>", srv.from, "envelope uses the bare address")
	assert.Contains(t, srv.data, `From: "Expenser" <app@gmail.com>`)
}
