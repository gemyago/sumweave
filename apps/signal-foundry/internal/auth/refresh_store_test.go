package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenStore(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *RefreshTokenStore {
		t.Helper()
		dsn := fmt.Sprintf("file:auth-refresh-%s?mode=memory&cache=shared", fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewRefreshTokenStore(RefreshTokenStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "test_auth_", Logger: telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}

	t.Run("stores only hashes and rejects expired tokens", func(t *testing.T) {
		store := makeStore(t)
		userID := fake.UUID().V4()
		token, err := store.Create(t.Context(), userID, time.Hour)
		require.NoError(t, err)
		var rawCount int64
		require.NoError(t, store.db.Model(&authRefreshTokenModel{}).
			Where("token_hash = ?", token).
			Count(&rawCount).Error)
		require.Zero(t, rawCount)
		consumedUserID, err := store.Consume(t.Context(), token)
		require.NoError(t, err)
		require.Equal(t, userID, consumedUserID)
		_, err = store.Consume(t.Context(), token)
		require.ErrorIs(t, err, ErrInvalidRefreshToken)

		expired, err := store.Create(t.Context(), fake.UUID().V4(), -time.Second)
		require.NoError(t, err)
		_, err = store.Consume(t.Context(), expired)
		require.ErrorIs(t, err, ErrInvalidRefreshToken)
	})

	t.Run("consumes one token exactly once under concurrency", func(t *testing.T) {
		store := makeStore(t)
		userID := fake.UUID().V4()
		token, err := store.Create(t.Context(), userID, time.Hour)
		require.NoError(t, err)
		const attempts = 8
		results := make(chan error, attempts)
		var group sync.WaitGroup
		for range attempts {
			group.Go(func() { _, consumeErr := store.Consume(t.Context(), token); results <- consumeErr })
		}
		group.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIs(t, err, ErrInvalidRefreshToken)
		}
		require.Equal(t, 1, successes)
	})

	t.Run("validates dependencies and propagates closed database errors", func(t *testing.T) {
		_, err := NewRefreshTokenStore(RefreshTokenStoreDeps{})
		require.ErrorContains(t, err, "refresh token store logger is required")
		_, err = NewRefreshTokenStore(RefreshTokenStoreDeps{Logger: telemetry.RootTestLogger()})
		require.ErrorContains(t, err, "auth sql database is required")

		store := makeStore(t)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		_, err = store.Create(t.Context(), fake.UUID().V4(), time.Hour)
		require.Error(t, err)
		_, err = store.Consume(t.Context(), fake.Lorem().Text(40))
		require.Error(t, err)
		require.Error(t, store.DeleteAllForUser(t.Context(), fake.UUID().V4()))
		require.Error(t, store.AutoMigrate())
	})
}
