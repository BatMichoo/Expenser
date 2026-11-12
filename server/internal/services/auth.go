package services

import (
	"context"
	"errors"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the claims stored in the JWT token
type JWTClaims struct {
	UserID            uuid.UUID
	Username          string
	PreferredLanguage string
	// TokenVersion is the user's token version when the token was issued;
	// tokens from before a password change carry an older one.
	TokenVersion int `json:",omitempty"`
	jwt.RegisteredClaims
}

// TokenVersions looks up a user's current token version.
type TokenVersions interface {
	TokenVersion(ctx context.Context, userID uuid.UUID) (int, error)
}

// ErrLoginRevoked means the token was issued before the user's password
// changed.
var ErrLoginRevoked = errors.New("login was revoked")

type AuthService struct {
	secretKey       []byte
	tokenExpiration time.Duration
	Domain          string
	versions        TokenVersions // nil: versions aren't checked (tests)
}

type Token struct {
	Value     string
	Claims    *JWTClaims
	Duration  time.Duration
	ExpiresAt time.Time
}

const authCookieName = "auth_token"
const claimsContextKey = "user_claims"

// tokenContextKey caches the validated token for the rest of the request,
// so the version is looked up once even though two middlewares validate.
const tokenContextKey = "auth_token_validated"

func SetClaims(c *gin.Context, claims *JWTClaims) {
	c.Set(claimsContextKey, claims)
}

func ClaimsFromContext(c *gin.Context) *JWTClaims {
	claims, _ := c.Get(claimsContextKey)
	jwtClaims, _ := claims.(*JWTClaims)
	return jwtClaims
}

func NewAuthService(secretKey string, tokenExp time.Duration, lanDomain string) *AuthService {
	return &AuthService{
		secretKey:       []byte(secretKey),
		tokenExpiration: tokenExp,
		Domain:          lanDomain,
	}
}

// CheckTokenVersions makes ValidateToken refuse tokens whose version is not
// the user's current one (and tokens of users that no longer exist).
func (as *AuthService) CheckTokenVersions(v TokenVersions) {
	as.versions = v
}

func (as *AuthService) SetCookie(t *Token, c *gin.Context) {
	secure := true
	httpOnly := true

	c.SetCookie(authCookieName, t.Value, int(t.Duration.Seconds()), "/", as.Domain, secure, httpOnly)
}

func (as *AuthService) ClearCookie(c *gin.Context) {
	secure := true
	httpOnly := true

	c.SetCookie(
		authCookieName,
		"",
		-1,
		"/",
		as.Domain,
		secure,
		httpOnly,
	)
}

// GenerateToken creates a new JWT token for the given user
func (as *AuthService) GenerateToken(user *models.User) (*Token, error) {
	timeNow := time.Now()
	expAt := timeNow.Add(as.tokenExpiration)
	claims := &JWTClaims{
		UserID:            user.ID,
		Username:          user.Username,
		PreferredLanguage: user.PreferredLanguage,
		TokenVersion:      user.TokenVersion,
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

// NeedsRefresh reports whether a login has used up half its lifetime, so the
// next request should renew it. An active user then stays logged in, while
// one who is away longer than the lifetime has to log in again.
func (as *AuthService) NeedsRefresh(t *Token, now time.Time) bool {
	return t.ExpiresAt.Sub(now) < as.tokenExpiration/2
}

// Refresh issues a new token with the same user claims and a full lifetime.
// Re-setting the old token would only extend the cookie, not the token's
// own expiry.
func (as *AuthService) Refresh(t *Token) (*Token, error) {
	return as.GenerateToken(&models.User{
		ID:                t.Claims.UserID,
		Username:          t.Claims.Username,
		PreferredLanguage: t.Claims.PreferredLanguage,
		TokenVersion:      t.Claims.TokenVersion,
	})
}

// ValidateToken validates and parses a JWT token
func (as *AuthService) ValidateToken(c *gin.Context) (*Token, error) {
	if cached, ok := c.Get(tokenContextKey); ok {
		return cached.(*Token), nil
	}
	tokenString, _ := c.Cookie(authCookieName)
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

	if as.versions != nil {
		current, err := as.versions.TokenVersion(c.Request.Context(), claims.UserID)
		if err != nil {
			return nil, fmt.Errorf("check token version: %w", err)
		}
		if current != claims.TokenVersion {
			return nil, ErrLoginRevoked
		}
	}

	expAt, _ := token.Claims.GetExpirationTime()

	t := &Token{
		Value:     token.Raw,
		Claims:    claims,
		Duration:  as.tokenExpiration,
		ExpiresAt: time.Unix(expAt.Unix(), 0),
	}
	c.Set(tokenContextKey, t)
	return t, nil
}
