package internal

import (
	"database/sql"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
)

// OpenApplicationSQLDB opens the shared application database. Explicit roots
// use this form when they need ordered cleanup with database-backed resources.
func OpenApplicationSQLDB(databaseDSN string) (*sql.DB, error) {
	db, err := sqlconn.Open(databaseDSN)
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
