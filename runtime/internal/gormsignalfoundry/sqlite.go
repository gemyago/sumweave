package gormsignalfoundry

import (
	"fmt"

	"gorm.io/gorm"
)

const (
	sqliteBusyTimeoutMillis = 5000
	sqliteMemoryDSN         = ":memory:"
)

// ApplySQLiteConnectionDefaults configures SQLite handles for shared local use.
func ApplySQLiteConnectionDefaults(db *gorm.DB, dsn string) error {
	if db == nil || !isSQLiteDSN(dsn) {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("resolve sqlite database handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if execErr := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeoutMillis)).Error; execErr != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", execErr)
	}
	return applySQLiteWAL(db, dsn)
}
