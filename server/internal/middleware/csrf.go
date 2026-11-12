package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// SameOrigin protects the app's forms against cross-site request forgery.
//
// Every cookie the request sets is SameSite=Lax, so other sites' forms and
// scripts don't send the login cookie along. As a second line, a request
// that changes something (anything but GET, HEAD and OPTIONS) is refused
// when the browser says it came from another site: an Origin header naming
// another host, or Sec-Fetch-Site: cross-site. Requests without either
// header (old browsers, curl) pass; SameSite still covers them.
func SameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetSameSite(http.SameSiteLaxMode)

		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if crossSite(c.Request) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func crossSite(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return true // includes "null", sent from sandboxed or privacy-sensitive contexts
		}
		return !strings.EqualFold(u.Host, r.Host)
	}
	return r.Header.Get("Sec-Fetch-Site") == "cross-site"
}
