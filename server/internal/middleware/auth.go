package middleware

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"
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

func (am *AuthMiddleware) redirectToLogin(c *gin.Context) {
	c.Header("HX-Redirect", "/login")
	lang := c.GetString("lang")
	rl := &models.RootLayout{
		TemplateName: utilities.Templates.Pages.Login,
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: false,
			IsOOB:      true,
			Lang:       lang,
		},
		Lang: lang,
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
