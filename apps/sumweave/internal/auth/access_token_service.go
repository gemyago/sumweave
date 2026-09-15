package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gofrs/uuid/v5"
)

const (
	accessTokenPrefix      = "swat_"
	accessTokenSecretBytes = 32
)

type AccessTokenPermission string

const (
	AccessTokenPermissionReadOnly  AccessTokenPermission = "read-only"
	AccessTokenPermissionReadWrite AccessTokenPermission = "read-write"
)

type AccessTokenStatus string

const (
	AccessTokenStatusActive  AccessTokenStatus = "active"
	AccessTokenStatusExpired AccessTokenStatus = "expired"
	AccessTokenStatusRevoked AccessTokenStatus = "revoked"
)

var (
	ErrInvalidAccessToken      = errors.New("invalid access token")
	ErrInvalidAccessTokenInput = errors.New("invalid access token input")
)

type AccessTokenMetadata struct {
	ID         string
	Name       string
	Permission AccessTokenPermission
	Status     AccessTokenStatus
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// ValidatedAccessToken is the safe owner and token metadata established during validation.
type ValidatedAccessToken struct {
	AccessTokenMetadata

	UserID string
}

type IssuedAccessToken struct {
	AccessTokenMetadata

	APIToken string
}

type CreateAccessTokenParams struct {
	UserID     string
	Name       string
	Permission AccessTokenPermission
	ExpiresAt  *time.Time
}

type RotateAccessTokenRequest struct {
	UserID    string
	TokenID   string
	ExpiresAt *time.Time
}

type accessTokenStore interface {
	Create(ctx context.Context, token AccessToken) error
	GetByID(ctx context.Context, tokenID string) (*AccessToken, error)
	ListByUserID(ctx context.Context, userID string) ([]AccessToken, error)
	Revoke(ctx context.Context, userID, tokenID string, revokedAt time.Time) error
	Rotate(ctx context.Context, params RotateAccessTokenParams) (*AccessToken, error)
}

type accessTokenUserReader interface {
	GetByID(ctx context.Context, id string) (*User, error)
}

type AccessTokenServiceDeps struct {
	Store        accessTokenStore
	Users        accessTokenUserReader
	IDGen        ident.Generator
	Clock        func() time.Time
	RandomReader io.Reader
	Logger       *slog.Logger
}

// AccessTokenService owns token issuance, validation, and lifecycle behavior.
type AccessTokenService struct {
	store        accessTokenStore
	users        accessTokenUserReader
	idGen        ident.Generator
	clock        func() time.Time
	randomReader io.Reader
	logger       *slog.Logger
}

func NewAccessTokenService(deps AccessTokenServiceDeps) (*AccessTokenService, error) {
	if deps.Store == nil {
		return nil, errors.New("access token store is required")
	}
	if deps.Users == nil {
		return nil, errors.New("access token user reader is required")
	}
	if deps.IDGen == nil {
		return nil, errors.New("access token id generator is required")
	}
	if deps.Clock == nil {
		return nil, errors.New("access token clock is required")
	}
	if deps.RandomReader == nil {
		return nil, errors.New("access token random reader is required")
	}
	if deps.Logger == nil {
		return nil, errors.New("access token service logger is required")
	}
	return &AccessTokenService{
		store: deps.Store, users: deps.Users, idGen: deps.IDGen, clock: deps.Clock,
		randomReader: deps.RandomReader, logger: deps.Logger,
	}, nil
}

func (s *AccessTokenService) Create(ctx context.Context, params CreateAccessTokenParams) (*IssuedAccessToken, error) {
	name, err := validateAccessTokenInput(params.Name, params.Permission, params.ExpiresAt, s.now())
	if err != nil {
		return nil, err
	}
	if _, inputErr := uuid.FromString(params.UserID); inputErr != nil {
		return nil, fmt.Errorf("parse access token owner id: %w", ErrInvalidAccessTokenInput)
	}
	if _, userErr := s.users.GetByID(ctx, params.UserID); userErr != nil {
		if errors.Is(userErr, ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get access token owner: %w", userErr)
	}
	return s.issue(ctx, params.UserID, name, params.Permission, params.ExpiresAt)
}

func (s *AccessTokenService) List(ctx context.Context, userID string) ([]AccessTokenMetadata, error) {
	if _, err := uuid.FromString(userID); err != nil {
		return nil, fmt.Errorf("parse access token owner id: %w", ErrInvalidAccessTokenInput)
	}
	tokens, err := s.store.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list access tokens: %w", err)
	}
	metadata := make([]AccessTokenMetadata, 0, len(tokens))
	for _, token := range tokens {
		metadata = append(metadata, accessTokenMetadata(token, s.now()))
	}
	return metadata, nil
}

func (s *AccessTokenService) Revoke(ctx context.Context, userID, tokenID string) error {
	if _, err := uuid.FromString(userID); err != nil {
		return ErrAccessTokenNotFound
	}
	if _, err := uuid.FromString(tokenID); err != nil {
		return ErrAccessTokenNotFound
	}
	if err := s.store.Revoke(ctx, userID, tokenID, s.now()); err != nil {
		return err
	}
	s.logger.DebugContext(
		ctx,
		"access token lifecycle revoke completed",
		slog.String("tokenID", tokenID),
		slog.String("userID", userID),
	)
	return nil
}

func (s *AccessTokenService) Rotate(ctx context.Context, request RotateAccessTokenRequest) (*IssuedAccessToken, error) {
	now := s.now()
	if request.ExpiresAt != nil && !request.ExpiresAt.After(now) {
		return nil, ErrInvalidAccessTokenInput
	}
	if _, err := uuid.FromString(request.UserID); err != nil {
		return nil, ErrAccessTokenNotFound
	}
	if _, err := uuid.FromString(request.TokenID); err != nil {
		return nil, ErrAccessTokenNotFound
	}
	secret, err := s.newSecret()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(secret)
	replacementID, err := s.idGen.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate replacement access token id: %w", err)
	}
	replacement, err := s.store.Rotate(ctx, RotateAccessTokenParams{
		UserID: request.UserID, TokenID: request.TokenID, ReplacementTokenID: replacementID.String(),
		SecretHash: hash[:], ExpiresAt: copyTime(request.ExpiresAt), Now: now,
	})
	if err != nil {
		return nil, err
	}
	issued := &IssuedAccessToken{
		AccessTokenMetadata: accessTokenMetadata(*replacement, now),
		APIToken:            formatAccessToken(replacement.ID, secret),
	}
	s.logger.DebugContext(
		ctx,
		"access token lifecycle rotation completed",
		slog.String("tokenID", replacement.ID),
		slog.String("userID", request.UserID),
	)
	return issued, nil
}

