package wireup

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
)

// UsersOptions identifies the configuration layer for a user administration root.
type UsersOptions struct {
	Environment string
}

// UsersRoot owns the concrete dependencies for one user administration command.
// It deliberately constructs neither HTTP, jobs, finance, nor agent runtime.
type UsersRoot struct {
	Store  *auth.UserStore
	Hasher *auth.Argon2idHasher

	closeDatabase func() error
}

// BuildUsers loads typed configuration and constructs only user administration
// dependencies.
func BuildUsers(
	options UsersOptions,
) (*UsersRoot, error) { // coverage-ignore // Real command-root behavior is covered directly with isolated storage.
	environment := options.Environment
	if environment == "" {
		environment = localEnvironment
	}
	values, err := config.LoadValues(config.ValuesLoadInput{Environment: environment})
	if err != nil {
		return nil, fmt.Errorf("load users configuration: %w", err)
	}
	rootConfig, err := values.UsersRoot(environment)
	if err != nil {
		return nil, err
	}

	database, err := sqlconn.Open(rootConfig.Application.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open users application database: %w", err)
	}
	store, err := auth.NewUserStore(auth.UserStoreDeps{
		SQLDB:       database,
		DatabaseDSN: rootConfig.Application.Database.DSN,
		TablePrefix: rootConfig.Application.Database.TablePrefix + "auth_",
		IDGen:       ident.NewDefaultGenerator(),
		Logger:      slog.New(slog.NewTextHandler(os.Stdout, nil)),
	})
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			return nil, fmt.Errorf("create users store: %w; close users database: %w", err, closeErr)
		}
		return nil, fmt.Errorf("create users store: %w", err)
	}
	return &UsersRoot{
		Store:         store,
		Hasher:        auth.NewArgon2idHasher(),
		closeDatabase: database.Close,
	}, nil
}

// Close releases the database opened for this command root.
func (root *UsersRoot) Close() error {
	return root.closeDatabase()
}
