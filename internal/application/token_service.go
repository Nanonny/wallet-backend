package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrTokenInvalid is returned when a session token fails validation.
var ErrTokenInvalid = errors.New("invalid session token")

// TokenService issues and validates stateless JWT (HS256) session tokens.
// Tokens carry only the authenticated username; wallet data stays in Redis.
type TokenService struct {
	secret  []byte
	issuer  string
	ttl     time.Duration
	nowFunc func() time.Time
}

// TokenOption configures the TokenService.
type TokenOption func(*TokenService)

func WithIssuer(issuer string) TokenOption {
	return func(s *TokenService) {
		s.issuer = issuer
	}
}

func WithTTL(ttl time.Duration) TokenOption {
	return func(s *TokenService) {
		s.ttl = ttl
	}
}

func WithClock(now func() time.Time) TokenOption {
	return func(s *TokenService) {
		s.nowFunc = now
	}
}

func NewTokenService(secret string, opts ...TokenOption) (*TokenService, error) {
	if len(secret) < 16 {
		return nil, fmt.Errorf("JWT secret must be at least 16 characters, got %d", len(secret))
	}
	s := &TokenService{
		secret:  []byte(secret),
		issuer:  "wallet-api",
		ttl:     8 * time.Hour,
		nowFunc: time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Issue creates a signed session token for the username.
func (s *TokenService) Issue(username string) (string, error) {
	now := s.nowFunc().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    s.issuer,
		Subject:   username,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Validate checks the signature, issuer, and expiry and returns the username.
func (s *TokenService) Validate(tokenString string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %q", token.Header["alg"])
		}
		return s.secret, nil
	},
		jwt.WithIssuer(s.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	if claims.Subject == "" {
		return "", ErrTokenInvalid
	}
	return claims.Subject, nil
}
