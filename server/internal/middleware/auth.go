package middleware

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
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
	lang := c.GetString("lang")
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

		currentTime := time.Now()
		diff := token.ExpiresAt.Sub(currentTime)

		if time.Duration(diff.Hours()) < time.Duration(time.Hour.Hours()) {
			am.authService.SetCookie(token, c)
		}

		// Store user information in context
		c.Set("user_id", token.Claims.UserID)
		c.Set("username", token.Claims.Username)
		c.Set("email", token.Claims.Email)
		services.SetClaims(c, token.Claims)

		c.Next()
	}
}
