package services

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsRefreshAfterHalfTheLifetime(t *testing.T) {
	as := NewAuthService("secret", 24*time.Hour, "localhost")
	now := time.Now()

	assert.False(t, as.NeedsRefresh(&Token{ExpiresAt: now.Add(23 * time.Hour)}, now), "fresh login")
	assert.False(t, as.NeedsRefresh(&Token{ExpiresAt: now.Add(13 * time.Hour)}, now))
	assert.True(t, as.NeedsRefresh(&Token{ExpiresAt: now.Add(11 * time.Hour)}, now))
	assert.True(t, as.NeedsRefresh(&Token{ExpiresAt: now.Add(30 * time.Minute)}, now))
}

func TestRefreshExtendsTheTokenItself(t *testing.T) {
	as := NewAuthService("secret", 24*time.Hour, "localhost")
	user := &models.User{ID: uuid.New(), Username: "ana", PreferredLanguage: "bg"}
	old, err := as.GenerateToken(user)
	require.NoError(t, err)
	// Pretend most of the lifetime has passed.
	old.ExpiresAt = time.Now().Add(time.Hour)

	fresh, err := as.Refresh(old)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), fresh.ExpiresAt, time.Minute)

	// The new token is valid and carries the same user.
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(&http.Cookie{Name: authCookieName, Value: fresh.Value})
	parsed, err := as.ValidateToken(c)
	require.NoError(t, err)
	assert.Equal(t, user.ID, parsed.Claims.UserID)
	assert.Equal(t, "ana", parsed.Claims.Username)
	assert.Equal(t, "bg", parsed.Claims.PreferredLanguage)
	assert.WithinDuration(t, fresh.ExpiresAt, parsed.ExpiresAt, time.Second)
}

type fakeVersions map[uuid.UUID]int

func (f fakeVersions) TokenVersion(_ context.Context, id uuid.UUID) (int, error) {
	v, ok := f[id]
	if !ok {
		return 0, sql.ErrNoRows
	}
	return v, nil
}

func validateCookie(as *AuthService, value string) (*Token, error) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(&http.Cookie{Name: authCookieName, Value: value})
	return as.ValidateToken(c)
}

func TestValidateTokenChecksTheTokenVersion(t *testing.T) {
	as := NewAuthService("secret", time.Hour, "localhost")
	user := &models.User{ID: uuid.New(), Username: "ana", TokenVersion: 3}
	versions := fakeVersions{user.ID: 3}
	as.CheckTokenVersions(versions)

	token, err := as.GenerateToken(user)
	require.NoError(t, err)
	_, err = validateCookie(as, token.Value)
	assert.NoError(t, err, "current version")

	versions[user.ID] = 4 // the password changed
	_, err = validateCookie(as, token.Value)
	assert.ErrorIs(t, err, ErrLoginRevoked)

	delete(versions, user.ID) // the user is gone
	_, err = validateCookie(as, token.Value)
	assert.Error(t, err)
}

func TestTokensFromBeforeVersionsStillWork(t *testing.T) {
	// Tokens issued before the version claim existed have none (0), the
	// default version of every user.
	as := NewAuthService("secret", time.Hour, "localhost")
	user := &models.User{ID: uuid.New(), Username: "ana"}
	as.CheckTokenVersions(fakeVersions{user.ID: 0})
	token, err := as.GenerateToken(user)
	require.NoError(t, err)
	_, err = validateCookie(as, token.Value)
	assert.NoError(t, err)
}

func TestRefreshKeepsTheTokenVersion(t *testing.T) {
	as := NewAuthService("secret", time.Hour, "localhost")
	user := &models.User{ID: uuid.New(), Username: "ana", TokenVersion: 2}
	old, err := as.GenerateToken(user)
	require.NoError(t, err)
	fresh, err := as.Refresh(old)
	require.NoError(t, err)
	assert.Equal(t, 2, fresh.Claims.TokenVersion)
}
