package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func sameOriginRecorder(method string, headers map[string]string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOrigin())
	r.Handle(method, "/x", func(c *gin.Context) {
		c.SetCookie("k", "v", 60, "/", "", true, true)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(method, "https://expenser.lan/x", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"same-origin post", http.MethodPost, map[string]string{"Origin": "https://expenser.lan"}, http.StatusOK},
		{"origin host case", http.MethodPost, map[string]string{"Origin": "https://EXPENSER.lan"}, http.StatusOK},
		{"no headers (curl, old browsers)", http.MethodPost, nil, http.StatusOK},
		{"same-site fetch without origin", http.MethodDelete, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"other site's form", http.MethodPost, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"other site's delete", http.MethodDelete, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"null origin", http.MethodPut, map[string]string{"Origin": "null"}, http.StatusForbidden},
		{"cross-site fetch without origin", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"cross-site GET is a link, allowed", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameOriginRecorder(tt.method, tt.headers).Code)
		})
	}
}

func TestSameOriginMakesCookiesLax(t *testing.T) {
	w := sameOriginRecorder(http.MethodGet, nil)
	assert.Contains(t, w.Header().Get("Set-Cookie"), "SameSite=Lax")
}
