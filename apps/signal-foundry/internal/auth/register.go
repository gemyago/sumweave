package auth

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"go.uber.org/dig"
)

// userStoreDeps is the DI-aware version of UserStoreDeps for container wiring.
type userStoreDIParams struct {
	dig.In

	SQLDB           *sql.DB
	DatabaseDSN     string `name:"config.dataLayer.database.dsn"`
	DataTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	IDGen           ident.Generator
	Logger          *slog.Logger
}

// jwtSigningKeyDIParams resolves the effective JWT signing key for DI consumers.
type jwtSigningKeyDIParams struct {
	dig.In

	SigningKey string `name:"config.auth.jwtSigningKey"`
}

// jwtServiceDIParams is the DI-aware version of JWTServiceDeps for container wiring.
type jwtServiceDIParams struct {
	dig.In

	SigningKey     string        `name:"auth.jwtKey"`
	AccessTokenTTL time.Duration `name:"config.auth.accessTokenTTL"`
	Logger         *slog.Logger
}

// refreshStoreDIParams is the DI-aware version of RefreshTokenStoreDeps for container wiring.
type refreshStoreDIParams struct {
	dig.In

	SQLDB           *sql.DB
	DatabaseDSN     string `name:"config.dataLayer.database.dsn"`
	DataTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	Logger          *slog.Logger
}

// authServiceDIParams is the DI-aware version of ServiceDeps for container wiring.
type authServiceDIParams struct {
	dig.In

	UserStore         *UserStore
	JWTService        *JWTService
	RefreshTokenStore *RefreshTokenStore
	PasswordHasher    *Argon2idHasher
	RefreshTokenTTL   time.Duration `name:"config.auth.refreshTokenTTL"`
	Logger            *slog.Logger
}

func newUserStoreFromDI(params userStoreDIParams) (*UserStore, error) {
	store, err := NewUserStore(UserStoreDeps{
		SQLDB:       params.SQLDB,
		DatabaseDSN: params.DatabaseDSN,
		TablePrefix: params.DataTablePrefix + "auth_",
		IDGen:       params.IDGen,
		Logger:      params.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create auth user store: %w", err)
	}
	return store, nil
}

func newJWTSigningKeyFromDI(params jwtSigningKeyDIParams) (string, error) {
	key, err := resolveSigningKey(params.SigningKey)
	if err != nil {
		return "", err
	}

	return string(key), nil
}

func newJWTServiceFromDI(params jwtServiceDIParams) (*JWTService, error) {
	return NewJWTService(JWTServiceDeps{
		SigningKey:     params.SigningKey,
		AccessTokenTTL: params.AccessTokenTTL,
		Logger:         params.Logger,
	})
}

func newRefreshTokenStoreFromDI(params refreshStoreDIParams) (*RefreshTokenStore, error) {
	store, err := NewRefreshTokenStore(RefreshTokenStoreDeps{
		SQLDB:       params.SQLDB,
		DatabaseDSN: params.DatabaseDSN,
		TablePrefix: params.DataTablePrefix + "auth_",
		Logger:      params.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create refresh token store: %w", err)
	}
	return store, nil
}

func newAuthServiceFromDI(params authServiceDIParams) *AuthService {
	return NewAuthService(ServiceDeps{
		UserStore:         params.UserStore,
		JWTService:        params.JWTService,
		RefreshTokenStore: params.RefreshTokenStore,
		PasswordHasher:    params.PasswordHasher,
		RefreshTokenTTL:   params.RefreshTokenTTL,
		Logger:            params.Logger,
	})
}

// Register provides all auth components into the DI container.
func Register(container *dig.Container) error {
	return di.ProvideAll(container,
		NewArgon2idHasher,
		newUserStoreFromDI,
		di.ConstructorWithOpts{
			Constructor: newJWTSigningKeyFromDI,
			Options:     []dig.ProvideOption{dig.Name("auth.jwtKey")},
		},
		newJWTServiceFromDI,
		newRefreshTokenStoreFromDI,
		newAuthServiceFromDI,
	)
}
