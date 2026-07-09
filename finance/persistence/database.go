package persistence

import (
	"database/sql"
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

// NewDatabase builds a finance database wrapper from an existing [sql.DB] handle.
func NewDatabase(sqlDB *sql.DB, dsn string, opts ...OpenDatabaseOption) (*Database, error) {
	trimmedDSN := strings.TrimSpace(dsn)
	if trimmedDSN == "" {
		return nil, errors.New("database dsn is required")
	}
	if sqlDB == nil {
		return nil, errors.New("sql database is required")
	}
	cfg := openDatabaseConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	dialector := postgres.New(postgres.Config{DSN: dsn, Conn: sqlDB})
	if dsn == ":memory:" ||
		strings.HasPrefix(dsn, "file:") ||
		strings.HasSuffix(dsn, ".db") ||
		strings.HasSuffix(dsn, ".sqlite") ||
		strings.Contains(dsn, "sqlite") {
		dialector = sqlite.Dialector{DriverName: sqlite.DriverName, DSN: dsn, Conn: sqlDB}
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
