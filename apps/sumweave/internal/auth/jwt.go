package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims is the set of claims embedded in each access token.
type JWTClaims struct {
	jwt.RegisteredClaims

	Username string `json:"username"`
}

// JWTServiceDeps are the dependencies for JWTService.
type JWTServiceDeps struct {
	// SigningKey is the required HMAC-SHA256 signing key.
	SigningKey     string
	AccessTokenTTL time.Duration
	Logger         *slog.Logger
}

// JWTService issues and validates JWT access tokens.
type JWTService struct {
	signingKey    []byte
	signingMethod jwt.SigningMethod
	ttl           time.Duration
	logger        *slog.Logger
}

// NewJWTService creates a JWTService, resolving the signing key.
func NewJWTService(deps JWTServiceDeps) (*JWTService, error) {
	key, err := resolveSigningKey(deps.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("resolve JWT signing key: %w", err)
	}

	return &JWTService{
		signingKey:    key,
		signingMethod: jwt.SigningMethodHS256,
		ttl:           deps.AccessTokenTTL,
		logger:        deps.Logger,
	}, nil
}

// GenerateAccessToken creates a signed JWT access token for the given user.
func (s *JWTService) GenerateAccessToken(userID, username string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
		Username: username,
	}

	token := jwt.NewWithClaims(s.signingMethod, claims)
	signed, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return signed, nil
}

// ValidateAccessToken parses and validates a JWT access token string.
func (s *JWTService) ValidateAccessToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("invalid access token claims")
	}
	return claims, nil
}

func resolveSigningKey(configKey string) ([]byte, error) {
	if configKey == "" {
		return nil, errors.New("JWT signing key is required")
	}
	return []byte(configKey), nil
}
