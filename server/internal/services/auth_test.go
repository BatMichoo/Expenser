package services

import (
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
