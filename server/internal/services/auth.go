package services

import (
	"errors"
	"expenser/internal/models"
	"fmt"
	"time"

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
	Value      string
	Expiration time.Duration
}

func NewAuthService(secretKey string, tokenExp time.Duration) *AuthService {
	return &AuthService{
		secretKey:       []byte(secretKey),
		tokenExpiration: tokenExp,
	}
}

// GenerateToken creates a new JWT token for the given user
func (as *AuthService) GenerateToken(user *models.User) (*Token, error) {
	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(as.tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "expenser-app",
			Subject:   fmt.Sprintf("user:%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(as.secretKey)

	if err != nil {
		return nil, fmt.Errorf("couldn't generate token %w", err)
	}

	return &Token{
		Value:      signedToken,
		Expiration: as.tokenExpiration,
	}, nil
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
