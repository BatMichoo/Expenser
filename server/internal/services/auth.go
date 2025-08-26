package services

import (
	"expenser/internal/models"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the claims stored in the JWT token
type JWTClaims struct {
	UserID   uuid.UUID
	Username string
	Email    string
	jwt.RegisteredClaims
}

type AuthService struct {
	secretKey       []byte
	tokenExpiration time.Duration
}

type Token struct {
	Value     string
	Claims    *JWTClaims
	Duration  time.Duration
	ExpiresAt time.Time
}

func NewAuthService(secretKey string, tokenExp time.Duration) *AuthService {
	return &AuthService{
		secretKey:       []byte(secretKey),
		tokenExpiration: tokenExp,
	}
}

func (as *AuthService) SetCookie(t *Token, c *gin.Context) {
	domain := os.Getenv("LAN_DOMAIN")

	if domain == "" {
		domain = "localhost"
	}

	secure := false
	httpOnly := true

	c.SetCookie("auth_token", t.Value, int(t.Duration.Seconds()), "/", domain, secure, httpOnly)
}

// GenerateToken creates a new JWT token for the given user
func (as *AuthService) GenerateToken(user *models.User) (*Token, error) {
	timeNow := time.Now()
	expAt := timeNow.Add(as.tokenExpiration)
	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expAt),
			IssuedAt:  jwt.NewNumericDate(timeNow),
			NotBefore: jwt.NewNumericDate(timeNow),
			Issuer:    "expenser-app",
			Subject:   fmt.Sprintf("user:%s", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(as.secretKey)
	token.Raw = signedToken

	if err != nil {
		return nil, fmt.Errorf("couldn't generate token %w", err)
	}

	return &Token{
		Value:     signedToken,
		Claims:    claims,
		Duration:  as.tokenExpiration,
		ExpiresAt: expAt,
	}, nil
}

// ValidateToken validates and parses a JWT token
func (as *AuthService) ValidateToken(tokenString string) (*Token, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return as.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("could not get claims from token")
	}

	expAt, _ := token.Claims.GetExpirationTime()

	return &Token{
		Value:     token.Raw,
		Claims:    claims,
		Duration:  as.tokenExpiration,
		ExpiresAt: time.Unix(expAt.Unix(), 0),
	}, nil
}