// Validate checks a presented token against PostgreSQL without exposing why it failed.
func (s *AccessTokenService) Validate(ctx context.Context, value string) (*ValidatedAccessToken, error) {
	id, secret, err := parseAccessToken(value)
	if err != nil {
		return nil, ErrInvalidAccessToken
	}
	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAccessTokenNotFound) {
			return nil, ErrInvalidAccessToken
		}
		return nil, fmt.Errorf("get presented access token: %w", err)
	}
	hash := sha256.Sum256(secret)
	if subtle.ConstantTimeCompare(hash[:], token.SecretHash) != 1 ||
		accessTokenStatus(*token, s.now()) != AccessTokenStatusActive ||
		!validAccessTokenPermission(token.Permission) {
		return nil, ErrInvalidAccessToken
	}
	metadata := accessTokenMetadata(*token, s.now())
	return &ValidatedAccessToken{UserID: token.UserID, AccessTokenMetadata: metadata}, nil
}

func (s *AccessTokenService) issue(
	ctx context.Context,
	userID, name string,
	permission AccessTokenPermission,
	expiresAt *time.Time,
) (*IssuedAccessToken, error) {
	id, err := s.idGen.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate access token id: %w", err)
	}
	secret, err := s.newSecret()
	if err != nil {
		return nil, err
	}
	now := s.now()
	hash := sha256.Sum256(secret)
	token := AccessToken{
		ID: id.String(), UserID: userID, Name: name, Permission: permission, SecretHash: hash[:],
		ExpiresAt: copyTime(expiresAt), CreatedAt: now, UpdatedAt: now,
	}
	if createErr := s.store.Create(ctx, token); createErr != nil {
		return nil, createErr
	}
	issued := &IssuedAccessToken{
		AccessTokenMetadata: accessTokenMetadata(token, now),
		APIToken:            formatAccessToken(token.ID, secret),
	}
	s.logger.DebugContext(
		ctx,
		"access token lifecycle create completed",
		slog.String("tokenID", token.ID),
		slog.String("userID", userID),
	)
	return issued, nil
}

func (s *AccessTokenService) newSecret() ([]byte, error) {
	secret := make([]byte, accessTokenSecretBytes)
	if _, err := io.ReadFull(s.randomReader, secret); err != nil {
		return nil, fmt.Errorf("generate access token secret: %w", err)
	}
	return secret, nil
}

func (s *AccessTokenService) now() time.Time { return s.clock().Truncate(time.Microsecond) }

func validateAccessTokenInput(
	name string,
	permission AccessTokenPermission,
	expiresAt *time.Time,
	now time.Time,
) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 ||
		utf8.RuneCountInString(name) > 100 ||
		!validAccessTokenPermission(permission) {
		return "", ErrInvalidAccessTokenInput
	}
	if expiresAt != nil && !expiresAt.After(now) {
		return "", ErrInvalidAccessTokenInput
	}
	return name, nil
}

func validAccessTokenPermission(permission AccessTokenPermission) bool {
	return permission == AccessTokenPermissionReadOnly || permission == AccessTokenPermissionReadWrite
}

func accessTokenStatus(token AccessToken, now time.Time) AccessTokenStatus {
	if token.RevokedAt != nil {
		return AccessTokenStatusRevoked
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
		return AccessTokenStatusExpired
	}
	return AccessTokenStatusActive
}

func accessTokenMetadata(token AccessToken, now time.Time) AccessTokenMetadata {
	return AccessTokenMetadata{
		ID: token.ID, Name: token.Name, Permission: token.Permission, Status: accessTokenStatus(token, now),
		ExpiresAt: copyTime(token.ExpiresAt), RevokedAt: copyTime(token.RevokedAt), CreatedAt: token.CreatedAt,
	}
}

func formatAccessToken(id string, secret []byte) string {
	return accessTokenPrefix + id + "_" + base64.RawURLEncoding.EncodeToString(secret)
}

func parseAccessToken(value string) (string, []byte, error) {
	if !strings.HasPrefix(value, accessTokenPrefix) {
		return "", nil, errors.New("access token prefix is invalid")
	}
	rest := strings.TrimPrefix(value, accessTokenPrefix)
	if len(rest) < 37 || rest[36] != '_' {
		return "", nil, errors.New("access token token id is invalid")
	}
	idText := rest[:36]
	id, err := uuid.FromString(idText)
	if err != nil || id.Version() != uuid.V7 {
		return "", nil, errors.New("access token token id is invalid")
	}
	secret, err := base64.RawURLEncoding.DecodeString(rest[37:])
	if err != nil || len(secret) != accessTokenSecretBytes {
		return "", nil, errors.New("access token secret is invalid")
	}
	return id.String(), secret, nil
}
