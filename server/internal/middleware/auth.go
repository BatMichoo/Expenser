package middleware

import (
	"errors"
	"expenser/internal/models"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims stored in the JWT token
type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// AuthService handles JWT token operations
type AuthService struct {
	secretKey       []byte
	tokenExpiration time.Duration
}

// NewAuthService creates a new AuthService instance
func NewAuthService(secretKey string, tokenExpiration time.Duration) *AuthService {
	return &AuthService{
		secretKey:       []byte(secretKey),
		tokenExpiration: tokenExpiration,
	}
}

// GenerateToken creates a new JWT token for the given user
func (a *AuthService) GenerateToken(user *models.User) (string, error) {
	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "expenser-app",
			Subject:   fmt.Sprintf("user:%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// ValidateToken validates and parses a JWT token
func (a *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ExtractTokenFromHeader extracts the JWT token from the Authorization header
func (a *AuthService) ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("authorization header format must be Bearer {token}")
	}

	return parts[1], nil
}

// ExtractTokenFromCookie extracts the JWT token from the auth cookie
func (a *AuthService) ExtractTokenFromCookie(c *gin.Context) (string, error) {
	token, err := c.Cookie("auth_token")
	if err != nil {
		return "", errors.New("auth token cookie not found")
	}
	return token, nil
}

// AuthMiddleware creates a middleware function that validates JWT tokens
func (a *AuthService) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		var err error

		// Try to get token from cookie first (for web interface)
		tokenString, err = a.ExtractTokenFromCookie(c)
		if err != nil {
			// If no cookie, try Authorization header (for API)
			authHeader := c.GetHeader("Authorization")
			tokenString, err = a.ExtractTokenFromHeader(authHeader)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}
		}

		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
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

// OptionalAuthMiddleware creates a middleware that extracts user info if token exists but doesn't require it
func (a *AuthService) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		var err error

		// Try to get token from cookie first
		tokenString, err = a.ExtractTokenFromCookie(c)
		if err != nil {
			// Try Authorization header
			authHeader := c.GetHeader("Authorization")
			tokenString, err = a.ExtractTokenFromHeader(authHeader)
			if err != nil {
				// No token found, continue without authentication
				c.Next()
				return
			}
		}

		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			// Invalid token, continue without authentication
			c.Next()
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

// GetUserFromContext extracts user information from the Gin context
func GetUserFromContext(c *gin.Context) (int, string, string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, "", "", false
	}

	username, _ := c.Get("username")
	email, _ := c.Get("email")

	return userID.(int), username.(string), email.(string), true
}
