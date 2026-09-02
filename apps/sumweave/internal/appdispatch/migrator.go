package appdispatch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const payloadHashColumnName = "payload_hash"

// Migrator owns the app dispatch schema setup flow.
type Migrator struct {
	config Config
	db     *sql.DB
	runner migrationRunner
}

type migrationRunner interface {
	Migrate(context.Context) error
}

type postgresMigrationRunner struct{ migrator *Migrator }

func (r postgresMigrationRunner) Migrate(
	ctx context.Context,
) error {
	return r.migrator.migratePostgres(ctx)
}

type sqliteMigrationRunner struct{ migrator *Migrator }

func (r sqliteMigrationRunner) Migrate(
	ctx context.Context,
) error {
	return r.migrator.migrateSQLite(ctx)
}

func NewMigrator(config Config, db *sql.DB) (*Migrator, error) {
	config = config.normalize()
	if db == nil {
		return nil, errors.New("sql database is required")
	}
	migrator := &Migrator{config: config, db: db}
	if config.Driver() == TransportDriverPostgres {
		migrator.runner = postgresMigrationRunner{migrator: migrator}
	} else {
		migrator.runner = sqliteMigrationRunner{migrator: migrator}
	}
	return migrator, nil
}

func AutoMigrate(
	ctx context.Context,
	config Config,
	db *sql.DB,
) error {
	migrator, err := NewMigrator(config, db)
	if err != nil {
		return err
	}
	return migrator.Migrate(ctx)
}

func (m *Migrator) Migrate(ctx context.Context) error {
	return m.runner.Migrate(ctx)
}

func (m *Migrator) migrateSQLite(
	ctx context.Context,
) error {
	for _, query := range buildSQLiteMigrationQueries(m.config) {
		if _, execErr := m.db.ExecContext(ctx, query); execErr != nil {
			return fmt.Errorf("migrate sqlite app dispatch transport: %w", execErr)
		}
	}
	if err := m.ensureSQLitePayloadHash(ctx); err != nil {
		return err
	}
	if err := m.ensureSQLiteMessageIDUniqueness(ctx); err != nil {
		return err
	}
	return nil
}

func (m *Migrator) ensureSQLitePayloadHash(
	ctx context.Context,
) error {
	rows, err := m.db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(m.config.MessagesTable())+`)`)
	if err != nil {
		return fmt.Errorf("inspect sqlite app dispatch messages columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			columnID, notNull, primaryKey int
			name, columnType              string
			defaultValue                  sql.NullString
		)
		if err = rows.Scan(&columnID, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan sqlite app dispatch messages column: %w", err)
		}
		if name == payloadHashColumnName {
			return nil
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite app dispatch messages columns: %w", err)
	}
	if _, err = m.db.ExecContext(
		ctx,
		`ALTER TABLE `+quoteIdentifier(m.config.MessagesTable())+
			` ADD COLUMN `+payloadHashColumnName+` TEXT NOT NULL DEFAULT ''`,
	); err != nil {
		return fmt.Errorf("add sqlite app dispatch payload hash: %w", err)
	}
	return nil
}

func (m *Migrator) ensureSQLiteMessageIDUniqueness(
	ctx context.Context,
) error {
	messagesTable := quoteIdentifier(m.config.MessagesTable())
	//nolint:gosec // Table names derive from trusted application configuration.
	if _, err := m.db.ExecContext(
		ctx,
		`DELETE FROM `+messagesTable+` WHERE "offset" NOT IN (`+
			`SELECT earliest_offset FROM (SELECT MIN("offset") AS earliest_offset FROM `+messagesTable+` GROUP BY uuid))`,
	); err != nil {
		return fmt.Errorf("deduplicate sqlite app dispatch message ids: %w", err)
	}
	if _, err := m.db.ExecContext(
		ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS `+quoteIdentifier(m.config.MessagesTable()+"_uuid_uidx")+
			` ON `+messagesTable+` (uuid)`,
	); err != nil {
		return fmt.Errorf("enforce sqlite app dispatch message ids: %w", err)
	}
	return nil
}

func (m *Migrator) migratePostgres(
	ctx context.Context,
) error {
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
	if err = m.ensurePostgresPayloadHash(ctx, tx); err != nil {
		return err
	}
	if err = m.ensurePostgresMessageIDUniqueness(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit postgres transport migration: %w", err)
	}
	return nil
}

func (m *Migrator) ensurePostgresPayloadHash(
	ctx context.Context,
	tx *sql.Tx,
) error {
	if _, err := tx.ExecContext(
		ctx,
		`ALTER TABLE `+quoteIdentifier(m.config.MessagesTable())+
			` ADD COLUMN IF NOT EXISTS payload_hash VARCHAR(64) NOT NULL DEFAULT ''`,
	); err != nil {
		return fmt.Errorf("add postgres app dispatch payload hash: %w", err)
	}
	return nil
}

func (m *Migrator) ensurePostgresMessageIDUniqueness(
	ctx context.Context,
	tx *sql.Tx,
) error {
	messagesTable := quoteIdentifier(m.config.MessagesTable())
	//nolint:gosec // Table names derive from trusted application configuration.
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM `+messagesTable+` AS duplicate USING `+messagesTable+
			` AS original WHERE duplicate.uuid = original.uuid AND duplicate.ctid > original.ctid`,
	); err != nil {
		return fmt.Errorf("deduplicate postgres app dispatch message ids: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS `+quoteIdentifier(m.config.MessagesTable()+"_uuid_uidx")+
			` ON `+messagesTable+` (uuid)`,
	); err != nil {
		return fmt.Errorf("enforce postgres app dispatch message ids: %w", err)
	}
	return nil
}
