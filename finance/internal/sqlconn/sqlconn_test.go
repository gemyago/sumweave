package sqlconn

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("rejects missing dsn", func(t *testing.T) {
		db, err := Open("  ")
		require.EqualError(t, err, "database dsn is required")
		require.Nil(t, db)
	})

	t.Run("opens sqlite memory handle with busy timeout", func(t *testing.T) {
		db, err := Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		var busyTimeout int
		require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout))
		require.Equal(t, SQLiteBusyTimeoutMillis, busyTimeout)
	})

	t.Run("opens file sqlite handle with wal mode", func(t *testing.T) {
		dsn := filepath.Join(t.TempDir(), "shared.sqlite")
		db, err := Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		var journalMode string
		require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode))
		require.Equal(t, "wal", journalMode)
	})

	t.Run("opens postgres handles without sqlite defaults", func(t *testing.T) {
		db, err := Open("postgres://sumweave:secret@example.invalid:5432/sumweave?sslmode=disable")
		require.NoError(t, err)
		require.NoError(t, db.Close())
	})

	t.Run("surfaces sqlite open failures during setup", func(t *testing.T) {
		_, err := Open(filepath.Join(t.TempDir(), "missing", "broken.sqlite"))
		require.Error(t, err)
	})

	t.Run("sqlite helpers detect supported dsn shapes", func(t *testing.T) {
		require.True(t, IsSQLiteDSN(":memory:"))
		require.True(t, SupportsSQLiteWAL(filepath.Join(t.TempDir(), "wal.sqlite")))
		require.False(t, IsSQLiteDSN("postgres://example.invalid/db"))
		require.False(t, SupportsSQLiteWAL("file:test.sqlite?mode=ro"))
		require.NoError(t, ApplySQLiteDefaults(nil, ":memory:"))
	})

	t.Run("sqlite defaults surface exec failures on closed handles", func(t *testing.T) {
		db, err := Open(":memory:")
		require.NoError(t, err)
		require.NoError(t, db.Close())
		err = ApplySQLiteDefaults(db, ":memory:")
		require.ErrorContains(t, err, "set sqlite busy timeout")
	})

	t.Run("sqlite defaults surface wal setup errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "readonly.sqlite")
		writable, err := sql.Open("sqlite", path)
		require.NoError(t, err)
		_, err = writable.ExecContext(t.Context(), "CREATE TABLE test_records (id INTEGER PRIMARY KEY)")
		require.NoError(t, err)
		require.NoError(t, writable.Close())
		readOnly, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		require.NoError(t, err)
		defer func() { require.NoError(t, readOnly.Close()) }()

		err = ApplySQLiteDefaults(readOnly, path)
		require.ErrorContains(t, err, "set sqlite journal mode")
	})

	t.Run("sqlite defaults reject unexpected wal response", func(t *testing.T) {
		db, err := Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, db.Close()) }()

		err = ApplySQLiteDefaults(db, filepath.Join(t.TempDir(), "wal.sqlite"))
		require.EqualError(t, err, `set sqlite journal mode: unexpected mode "memory"`)
	})
}
