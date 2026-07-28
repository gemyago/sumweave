package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Migrator owns the app dispatch schema setup flow.
type Migrator struct {
	config Config
	db     *sql.DB
}

func NewMigrator(config Config, db *sql.DB) (*Migrator, error) {
	config = config.normalize()
	if db == nil {
		return nil, errors.New("sql database is required")
	}
	return &Migrator{config: config, db: db}, nil
}

func AutoMigrate(ctx context.Context, config Config, db *sql.DB) error {
	migrator, err := NewMigrator(config, db)
	if err != nil {
		return err
	}
	return migrator.Migrate(ctx)
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if m.config.Driver() == TransportDriverPostgres {
		return m.migratePostgres(ctx)
	}
	return m.migrateSQLite(ctx)
}

func (m *Migrator) migrateSQLite(ctx context.Context) error {
	for _, query := range buildSQLiteMigrationQueries(m.config) {
		if _, execErr := m.db.ExecContext(ctx, query); execErr != nil {
			return fmt.Errorf("migrate sqlite app dispatch transport: %w", execErr)
		}
	}
	return nil
}

func migratePostgres(ctx context.Context, db *sql.DB, config Config) error {
	migrator, err := NewMigrator(config, db)
	if err != nil {
		return err
	}
	return migrator.migratePostgres(ctx)
}

func (m *Migrator) migratePostgres(ctx context.Context) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin postgres transport migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries, err := buildPostgresMigrationQueries(m.config)
	if err != nil {
		return err
	}
	for _, query := range queries {
		if _, err = tx.ExecContext(ctx, query.Query, query.Args...); err != nil {
			return fmt.Errorf("exec postgres transport migration query: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres transport migration: %w", err)
	}
	return nil
}
