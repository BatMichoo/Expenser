package middleware

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware handles JWT token operations
type AuthMiddleware struct {
	authService *services.AuthService
}

// NewAuthMiddleware creates a new Auth middleware instance
func NewAuthMiddleware(as *services.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: as,
	}
}

func (am *AuthMiddleware) extractTokenFromCookie(c *gin.Context) (string, error) {
	token, err := c.Cookie("auth_token")
	if err != nil {
		return "", err
	}
	return token, nil
}

func (am *AuthMiddleware) redirectToLogin(c *gin.Context) {
	c.Header("HX-Redirect", "/login")
	rl := &models.RootLayout{
		TemplateName: utilities.Templates.Pages.Login,
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: false,
			IsOOB:      true,
		},
	}
	c.HTML(http.StatusOK, utilities.Templates.Root, rl)
}

// AuthMiddleware creates a middleware function that validates JWT tokens
func (am *AuthMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := am.extractTokenFromCookie(c)
		if err != nil {
			am.redirectToLogin(c)
			c.Abort()
			return
		}

		token, err := am.authService.ValidateToken(tokenString)
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
		c.Set("user_claims", token.Claims)

		c.Next()
	}
}
