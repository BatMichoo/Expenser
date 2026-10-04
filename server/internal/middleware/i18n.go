package middleware

import (
	"expenser/internal/services"

	"github.com/gin-gonic/gin"
)

// I18nMiddleware determines the language for the request.
func I18nMiddleware(as *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		const cookieKey = "lang"
		lang, err := c.Cookie(cookieKey)
		if err == nil && lang != "" {
			c.Set(cookieKey, lang)
			c.Next()
			return
		}

		token, err := as.ValidateToken(c)
		if err == nil && token.Claims.PreferredLanguage != "" {
			c.Set(cookieKey, token.Claims.PreferredLanguage)
			c.Next()
			return
		}

		const defaultLanguage = "en"
		const duration = 365 * 24 * 60
		const secure = false
		const httpOnly = true

		c.SetCookie(cookieKey, defaultLanguage, duration, "/", as.Domain, secure, httpOnly)
		c.Set(cookieKey, defaultLanguage)
		c.Next()
	}
}
