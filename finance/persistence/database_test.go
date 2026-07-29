package persistence

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNewDatabase(t *testing.T) {
	t.Run("rejects missing dsn", func(t *testing.T) {
		_, err := NewDatabase(nil, "   ")
		require.Error(t, err)
	})

	t.Run("opens sqlite database", func(t *testing.T) {
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NotNil(t, database)
		require.NotNil(t, database.db)
	})

	t.Run("requires sql database", func(t *testing.T) {
		_, err := NewDatabase(nil, fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4()))
		require.Error(t, err)
	})

	t.Run("configures slog-backed gorm logger when requested", func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		database, err := NewDatabase(sqlDB, dsn, WithLogger(logger))
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

	t.Run("builds postgres timestamp query chains", func(t *testing.T) {
		db, err := gorm.Open(
			postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=test dbname=test sslmode=disable"}),
			&gorm.Config{DisableAutomaticPing: true, DryRun: true},
		)
		require.NoError(t, err)

		now := time.Now()
		query := db.Table("events").
			Where(instantRangePredicate(db, "event_time"), now, now.Add(time.Hour))
		query = applyInstantAtOrAfter(query, "start_at", now)
		query = applyInstantAtOrBefore(query, "end_at", now.Add(time.Hour))
		query = query.Where(expiresAfterPredicate(db), now).Find(&[]struct{}{})
		require.NoError(t, query.Error)
	})
}
