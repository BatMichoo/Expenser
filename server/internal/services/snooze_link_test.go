package services

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnoozeLinkRoundTrip(t *testing.T) {
	s := NewSnoozeSigner("jwt-secret")
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	claim := SnoozeClaim{BillID: uuid.New(), DueFor: day(2026, 11, 20), Until: day(2026, 11, 14), Expires: now.Add(14 * 24 * time.Hour)}

	token := s.Sign(claim)
	assert.NotContains(t, token, "/", "safe in a URL path")
	assert.Len(t, token, 30+1+16, "short enough for the plain-text email")
	got, err := s.Verify(token, now)
	require.NoError(t, err)
	assert.Equal(t, claim.BillID, got.BillID)
	assert.True(t, got.DueFor.Equal(claim.DueFor))
	assert.True(t, got.Until.Equal(claim.Until))
}

func TestSnoozeLinkRefusals(t *testing.T) {
	s := NewSnoozeSigner("jwt-secret")
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	claim := SnoozeClaim{BillID: uuid.New(), DueFor: day(2026, 11, 20), Until: day(2026, 11, 14), Expires: now}
	token := s.Sign(claim)

	_, err := s.Verify(token, now.Add(14*time.Hour))
	assert.NoError(t, err, "valid through the whole expiry day")
	_, err = s.Verify(token, now.Add(24*time.Hour))
	assert.ErrorIs(t, err, ErrSnoozeLinkExpired)

	_, err = NewSnoozeSigner("other-secret").Verify(token, now)
	assert.ErrorIs(t, err, ErrSnoozeLinkInvalid, "signed with another key")

	payload, sig, _ := strings.Cut(token, ".")
	tampered := strings.ToUpper(payload[:1]) + payload[1:] + "." + sig
	if tampered != token {
		_, err = s.Verify(tampered, now)
		assert.ErrorIs(t, err, ErrSnoozeLinkInvalid, "payload changed")
	}
	for _, bad := range []string{"", "abc", "a.b", "..", token + "x"} {
		_, err = s.Verify(bad, now)
		assert.ErrorIs(t, err, ErrSnoozeLinkInvalid, bad)
	}
}
