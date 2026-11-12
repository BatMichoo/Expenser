package middleware

import (
	"expenser/internal/services"

	"github.com/gin-gonic/gin"
)

const LanguageCookieKey = "lang"

// languageCookieMaxAge is how long the language choice is remembered, in
// seconds.
const languageCookieMaxAge = 365 * 24 * 60 * 60

// SupportedLanguage reports whether lang has a translation.
func SupportedLanguage(lang string) bool {
	return lang == "en" || lang == "bg"
}

// SetLanguageCookie remembers the language for this browser. The cookie is
// host-only, so there is exactly one; clearing a copy that older versions
// set for the LAN domain keeps it from shadowing the new choice.
func SetLanguageCookie(c *gin.Context, domain, lang string) {
	if domain != "" {
		c.SetCookie(LanguageCookieKey, "", -1, "/", domain, false, true)
	}
	c.SetCookie(LanguageCookieKey, lang, languageCookieMaxAge, "/", "", false, true)
}

// I18nMiddleware determines the language for the request.
func I18nMiddleware(as *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang, err := c.Cookie(LanguageCookieKey)
		if err == nil && SupportedLanguage(lang) {
			c.Set(LanguageCookieKey, lang)
			c.Next()
			return
		}

		claims := services.ClaimsFromContext(c)
		if claims != nil && SupportedLanguage(claims.PreferredLanguage) {
			c.Set(LanguageCookieKey, claims.PreferredLanguage)
			c.Next()
			return
		}

		const language = "en"
		SetLanguageCookie(c, as.Domain, language)
		c.Set(LanguageCookieKey, language)
		c.Next()
	}
}
