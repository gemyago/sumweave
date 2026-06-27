package persistence

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	db *gorm.DB
}

func OpenDatabase(dsn string) (*Database, error) {
	trimmedDSN := strings.TrimSpace(dsn)
	if trimmedDSN == "" {
		return nil, errors.New("database dsn is required")
	}

	dialector := postgres.Open(trimmedDSN)
	if trimmedDSN == ":memory:" ||
		strings.HasPrefix(trimmedDSN, "file:") ||
		strings.HasSuffix(trimmedDSN, ".db") ||
		strings.HasSuffix(trimmedDSN, ".sqlite") ||
		strings.Contains(trimmedDSN, "sqlite") {
		dialector = sqlite.Open(trimmedDSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open finance database: %w", err)
	}

	return &Database{db: db}, nil
}
