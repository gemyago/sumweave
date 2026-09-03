package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUsernameExists = errors.New("username already exists")
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateUserParams struct {
	Username     string
	PasswordHash string
}

type UserStoreDeps struct {
	SQLDB       *sql.DB
	DatabaseDSN string
	TablePrefix string
	IDGen       ident.Generator
	Logger      *slog.Logger
}

type authUserModel struct {
	ID           string    `gorm:"column:id;size:255;not null;primaryKey"`
	Username     string    `gorm:"column:username;size:255;not null;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (authUserModel) TableName(namer schema.Namer) string { return namer.TableName("auth_users") }

type UserStore struct {
	db     *gorm.DB
	idGen  ident.Generator
	logger *slog.Logger
	now    func() time.Time
}

func NewUserStore(deps UserStoreDeps) (*UserStore, error) {
	if deps.IDGen == nil {
		return nil, errors.New("user id generator is required")
	}
	if deps.Logger == nil {
		return nil, errors.New("auth user store logger is required")
	}
	db, err := openAuthDatabase(deps.SQLDB, deps.DatabaseDSN, deps.TablePrefix)
	if err != nil {
		return nil, fmt.Errorf("open auth user store database: %w", err)
	}
	return &UserStore{db: db, idGen: deps.IDGen, logger: deps.Logger, now: time.Now}, nil
}

func (s *UserStore) AutoMigrate() error {
	if err := s.db.AutoMigrate(&authUserModel{}); err != nil {
		return fmt.Errorf("auto migrate auth users: %w", err)
	}
	return nil
}

func (s *UserStore) Create(ctx context.Context, params CreateUserParams) (*User, error) {
	now := s.now().Truncate(time.Microsecond)
	model := authUserModel{
		ID:           s.idGen.MustNewV7().String(),
		Username:     params.Username,
		PasswordHash: params.PasswordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrUsernameExists
		}
		return nil, fmt.Errorf("create auth user: %w", err)
	}
	user := authUserFromModel(model)
	s.logger.DebugContext(ctx, "user created", slog.String("userID", user.ID))
	return &user, nil
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	var model authUserModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get auth user by id: %w", err)
	}
	user := authUserFromModel(model)
	return &user, nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	var model authUserModel
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get auth user by username: %w", err)
	}
	user := authUserFromModel(model)
	return &user, nil
}

func (s *UserStore) List(ctx context.Context) ([]User, error) {
	var models []authUserModel
	if err := s.db.WithContext(ctx).Order("username ASC, id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list auth users: %w", err)
	}
	users := make([]User, 0, len(models))
	for _, model := range models {
		users = append(users, authUserFromModel(model))
	}
	return users, nil
}

func (s *UserStore) UpdatePassword(ctx context.Context, id string, newHash string) error {
	result := s.db.WithContext(ctx).Model(&authUserModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"password_hash": newHash, "updated_at": s.now().Truncate(time.Microsecond)})
	if result.Error != nil {
		return fmt.Errorf("update auth user password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	s.logger.DebugContext(ctx, "user password updated", slog.String("userID", id))
	return nil
}

func authUserFromModel(model authUserModel) User {
	return User(model)
}

func openAuthDatabase(sqlDB *sql.DB, dsn, tablePrefix string) (*gorm.DB, error) {
	if sqlDB == nil {
		return nil, errors.New("auth sql database is required")
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("auth database dsn is required")
	}
	if err := validateTablePrefix(tablePrefix); err != nil {
		return nil, err
	}

	if _, err := pgx.ParseConfig(dsn); err != nil {
		return nil, fmt.Errorf("parse auth database dsn: %w", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
		TranslateError: true,
		NowFunc: func() time.Time {
			return time.Now().Truncate(time.Microsecond)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm database: %w", err)
	}
	return db, nil
}

func validateTablePrefix(prefix string) error {
	for _, character := range prefix {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' {
			return errors.New("auth table prefix may contain only letters, digits, and underscores")
		}
	}
	return nil
}
