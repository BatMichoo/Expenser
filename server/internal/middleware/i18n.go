package middleware

import (
	"expenser/internal/services"
	"github.com/gin-gonic/gin"
)

// I18nMiddleware determines the language for the request.
func I18nMiddleware(as *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Check for 'lang' cookie (explicit selection takes precedence)
		lang, err := c.Cookie("lang")
		if err == nil && lang != "" {
			c.Set("lang", lang)
			c.Next()
			return
		}

		// 2. Check for user preference in JWT claims (even on public pages)
		tokenString, err := c.Cookie("auth_token")
		if err == nil && tokenString != "" {
			token, err := as.ValidateToken(tokenString)
			if err == nil && token.Claims.PreferredLanguage != "" {
				c.Set("lang", token.Claims.PreferredLanguage)
				c.Next()
				return
			}
		}

		// 3. Fallback to default
		c.Set("lang", "en")
		c.Next()
	}
}
