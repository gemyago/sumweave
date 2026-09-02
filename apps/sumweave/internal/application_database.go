package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // Register the PostgreSQL database driver.
)

// OpenApplicationSQLDB opens the shared application database. Explicit roots
// use this form when they need ordered cleanup with database-backed resources.
func OpenApplicationSQLDB(databaseDSN string) (*sql.DB, error) {
	databaseDSN = strings.TrimSpace(databaseDSN)
	if databaseDSN == "" {
		return nil, fmt.Errorf("open application sql database: %w", errors.New("database dsn is required"))
	}
	if _, err := pgx.ParseConfig(databaseDSN); err != nil {
		return nil, fmt.Errorf("open application sql database: parse PostgreSQL dsn: %w", err)
	}
	db, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open application sql database: %w", err)
	}
	return db, nil
}

// NewApplicationSQLDB opens the application database and registers its cleanup.
func NewApplicationSQLDB(databaseDSN string, shutdownHooks *lifecycle.ShutdownHooks) (*sql.DB, error) {
	db, err := OpenApplicationSQLDB(databaseDSN)
	if err != nil {
		return nil, err
	}
	shutdownHooks.RegisterNoCtx("application-sql-db", db.Close)
	return db, nil
}
