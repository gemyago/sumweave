package auth

import (
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenStore(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *RefreshTokenStore {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		sqlDB, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewRefreshTokenStore(RefreshTokenStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "sumweave_auth_", Logger: telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
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

		toDelete, err := store.Create(t.Context(), userID, time.Hour)
		require.NoError(t, err)
		require.NoError(t, store.DeleteAllForUser(t.Context(), userID))
		_, err = store.Consume(t.Context(), toDelete)
		require.ErrorIs(t, err, ErrInvalidRefreshToken)
	})

	t.Run("normalizes persisted timestamps to PostgreSQL microsecond precision", func(t *testing.T) {
		store := makeStore(t)
		location := time.FixedZone(fake.Lorem().Word(), 2*60*60)
		clockValue := time.Date(2026, time.September, 3, 19, 20, 30, 123456789, location)
		store.now = func() time.Time { return clockValue }
		ttl := time.Hour + 987

		token, err := store.Create(t.Context(), fake.UUID().V4(), ttl)
		require.NoError(t, err)
		var stored authRefreshTokenModel
		require.NoError(t, store.db.Where("token_hash = ?", hashToken(token)).First(&stored).Error)

		expectedCreatedAt := clockValue.Truncate(time.Microsecond)
		expectedExpiresAt := expectedCreatedAt.Add(ttl).Truncate(time.Microsecond)
		require.True(t, expectedCreatedAt.Equal(stored.CreatedAt))
		require.True(t, expectedExpiresAt.Equal(stored.ExpiresAt))
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
