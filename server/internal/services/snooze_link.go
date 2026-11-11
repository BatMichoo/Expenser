package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSnoozeLinkInvalid = errors.New("snooze link is not valid")
	ErrSnoozeLinkExpired = errors.New("snooze link has expired")
)

// SnoozeClaim is what a snooze link in a reminder email allows: pausing
// one bill's reminders until Until, for the cycle due on DueFor, through
// the day Expires. Dates are whole days (UTC).
type SnoozeClaim struct {
	BillID  uuid.UUID
	DueFor  time.Time
	Until   time.Time
	Expires time.Time
}

// SnoozeSigner signs and checks snooze links, so they work without logging
// in but can't be made up or altered.
type SnoozeSigner struct {
	key []byte
}

// NewSnoozeSigner derives its key from the app's secret (the JWT secret),
// kept separate from the key used for logins.
func NewSnoozeSigner(secret string) *SnoozeSigner {
	sum := sha256.Sum256([]byte("snooze-link:" + secret))
	return &SnoozeSigner{key: sum[:]}
}

// A token is base64url(payload).base64url(mac), kept short for the plain-
// text email: the payload is the bill ID (16 bytes) and three dates as days
// since 1970 (2 bytes each, good until 2149); the HMAC-SHA256 is cut to 12
// bytes (96 bits), plenty against guessing over the network.
const (
	snoozePayloadLen = 16 + 3*2
	snoozeMACLen     = 12
	secondsPerDay    = 24 * 60 * 60
)

func toDay(t time.Time) uint16 {
	y, m, d := t.Date()
	return uint16(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / secondsPerDay)
}

func fromDay(n uint16) time.Time {
	return time.Unix(int64(n)*secondsPerDay, 0).UTC()
}

func (s *SnoozeSigner) mac(payload []byte) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write(payload)
	return m.Sum(nil)[:snoozeMACLen]
}

// Sign returns the token for a snooze link.
func (s *SnoozeSigner) Sign(c SnoozeClaim) string {
	p := make([]byte, 0, snoozePayloadLen)
	p = append(p, c.BillID[:]...)
	p = binary.BigEndian.AppendUint16(p, toDay(c.DueFor))
	p = binary.BigEndian.AppendUint16(p, toDay(c.Until))
	p = binary.BigEndian.AppendUint16(p, toDay(c.Expires))
	enc := base64.RawURLEncoding
	return enc.EncodeToString(p) + "." + enc.EncodeToString(s.mac(p))
}

// Verify checks a token's signature and expiry and returns its claim.
func (s *SnoozeSigner) Verify(token string, now time.Time) (*SnoozeClaim, error) {
	enc := base64.RawURLEncoding
	var p, sig []byte
	if n := enc.EncodedLen(snoozePayloadLen); len(token) == n+1+enc.EncodedLen(snoozeMACLen) && token[n] == '.' {
		var err1, err2 error
		p, err1 = enc.DecodeString(token[:n])
		sig, err2 = enc.DecodeString(token[n+1:])
		if err1 != nil || err2 != nil {
			p = nil
		}
	}
	if len(p) != snoozePayloadLen || !hmac.Equal(sig, s.mac(p)) {
		return nil, ErrSnoozeLinkInvalid
	}
	c := &SnoozeClaim{
		DueFor:  fromDay(binary.BigEndian.Uint16(p[16:])),
		Until:   fromDay(binary.BigEndian.Uint16(p[18:])),
		Expires: fromDay(binary.BigEndian.Uint16(p[20:])),
	}
	copy(c.BillID[:], p[:16])
	if toDay(now) > toDay(c.Expires) {
		return nil, ErrSnoozeLinkExpired
	}
	return c, nil
}
