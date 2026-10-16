package middleware

import (
	"expenser/internal/services"

	"github.com/gin-gonic/gin"
)

const LanguageCookieKey = "lang"

// I18nMiddleware determines the language for the request.
func I18nMiddleware(as *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang, err := c.Cookie(LanguageCookieKey)
		if err == nil && lang != "" {
			c.Set(LanguageCookieKey, lang)
			c.Next()
			return
		}

		claims := services.ClaimsFromContext(c)
		if claims != nil && claims.PreferredLanguage != "" {
			c.Set(LanguageCookieKey, claims.PreferredLanguage)
			c.Next()
			return
		}

		const language = "en"

		const duration = 365 * 24 * 60
		const secure = false
		const httpOnly = true

		c.SetCookie(LanguageCookieKey, language, duration, "/", as.Domain, secure, httpOnly)
		c.Set(LanguageCookieKey, language)
		c.Next()
	}
}
