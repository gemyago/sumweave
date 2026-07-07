package persistence

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Database struct {
	db *gorm.DB
}

// OpenDatabaseOption configures optional finance database initialization behavior.
type OpenDatabaseOption func(*openDatabaseConfig)

type openDatabaseConfig struct {
	logger *slog.Logger
}

// WithLogger enables slog-backed GORM logging for the opened finance database.
func WithLogger(logger *slog.Logger) OpenDatabaseOption {
	return func(cfg *openDatabaseConfig) {
		cfg.logger = logger
	}
}

// OpenDatabase opens a finance persistence database using PostgreSQL or SQLite DSN detection.
func OpenDatabase(dsn string, opts ...OpenDatabaseOption) (*Database, error) {
	trimmedDSN := strings.TrimSpace(dsn)
	if trimmedDSN == "" {
		return nil, errors.New("database dsn is required")
	}
	cfg := openDatabaseConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	dialector := postgres.Open(trimmedDSN)
	if trimmedDSN == ":memory:" ||
		strings.HasPrefix(trimmedDSN, "file:") ||
		strings.HasSuffix(trimmedDSN, ".db") ||
		strings.HasSuffix(trimmedDSN, ".sqlite") ||
		strings.Contains(trimmedDSN, "sqlite") {
		dialector = sqlite.Open(trimmedDSN)
	}

	gormCfg := &gorm.Config{TranslateError: true}
	if cfg.logger != nil {
		gormCfg.Logger = gormlogger.NewSlogLogger(cfg.logger.WithGroup("gorm"), gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		})
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open finance database: %w", err)
	}

	return &Database{db: db}, nil
}
