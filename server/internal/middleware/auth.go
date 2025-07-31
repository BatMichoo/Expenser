package middleware

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"net/http"

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
	c.HTML(http.StatusOK, utilities.Templates.Components.Header, &models.HeaderOptions{
		IsLoggedIn: false,
		IsOOB:      true,
	})
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

		claims, err := am.authService.ValidateToken(tokenString)
		if err != nil {
			am.redirectToLogin(c)
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("user_claims", claims)

		c.Next()
	}
}
