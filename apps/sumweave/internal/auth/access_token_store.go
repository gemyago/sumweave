package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var (
	ErrAccessTokenNotFound   = errors.New("access token not found")
	ErrAccessTokenNameExists = errors.New("active access token name already exists")
	ErrAccessTokenConflict   = errors.New("access token conflict")
	ErrAccessTokenInactive   = errors.New("access token is inactive")
)

const updatedAtColumn = "updated_at"

// AccessToken contains persisted token data. Callers must not expose SecretHash.
type AccessToken struct {
	ID         string
	UserID     string
	Name       string
	Permission AccessTokenPermission
	SecretHash []byte
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AccessTokenStoreDeps struct {
	SQLDB       *sql.DB
	DatabaseDSN string
	TablePrefix string
	Logger      *slog.Logger
}

type accessTokenModel struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;not null;primaryKey"`
	UserID     uuid.UUID  `gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_auth_access_tokens_active_user_name,priority:1,where:revoked_at IS NULL;index:idx_auth_access_tokens_user_created,priority:1"`
	Name       string     `gorm:"column:name;type:text;not null;check:chk_auth_access_tokens_name_length,char_length(name) BETWEEN 1 AND 100;uniqueIndex:idx_auth_access_tokens_active_user_name,priority:2,where:revoked_at IS NULL"`
	Permission string     `gorm:"column:permission;type:text;not null;check:chk_auth_access_tokens_permission,permission IN ('read-only', 'read-write')"`
	SecretHash []byte     `gorm:"column:secret_hash;type:bytea;not null;check:chk_auth_access_tokens_secret_hash_length,octet_length(secret_hash) = 32"`
	ExpiresAt  *time.Time `gorm:"column:expires_at;type:timestamptz"`
	RevokedAt  *time.Time `gorm:"column:revoked_at;type:timestamptz"`
	CreatedAt  time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime;index:idx_auth_access_tokens_user_created,priority:2,sort:desc"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (accessTokenModel) TableName(namer schema.Namer) string {
	return namer.TableName("auth_access_tokens")
}

type RotateAccessTokenParams struct {
	UserID             string
	TokenID            string
	ReplacementTokenID string
	SecretHash         []byte
	ExpiresAt          *time.Time
	Now                time.Time
}

// AccessTokenStore owns PostgreSQL persistence for personal access tokens.
type AccessTokenStore struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewAccessTokenStore(deps AccessTokenStoreDeps) (*AccessTokenStore, error) {
	if deps.Logger == nil {
		return nil, errors.New("access token store logger is required")
	}
	db, err := openAuthDatabase(deps.SQLDB, deps.DatabaseDSN, deps.TablePrefix)
	if err != nil {
		return nil, fmt.Errorf("open access token store database: %w", err)
	}
	return &AccessTokenStore{db: db, logger: deps.Logger}, nil
}

func (s *AccessTokenStore) AutoMigrate() error {
	if err := s.db.AutoMigrate(&accessTokenModel{}); err != nil {
		return fmt.Errorf("auto migrate access tokens: %w", err)
	}
	return nil
}

func (s *AccessTokenStore) Create(ctx context.Context, token AccessToken) error {
	model, err := accessTokenToModel(token)
	if err != nil {
		return err
	}
	if createErr := s.db.WithContext(ctx).Create(&model).Error; createErr != nil {
		if errors.Is(createErr, gorm.ErrDuplicatedKey) {
			return ErrAccessTokenNameExists
		}
		return fmt.Errorf("create access token: %w", createErr)
	}
	s.logger.DebugContext(
		ctx,
		"access token created",
		slog.String("tokenID", token.ID),
		slog.String("userID", token.UserID),
	)
	return nil
}

func (s *AccessTokenStore) GetByID(ctx context.Context, tokenID string) (*AccessToken, error) {
	id, err := parseAccessTokenUUID(tokenID)
	if err != nil {
		return nil, ErrAccessTokenNotFound
	}
	var model accessTokenModel
	if getErr := s.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; getErr != nil {
		if errors.Is(getErr, gorm.ErrRecordNotFound) {
			return nil, ErrAccessTokenNotFound
		}
		return nil, fmt.Errorf("get access token by id: %w", getErr)
	}
	token := accessTokenFromModel(model)
	return &token, nil
}

func (s *AccessTokenStore) ListByUserID(ctx context.Context, userID string) ([]AccessToken, error) {
	id, err := parseAccessTokenUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("parse access token owner id: %w", err)
	}
	var models []accessTokenModel
	if listErr := s.db.WithContext(ctx).
		Where("user_id = ?", id).
		Order("created_at DESC, id DESC").
		Find(&models).Error; listErr != nil {
		return nil, fmt.Errorf("list access tokens by user id: %w", listErr)
	}
	tokens := make([]AccessToken, 0, len(models))
	for _, model := range models {
		tokens = append(tokens, accessTokenFromModel(model))
	}
	return tokens, nil
}

