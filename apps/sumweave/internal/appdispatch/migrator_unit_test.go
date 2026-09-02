package appdispatch

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestMigratorUnit(t *testing.T) {
	fake := faker.New()

	t.Run("selects a runner without executing schema DDL", func(t *testing.T) {
		postgresMigrator, err := NewMigrator(Config{DatabaseDSN: "postgres://" + fake.UUID().V4()}, &sql.DB{})
		require.NoError(t, err)
		require.IsType(t, postgresMigrationRunner{}, postgresMigrator.runner)

		postgresOnlyMigrator, err := NewMigrator(Config{}, &sql.DB{})
		require.NoError(t, err)
		require.IsType(t, postgresMigrationRunner{}, postgresOnlyMigrator.runner)
		_, err = NewMigrator(Config{}, nil)
		require.EqualError(t, err, "sql database is required")
	})

	t.Run("delegates migration orchestration to the configured runner", func(t *testing.T) {
		runner := newMockmigrationRunner(t)
		migrator := &Migrator{runner: runner}
		runner.EXPECT().Migrate(t.Context()).Return(nil).Once()
		require.NoError(t, migrator.Migrate(t.Context()))

		expected := errors.New(fake.UUID().V4())
		runner.EXPECT().Migrate(t.Context()).Return(expected).Once()
		require.ErrorIs(t, migrator.Migrate(t.Context()), expected)
	})

	t.Run("runs concrete PostgreSQL migration through a mock transaction", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		config := Config{DatabaseDSN: "postgres://" + fake.UUID().V4(), TablePrefix: "postgres_"}
		queries, err := buildPostgresMigrationQueries(config)
		require.NoError(t, err)
		databaseMock.ExpectBegin()
		for _, query := range queries {
			databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectExec("CREATE UNIQUE INDEX").WillReturnResult(sqlmock.NewResult(0, 0))
		databaseMock.ExpectCommit()
		require.NoError(t, AutoMigrate(t.Context(), config, db))
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})

	t.Run("returns concrete PostgreSQL migration transaction errors", func(t *testing.T) {
		config := Config{DatabaseDSN: "postgres://" + fake.UUID().V4(), TablePrefix: "postgres_"}
		makeMigrator := func(t *testing.T) (*Migrator, sqlmock.Sqlmock) {
			t.Helper()
			db, databaseMock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			return &Migrator{config: config, db: db}, databaseMock
		}

		t.Run("begin", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectBegin().WillReturnError(expectedErr)
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
		t.Run("schema query", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			queries, err := buildPostgresMigrationQueries(config)
			require.NoError(t, err)
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectBegin()
			databaseMock.ExpectExec(regexp.QuoteMeta(queries[0].Query)).WillReturnError(expectedErr)
			databaseMock.ExpectRollback()
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
		t.Run("payload hash", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			queries, err := buildPostgresMigrationQueries(config)
			require.NoError(t, err)
			databaseMock.ExpectBegin()
			for _, query := range queries {
				databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectExec("ALTER TABLE").WillReturnError(expectedErr)
			databaseMock.ExpectRollback()
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
		t.Run("message id deduplication", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			queries, err := buildPostgresMigrationQueries(config)
			require.NoError(t, err)
			databaseMock.ExpectBegin()
			for _, query := range queries {
				databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectExec("DELETE FROM").WillReturnError(expectedErr)
			databaseMock.ExpectRollback()
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
		t.Run("message id unique index", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			queries, err := buildPostgresMigrationQueries(config)
			require.NoError(t, err)
			databaseMock.ExpectBegin()
			for _, query := range queries {
				databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectExec("CREATE UNIQUE INDEX").WillReturnError(expectedErr)
			databaseMock.ExpectRollback()
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
		t.Run("commit", func(t *testing.T) {
			migrator, databaseMock := makeMigrator(t)
			queries, err := buildPostgresMigrationQueries(config)
			require.NoError(t, err)
			databaseMock.ExpectBegin()
			for _, query := range queries {
				databaseMock.ExpectExec(regexp.QuoteMeta(query.Query)).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			databaseMock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
			databaseMock.ExpectExec("CREATE UNIQUE INDEX").WillReturnResult(sqlmock.NewResult(0, 0))
			expectedErr := errors.New(fake.UUID().V4())
			databaseMock.ExpectCommit().WillReturnError(expectedErr)
			require.ErrorIs(t, migrator.migratePostgres(t.Context()), expectedErr)
		})
	})
}
