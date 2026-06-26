package gormsignalfoundry

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func applySQLiteWAL(db *gorm.DB, dsn string) error {
	if !supportsSQLiteWAL(dsn) {
		return nil
	}

	var journalMode string
	if rowErr := db.Raw("PRAGMA journal_mode = WAL").Row().Scan(&journalMode); rowErr != nil {
		return fmt.Errorf("set sqlite journal mode: %w", rowErr)
	}
	if !strings.EqualFold(strings.TrimSpace(journalMode), "wal") {
		return fmt.Errorf("set sqlite journal mode: unexpected mode %q", journalMode)
	}

	return nil
}

func supportsSQLiteWAL(dsn string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(dsn))
	return trimmed != sqliteMemoryDSN &&
		!strings.Contains(trimmed, "mode=memory") &&
		!strings.Contains(trimmed, "cache=shared&mode=memory") &&
		!strings.Contains(trimmed, "mode=ro") &&
		!strings.Contains(trimmed, "immutable=1")
}
