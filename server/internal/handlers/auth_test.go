package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"expenser/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func logoutRecorder(t *testing.T, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{auth: services.NewAuthService("secret", time.Hour, "localhost")}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/logout", nil)
	if htmx {
		c.Request.Header.Set("HX-Request", "true")
	}
	assert.NoError(t, h.Logout(c))
	c.Writer.WriteHeaderNow()
	return w
}

func TestLogoutFromButton(t *testing.T) {
	w := logoutRecorder(t, true)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/", w.Header().Get("HX-Redirect"), "unchanged: HTMX takes the browser home")
	assert.NotEmpty(t, w.Header().Get("Set-Cookie"), "clears the login cookie")
}

func TestLogoutVisitedDirectly(t *testing.T) {
	w := logoutRecorder(t, false)
	assert.Equal(t, http.StatusSeeOther, w.Code, "a real redirect, not a blank page")
	assert.Equal(t, "/", w.Header().Get("Location"))
	assert.NotEmpty(t, w.Header().Get("Set-Cookie"), "clears the login cookie")
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "/",
		"/bills":                "/bills",
		"/house?month=2026-01":  "/house?month=2026-01",
		"//evil.example":        "/",
		"/\\evil.example":       "/",
		"https://evil.example/": "/",
		"bills":                 "/",
		"/login":                "/",
		"javascript:alert(1)":   "/",
	} {
		assert.Equal(t, want, safeNext(in), in)
	}
}
