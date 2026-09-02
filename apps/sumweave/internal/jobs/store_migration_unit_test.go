package jobs

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestStoreMigrationUnit(t *testing.T) {
	fake := faker.New()

	makeStore := func(t *testing.T) (*Store, *mockschemaMigrator) {
		t.Helper()
		migrator := newMockschemaMigrator(t)
		return &Store{tableName: "jobs_" + fake.UUID().V4(), migration: migrator}, migrator
	}

	t.Run("orchestrates bootstrap-owned schema cleanup", func(t *testing.T) {
		store, migrator := makeStore(t)
		migrator.EXPECT().AutoMigrate(store.tableName).Return(nil).Once()
		migrator.EXPECT().DropTableIfExists(store.scheduleTableName()).Return(nil).Once()
		for _, column := range []string{"agent_session_id", "agent_run_id", "idempotency_key", "canonical_input_hash", "input_json", "result_json", "progress_json", "max_attempts", "correlation_id"} {
			migrator.EXPECT().DropColumnIfExists(store.tableName, column).Return(nil).Once()
		}
		require.NoError(t, store.AutoMigrate())
	})

	t.Run("stops on the first bootstrap migration failure", func(t *testing.T) {
		store, migrator := makeStore(t)
		migrateErr := errors.New(fake.UUID().V4())
		migrator.EXPECT().AutoMigrate(store.tableName).Return(migrateErr).Once()
		require.ErrorIs(t, store.AutoMigrate(), migrateErr)

		store, migrator = makeStore(t)
		dropErr := errors.New(fake.UUID().V4())
		migrator.EXPECT().AutoMigrate(store.tableName).Return(nil).Once()
		migrator.EXPECT().DropTableIfExists(store.scheduleTableName()).Return(dropErr).Once()
		require.ErrorIs(t, store.AutoMigrate(), dropErr)

		store, migrator = makeStore(t)
		columnErr := errors.New(fake.UUID().V4())
		migrator.EXPECT().AutoMigrate(store.tableName).Return(nil).Once()
		migrator.EXPECT().DropTableIfExists(store.scheduleTableName()).Return(nil).Once()
		migrator.EXPECT().DropColumnIfExists(store.tableName, "agent_session_id").Return(columnErr).Once()
		require.ErrorIs(t, store.AutoMigrate(), columnErr)
	})

	t.Run("validates store inputs before accessing persistence", func(t *testing.T) {
		store := &Store{}
		_, err := NewStore(&sql.DB{}, "", StoreOpts{})
		require.ErrorContains(t, err, "database dsn is required")
		_, err = store.ClaimQueued(t.Context(), fake.UUID().V4(), fake.UUID().V4(), time.Time{})
		require.ErrorContains(t, err, "claimedAt")
		require.ErrorContains(t, store.RequeueRunning(t.Context(), Job{}, time.Time{}), "queuedAt")
		require.ErrorContains(t, store.RecoverStaleRunning(t.Context(), time.Time{}, time.Second, 1), "now")
	})
	t.Run("attaches the GORM schema adapter to a SQLMock-compatible database", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })

		store, err := NewStore(db, "postgres://"+fake.UUID().V4(), StoreOpts{TablePrefix: fake.Lorem().Word() + "_"})
		require.NoError(t, err)
		require.IsType(t, gormSchemaMigrator{}, store.migration)
		require.Error(t, store.migration.AutoMigrate(store.tableName))
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})

	t.Run("runs GORM schema cleanup through SQLMock", func(t *testing.T) {
		db, databaseMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		store, err := NewStore(db, "postgres://"+fake.UUID().V4(), StoreOpts{TablePrefix: fake.Lorem().Word() + "_"})
		require.NoError(t, err)
		migrator := gormSchemaMigrator{db: store.db}
		tableName := "obsolete_" + fake.UUID().V4()

		databaseMock.ExpectQuery("").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		databaseMock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 0))
		require.NoError(t, migrator.DropTableIfExists(tableName))

		dropErr := errors.New(fake.UUID().V4())
		databaseMock.ExpectQuery("").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		databaseMock.ExpectExec("").WillReturnError(dropErr)
		require.ErrorIs(t, migrator.DropTableIfExists(tableName), dropErr)

		databaseMock.ExpectQuery("").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		databaseMock.ExpectExec("").WillReturnResult(sqlmock.NewResult(0, 0))
		require.NoError(t, migrator.DropColumnIfExists(tableName, "obsolete"))
		require.NoError(t, databaseMock.ExpectationsWereMet())
	})
}
