package persistence

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestOpenDatabase(t *testing.T) {
	t.Run("rejects missing dsn", func(t *testing.T) {
		_, err := OpenDatabase("   ")
		require.Error(t, err)
	})

	t.Run("opens sqlite database", func(t *testing.T) {
		database, err := OpenDatabase(
			fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4()),
		)
		require.NoError(t, err)
		require.NotNil(t, database)
		require.NotNil(t, database.db)
	})

	t.Run("surfaces database open failures", func(t *testing.T) {
		_, err := OpenDatabase(fmt.Sprintf("%s/nope/test.sqlite", t.TempDir()))
		require.Error(t, err)
	})

	t.Run("configures slog-backed gorm logger when requested", func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		database, err := OpenDatabase(
			fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4()),
			WithLogger(logger),
		)
		require.NoError(t, err)
		require.NotNil(t, database)
		require.NotNil(t, database.db.Config.Logger)

		database.db.Config.Logger.Trace(
			context.Background(),
			time.Now().Add(-2*time.Second),
			func() (string, int64) { return "SELECT ? FROM accounts WHERE id = ?", 1 },
			nil,
		)

		require.Contains(t, logs.String(), `"gorm"`)
		require.Contains(t, logs.String(), `"sql":"SELECT ? FROM accounts WHERE id = ?"`)
	})
}
