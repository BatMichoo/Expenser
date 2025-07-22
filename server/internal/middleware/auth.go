package middleware

import (
	"errors"
	"expenser/internal/services"
	"net/http"
	"strings"

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
		return "", errors.New("auth token cookie not found")
	}
	return token, nil
}

// ExtractTokenFromHeader extracts the JWT token from the Authorization header
func (am *AuthMiddleware) extractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("authorization header format must be Bearer {token}")
	}

	return parts[1], nil
}

// AuthMiddleware creates a middleware function that validates JWT tokens
func (am *AuthMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		var err error

		tokenString, err = am.extractTokenFromCookie(c)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		claims, err := am.authService.ValidateToken(tokenString)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
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
