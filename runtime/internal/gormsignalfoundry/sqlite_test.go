package gormsignalfoundry

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stretchr/testify/require"
)

func TestApplySQLiteConnectionDefaults(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for nil and non-sqlite handles", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ApplySQLiteConnectionDefaults(nil, ":memory:"))
		require.NoError(t, ApplySQLiteConnectionDefaults(&gorm.DB{}, "postgres://example.invalid/db"))
	})

	t.Run("returns exec errors for closed sqlite handles", func(t *testing.T) {
		t.Parallel()

		db, err := gorm.Open(NewGormDialector(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = ApplySQLiteConnectionDefaults(db, ":memory:")
		require.ErrorContains(t, err, "set sqlite busy timeout")
	})

	t.Run("returns handle errors for sqlite DBs without a sql connector", func(t *testing.T) {
		t.Parallel()

		db, err := gorm.Open(NewGormDialector(sqliteMemoryDSN), &gorm.Config{})
		require.NoError(t, err)
		db.ConnPool = nil
		db.Statement.ConnPool = nil

		err = ApplySQLiteConnectionDefaults(db, sqliteMemoryDSN)
		require.ErrorContains(t, err, "resolve sqlite database handle")
	})

	t.Run("configures sqlite busy timeout", func(t *testing.T) {
		t.Parallel()

		db, err := gorm.Open(NewGormDialector(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, ApplySQLiteConnectionDefaults(db, ":memory:"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()

		var busyTimeout int
		require.NoError(t, sqlDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout))
		require.Equal(t, sqliteBusyTimeoutMillis, busyTimeout)
	})

	t.Run("uses WAL mode for file-backed sqlite handles", func(t *testing.T) {
		t.Parallel()

		dsn := filepath.Join(t.TempDir(), "sqlite-defaults.db")
		db, err := gorm.Open(NewGormDialector(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, ApplySQLiteConnectionDefaults(db, dsn))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()

		var journalMode string
		require.NoError(t, sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
		require.Equal(t, "wal", journalMode)
	})

	t.Run("skips WAL for readonly sqlite handles", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "readonly.db")
		db, err := gorm.Open(NewGormDialector(dbPath), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, ApplySQLiteConnectionDefaults(db, dbPath))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		readonlyDSN := (&url.URL{Scheme: "file", Path: dbPath, RawQuery: "mode=ro"}).String()
		readonlyDB, err := gorm.Open(NewGormDialector(readonlyDSN), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, ApplySQLiteConnectionDefaults(readonlyDB, readonlyDSN))
	})

	t.Run("surfaces WAL setup query errors for file-backed sqlite handles", func(t *testing.T) {
		t.Parallel()

		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectQuery("PRAGMA journal_mode = WAL").WillReturnError(assertiveTestError("wal boom"))

		db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
		require.NoError(t, err)
		err = applySQLiteWAL(db, filepath.Join(t.TempDir(), "wal-error.sqlite"))
		require.ErrorContains(t, err, "set sqlite journal mode")
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects unexpected WAL modes for file-backed sqlite handles", func(t *testing.T) {
		t.Parallel()

		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectQuery("PRAGMA journal_mode = WAL").WillReturnRows(
			sqlmock.NewRows([]string{"journal_mode"}).AddRow("delete"),
		)

		db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
		require.NoError(t, err)
		err = applySQLiteWAL(db, filepath.Join(t.TempDir(), "wal-mode.sqlite"))
		require.EqualError(t, err, `set sqlite journal mode: unexpected mode "delete"`)
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func assertiveTestError(message string) error {
	return &testError{message: message}
}

type testError struct{ message string }

func (e *testError) Error() string { return e.message }
