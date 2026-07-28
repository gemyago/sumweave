package sqlconn

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("rejects empty dsn", func(t *testing.T) {
		db, err := Open("   ")
		require.Error(t, err)
		require.Nil(t, db)
		require.EqualError(t, err, "database dsn is required")
	})

	t.Run("configures sqlite busy timeout and file WAL mode", func(t *testing.T) {
		memoryDB, err := Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, memoryDB.Close()) }()
		var busyTimeout int
		require.NoError(t, memoryDB.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout))
		require.Equal(t, SQLiteBusyTimeoutMillis, busyTimeout)

		fileDB, err := Open(filepath.Join(t.TempDir(), "application.sqlite"))
		require.NoError(t, err)
		defer func() { require.NoError(t, fileDB.Close()) }()
		var journalMode string
		require.NoError(t, fileDB.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode))
		require.Equal(t, "wal", journalMode)
	})

	t.Run("opens postgres handles without sqlite defaults", func(t *testing.T) {
		db, err := Open("postgres://signal-foundry:secret@example.invalid:5432/signal_foundry?sslmode=disable")
		require.NoError(t, err)
		require.NoError(t, db.Close())
	})

	t.Run("surfaces sqlite setup failures from real connections", func(t *testing.T) {
		db, err := Open(":memory:")
		require.NoError(t, err)
		require.NoError(t, db.Close())
		require.ErrorContains(t, ApplySQLiteDefaults(db, ":memory:"), "set sqlite busy timeout")

		path := filepath.Join(t.TempDir(), "readonly.sqlite")
		writable, err := sql.Open("sqlite", path)
		require.NoError(t, err)
		_, err = writable.ExecContext(t.Context(), "CREATE TABLE test_records (id INTEGER PRIMARY KEY)")
		require.NoError(t, err)
		require.NoError(t, writable.Close())
		readOnly, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		require.NoError(t, err)
		defer func() { require.NoError(t, readOnly.Close()) }()
		require.ErrorContains(t, ApplySQLiteDefaults(readOnly, path), "set sqlite journal mode")
	})

	t.Run("rejects unexpected sqlite WAL responses", func(t *testing.T) {
		db, err := Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()
		require.EqualError(
			t,
			ApplySQLiteDefaults(db, filepath.Join(t.TempDir(), "application.sqlite")),
			`set sqlite journal mode: unexpected mode "memory"`,
		)
	})

	t.Run("recognizes supported sqlite DSN shapes", func(t *testing.T) {
		require.True(t, IsSQLiteDSN(":memory:"))
		require.True(t, SupportsSQLiteWAL(filepath.Join(t.TempDir(), "application.sqlite")))
		require.False(t, IsSQLiteDSN("postgres://example.invalid/database"))
		require.False(t, SupportsSQLiteWAL("file:application.sqlite?mode=ro"))
		require.NoError(t, ApplySQLiteDefaults(nil, ":memory:"))
	})
}
