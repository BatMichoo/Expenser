package middleware

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	authService *services.AuthService
}

func NewAuthMiddleware(as *services.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: as,
	}
}

// redirectToLogin shows the login form and remembers the page that was
// asked for, so logging in leads back to it (e.g. a link in an email).
func (am *AuthMiddleware) redirectToLogin(c *gin.Context) {
	next := ""
	if c.GetHeader("HX-Request") == "true" {
		// The page the user is on, not the fragment HTMX asked for.
		if u, err := url.Parse(c.GetHeader("HX-Current-URL")); err == nil {
			next = u.RequestURI()
		}
	} else if c.Request.Method == http.MethodGet {
		next = c.Request.URL.RequestURI()
	}
	login := "/login"
	if next != "" && next != "/" {
		login += "?next=" + url.QueryEscape(next)
	}
	c.Header("HX-Redirect", login)
	lang := c.GetString(LanguageCookieKey)
	rl := &models.RootLayout{
		TemplateName:    utilities.Templates.Pages.Login,
		TemplateContent: gin.H{"Lang": lang, "Next": next},
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: false,
			IsOOB:      true,
			Lang:       lang,
		},
		Lang: lang,
		Year: time.Now().Year(),
	}
	c.HTML(http.StatusOK, utilities.Templates.Root, rl)
}

func LoadClaims(as *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := as.ValidateToken(c); err == nil {
			services.SetClaims(c, token.Claims)
		}
		c.Next()
	}
}

func (am *AuthMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := am.authService.ValidateToken(c)
		if err != nil {
			am.redirectToLogin(c)
			c.Abort()
			return
		}

		if am.authService.NeedsRefresh(token, time.Now()) {
			if fresh, err := am.authService.Refresh(token); err == nil {
				am.authService.SetCookie(fresh, c)
				token = fresh
			} else {
				log.Printf("refresh login token: %v", err)
			}
		}

		services.SetClaims(c, token.Claims)

		c.Next()
	}
}
