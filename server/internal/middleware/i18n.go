package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LanguageCookieKey is the context key of the request's language.
const LanguageCookieKey = "lang"

// LanguageCookie is the home server's language cookie, shared by every
// app and set for the whole LAN domain.
const LanguageCookie = "homeserver_lang"

// languageCookieMaxAge is how long the language choice is remembered, in
// seconds.
const languageCookieMaxAge = 365 * 24 * 60 * 60

// SupportedLanguage reports whether lang has a translation.
func SupportedLanguage(lang string) bool {
	return lang == "en" || lang == "bg"
}

// SetLanguageCookie remembers the language for every app of the home
// server: the cookie is for domain (the LAN domain; "" = this host only).
func SetLanguageCookie(c *gin.Context, domain, lang string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: LanguageCookie, Value: lang, Domain: domain, Path: "/",
		MaxAge: languageCookieMaxAge, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

// I18nMiddleware sets the request's language from the home server's
// language cookie; English without one.
func I18nMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := "en"
		if v, err := c.Cookie(LanguageCookie); err == nil && SupportedLanguage(v) {
			lang = v
		}
		c.Set(LanguageCookieKey, lang)
		c.Next()
	}
}
