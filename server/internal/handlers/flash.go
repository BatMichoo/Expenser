package handlers

import (
	"encoding/base64"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// An error on a full page load can't be swapped in as a popup, because there
// is no HTMX request. renderError stores it in this cookie and redirects home;
// the page then fetches it from /flash and shows it like any other popup.
const (
	flashCookieName = "flash_error"
	flashMaxAge     = 60 // seconds
	flashSetKey     = "flash_set"
)

type flashError struct {
	Title   string `json:"t"`
	Message string `json:"m"`
}

func setFlash(c *gin.Context, title, message string) {
	data, _ := json.Marshal(flashError{Title: title, Message: message})
	c.SetCookie(flashCookieName, base64.RawURLEncoding.EncodeToString(data), flashMaxAge, "/", "", false, true)
	c.Set(flashSetKey, true)
}

// hasFlash reports whether a flash error waits to be shown, either sent by
// the browser or set earlier in this request.
func hasFlash(c *gin.Context) bool {
	if c.GetBool(flashSetKey) {
		return true
	}
	value, err := c.Cookie(flashCookieName)
	return err == nil && value != ""
}

// popFlash returns the pending flash error and clears its cookie.
func popFlash(c *gin.Context) (flashError, bool) {
	var f flashError
	value, err := c.Cookie(flashCookieName)
	if err != nil || value == "" {
		return f, false
	}
	c.SetCookie(flashCookieName, "", -1, "/", "", false, true)

	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(data, &f) != nil {
		return f, false
	}
	return f, true
}
