package gormsignalfoundry

import (
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// GormSignalFoundryTablesOpts configures GORM for Signal Foundry-managed relational tables.
//
//nolint:revive // exported name intentionally mirrors the package helper family.
type GormSignalFoundryTablesOpts struct {
	// TablePrefix is passed to NamingStrategy.TablePrefix. Empty means no prefix.
	TablePrefix string
	// TranslateError enables GORM dialect error translation (for example ErrRecordNotFound).
	TranslateError bool
}

// NewGormConfigForSignalFoundryTables returns a shared GORM config for session storage, provider config,
// and other Signal Foundry database-backed services so physical table names use the same prefix.
func NewGormConfigForSignalFoundryTables(opts GormSignalFoundryTablesOpts) *gorm.Config {
	return &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: opts.TablePrefix},
		TranslateError: opts.TranslateError,
	}
}
