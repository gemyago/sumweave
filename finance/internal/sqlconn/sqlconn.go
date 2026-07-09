package sqlconn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/glebarez/go-sqlite"  // Register the repo-standard SQLite driver.
	_ "github.com/jackc/pgx/v5/stdlib" // Register the repo-standard Postgres driver.
)

const (
	SQLiteBusyTimeoutMillis = 5000
	sqliteMemoryDSN         = ":memory:"
	sqliteMaxOpenConns      = 4
)

func Open(dsn string) (*sql.DB, error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return nil, errors.New("database dsn is required")
	}
	driver := "sqlite"
	if !IsSQLiteDSN(trimmed) {
		driver = "pgx"
	}
	db, err := sql.Open(driver, trimmed)
	if err != nil {
		return nil, fmt.Errorf("open sql database: %w", err)
	}
	if err = ApplySQLiteDefaults(db, trimmed); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func IsSQLiteDSN(dsn string) bool {
	trimmed := strings.TrimSpace(dsn)
	return trimmed == sqliteMemoryDSN ||
		strings.HasPrefix(trimmed, "file:") ||
		strings.Contains(trimmed, "sqlite") ||
		strings.HasSuffix(trimmed, ".db") ||
		strings.HasSuffix(trimmed, ".sqlite")
}

func ApplySQLiteDefaults(db *sql.DB, dsn string) error {
	if db == nil || !IsSQLiteDSN(dsn) {
		return nil
	}
	if SupportsSQLiteWAL(dsn) {
		db.SetMaxOpenConns(sqliteMaxOpenConns)
		db.SetMaxIdleConns(sqliteMaxOpenConns)
	} else {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	if _, err := db.ExecContext(
		context.Background(),
		fmt.Sprintf("PRAGMA busy_timeout = %d", SQLiteBusyTimeoutMillis),
	); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if !SupportsSQLiteWAL(dsn) {
		return nil
	}
	var journalMode string
	if err := db.QueryRowContext(context.Background(), "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("set sqlite journal mode: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(journalMode), "wal") {
		return fmt.Errorf("set sqlite journal mode: unexpected mode %q", journalMode)
	}
	return nil
}

func SupportsSQLiteWAL(dsn string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(dsn))
	return trimmed != sqliteMemoryDSN &&
		!strings.Contains(trimmed, "mode=memory") &&
		!strings.Contains(trimmed, "cache=shared&mode=memory") &&
		!strings.Contains(trimmed, "mode=ro") &&
		!strings.Contains(trimmed, "immutable=1")
}
