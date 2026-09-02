package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ErrInvalidRefreshToken is returned when a refresh token is missing, expired, or already consumed.
var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

type RefreshTokenStoreDeps struct {
	SQLDB       *sql.DB
	DatabaseDSN string
	TablePrefix string
	Logger      *slog.Logger
}

type authRefreshTokenModel struct {
	TokenHash string    `gorm:"column:token_hash;size:64;not null;primaryKey"`
	UserID    string    `gorm:"column:user_id;size:255;not null;index"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (authRefreshTokenModel) TableName(namer schema.Namer) string {
	return namer.TableName("auth_refresh_tokens")
}

type RefreshTokenStore struct {
	db           *gorm.DB
	logger       *slog.Logger
	randomSource io.Reader
}

func NewRefreshTokenStore(deps RefreshTokenStoreDeps) (*RefreshTokenStore, error) {
	if deps.Logger == nil {
		return nil, errors.New("refresh token store logger is required")
	}
	db, err := openAuthDatabase(deps.SQLDB, deps.DatabaseDSN, deps.TablePrefix)
	if err != nil {
		return nil, fmt.Errorf("open refresh token store database: %w", err)
	}
	return &RefreshTokenStore{db: db, logger: deps.Logger, randomSource: rand.Reader}, nil
}

func (s *RefreshTokenStore) AutoMigrate() error {
	if err := s.db.AutoMigrate(&authRefreshTokenModel{}); err != nil {
		return fmt.Errorf("auto migrate refresh tokens: %w", err)
	}
	return nil
}

func hashToken(opaqueToken string) string {
	sum := sha256.Sum256([]byte(opaqueToken))
	return hex.EncodeToString(sum[:])
}

// Create generates a new opaque refresh token and stores only its SHA-256 hash.
func (s *RefreshTokenStore) Create(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.randomSource, raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	opaqueToken := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now().Round(0)
	if err := s.db.WithContext(ctx).Create(&authRefreshTokenModel{
		TokenHash: hashToken(opaqueToken),
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}).Error; err != nil {
		return "", fmt.Errorf("create refresh token: %w", err)
	}
	s.logger.DebugContext(ctx, "refresh token created", slog.String("userID", userID))
	return opaqueToken, nil
}

// Consume atomically deletes a valid refresh token and returns its user ID.
func (s *RefreshTokenStore) Consume(ctx context.Context, opaqueToken string) (string, error) {
	var row struct {
		UserID string `gorm:"column:user_id"`
	}
	result := s.db.WithContext(ctx).Raw(
		"DELETE FROM "+s.db.NamingStrategy.TableName("auth_refresh_tokens")+
			" WHERE token_hash = ? AND expires_at > ? RETURNING user_id",
		hashToken(opaqueToken),
		time.Now(),
	).Scan(&row)
	if result.Error != nil {
		return "", fmt.Errorf("consume refresh token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return "", ErrInvalidRefreshToken
	}
	return row.UserID, nil
}

func (s *RefreshTokenStore) DeleteAllForUser(ctx context.Context, userID string) error {
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&authRefreshTokenModel{}).Error; err != nil {
		return fmt.Errorf("delete user refresh tokens: %w", err)
	}
	return nil
}