// Revoke marks one owned token revoked while preserving its first revocation time.
func (s *AccessTokenStore) Revoke(ctx context.Context, userID, tokenID string, revokedAt time.Time) error {
	ownerID, err := parseAccessTokenUUID(userID)
	if err != nil {
		return ErrAccessTokenNotFound
	}
	id, err := parseAccessTokenUUID(tokenID)
	if err != nil {
		return ErrAccessTokenNotFound
	}
	revokedAt = revokedAt.Truncate(time.Microsecond)
	result := s.db.WithContext(ctx).Model(&accessTokenModel{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", id, ownerID).
		Updates(map[string]any{"revoked_at": revokedAt, updatedAtColumn: revokedAt})
	if result.Error != nil {
		return fmt.Errorf("revoke access token: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		s.logger.DebugContext(
			ctx,
			"access token revoked",
			slog.String("tokenID", tokenID),
			slog.String("userID", userID),
		)
		return nil
	}
	var count int64
	if lookupErr := s.db.WithContext(ctx).
		Model(&accessTokenModel{}).
		Where("id = ? AND user_id = ?", id, ownerID).
		Count(&count).Error; lookupErr != nil {
		return fmt.Errorf("check revoked access token: %w", lookupErr)
	}
	if count == 0 {
		return ErrAccessTokenNotFound
	}
	return nil
}

// Rotate serializes replacement issuance with a row lock and one transaction.
//
//nolint:gocognit // The transaction's explicit lifecycle branches preserve rotation invariants.
func (s *AccessTokenStore) Rotate(ctx context.Context, params RotateAccessTokenParams) (*AccessToken, error) {
	ownerID, err := parseAccessTokenUUID(params.UserID)
	if err != nil {
		return nil, ErrAccessTokenNotFound
	}
	tokenID, err := parseAccessTokenUUID(params.TokenID)
	if err != nil {
		return nil, ErrAccessTokenNotFound
	}
	now := params.Now.Truncate(time.Microsecond)
	var replacement AccessToken
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var original accessTokenModel
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", tokenID, ownerID).First(&original).Error; lockErr != nil {
			if errors.Is(lockErr, gorm.ErrRecordNotFound) {
				return ErrAccessTokenNotFound
			}
			return fmt.Errorf("lock access token for rotation: %w", lockErr)
		}
		if original.RevokedAt != nil {
			return ErrAccessTokenConflict
		}
		if original.ExpiresAt != nil && !original.ExpiresAt.After(now) {
			return ErrAccessTokenInactive
		}

		if updateErr := tx.Model(&original).Updates(map[string]any{
			"revoked_at":    now,
			updatedAtColumn: now,
		}).Error; updateErr != nil {
			return fmt.Errorf("revoke access token for rotation: %w", updateErr)
		}
		newID, idErr := parseAccessTokenUUID(params.ReplacementTokenID)
		if idErr != nil {
			return fmt.Errorf("parse replacement access token id: %w", idErr)
		}
		model := accessTokenModel{
			ID: newID, UserID: ownerID, Name: original.Name, Permission: original.Permission,
			SecretHash: append([]byte(nil), params.SecretHash...), ExpiresAt: copyTime(params.ExpiresAt),
			CreatedAt: now, UpdatedAt: now,
		}
		if createErr := tx.Create(&model).Error; createErr != nil {
			if errors.Is(createErr, gorm.ErrDuplicatedKey) {
				return ErrAccessTokenNameExists
			}
			return fmt.Errorf("create replacement access token: %w", createErr)
		}
		replacement = accessTokenFromModel(model)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.DebugContext(
		ctx,
		"access token rotated",
		slog.String("tokenID", tokenID.String()),
		slog.String("userID", params.UserID),
	)
	return &replacement, nil
}

func accessTokenToModel(token AccessToken) (accessTokenModel, error) {
	id, err := parseAccessTokenUUID(token.ID)
	if err != nil {
		return accessTokenModel{}, fmt.Errorf("parse access token id: %w", err)
	}
	userID, err := parseAccessTokenUUID(token.UserID)
	if err != nil {
		return accessTokenModel{}, fmt.Errorf("parse access token user id: %w", err)
	}
	return accessTokenModel{
		ID: id, UserID: userID, Name: token.Name, Permission: string(token.Permission),
		SecretHash: append([]byte(nil), token.SecretHash...), ExpiresAt: copyTime(token.ExpiresAt),
		RevokedAt: copyTime(token.RevokedAt), CreatedAt: token.CreatedAt.Truncate(time.Microsecond),
		UpdatedAt: token.UpdatedAt.Truncate(time.Microsecond),
	}, nil
}

func accessTokenFromModel(model accessTokenModel) AccessToken {
	return AccessToken{
		ID: model.ID.String(), UserID: model.UserID.String(), Name: model.Name,
		Permission: AccessTokenPermission(model.Permission), SecretHash: append([]byte(nil), model.SecretHash...),
		ExpiresAt: copyTime(model.ExpiresAt), RevokedAt: copyTime(model.RevokedAt),
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func parseAccessTokenUUID(value string) (uuid.UUID, error) {
	id, err := uuid.FromString(value)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
