package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig is how to reach the mail server. For Gmail: host
// smtp.gmail.com, port 587, the Gmail address as user and From, and an app
// password (Google account > Security > 2-Step Verification > App passwords).
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// Configured reports whether there is enough to send mail.
func (c SMTPConfig) Configured() bool {
	return c.Host != "" && c.From != ""
}

// SMTPMailer sends email over SMTP with STARTTLS and PLAIN auth.
type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	return &SMTPMailer{cfg: cfg}
}

// defaultSendTimeout bounds one send when the context has no deadline.
const defaultSendTimeout = 30 * time.Second

func (m *SMTPMailer) Send(ctx context.Context, msg EmailMessage) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultSendTimeout)
		defer cancel()
	}
	deadline, _ := ctx.Deadline()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(m.cfg.Host, m.cfg.Port))
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	// Every read and write after this is bounded by the same deadline, and
	// cancelling the context closes the connection.
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if m.cfg.User != "" {
		// PlainAuth refuses to send the password without TLS, except to
		// localhost.
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	from, err := mail.ParseAddress(m.cfg.From)
	if err != nil {
		return fmt.Errorf("smtp from address %q: %w", m.cfg.From, err)
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp to %s: %w", msg.To, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	body, err := buildMIME(from, msg)
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	if err := c.Quit(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

// buildMIME renders a multipart/alternative message (plain text + HTML),
// UTF-8, quoted-printable, with an encoded subject.
func buildMIME(from *mail.Address, msg EmailMessage) ([]byte, error) {
	boundary, err := randomToken()
	if err != nil {
		return nil, err
	}
	id, err := randomToken()
	if err != nil {
		return nil, err
	}
	domain := "localhost"
	if at := strings.LastIndex(from.Address, "@"); at >= 0 {
		domain = from.Address[at+1:]
	}

	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	fromHeader := from.Address
	if from.Name != "" {
		fromHeader = from.String() // encodes non-ASCII names
	}
	header("From", fromHeader)
	header("To", msg.To)
	header("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", fmt.Sprintf("<%s@%s>", id, domain))
	header("MIME-Version", "1.0")
	header("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")

	for _, part := range []struct{ kind, body string }{{"text/plain", msg.Text}, {"text/html", msg.HTML}} {
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		header("Content-Type", part.kind+"; charset=UTF-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		qp := quotedprintable.NewWriter(&b)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), nil
}

func randomToken() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
