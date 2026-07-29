package gormsumweave

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// GormSumweaveTablesOpts configures GORM for Sumweave-managed relational tables.
//
//nolint:revive // exported name intentionally mirrors the package helper family.
type GormSumweaveTablesOpts struct {
	// TablePrefix is passed to NamingStrategy.TablePrefix. Empty means no prefix.
	TablePrefix string
	// TranslateError enables GORM dialect error translation (for example ErrRecordNotFound).
	TranslateError bool
	// Logger enables project slog-backed GORM logging when provided.
	Logger *slog.Logger
}

// NewGormConfigForSumweaveTables returns a shared GORM config for session storage, provider config,
// and other Sumweave database-backed services so physical table names use the same prefix.
func NewGormConfigForSumweaveTables(opts GormSumweaveTablesOpts) *gorm.Config {
	cfg := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: opts.TablePrefix},
		TranslateError: opts.TranslateError,
	}

	if opts.Logger != nil {
		cfg.Logger = gormlogger.NewSlogLogger(opts.Logger.WithGroup("gorm"), gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		})
	}

	return cfg
}
