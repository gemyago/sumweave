package auth

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestUserStore(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *UserStore {
		t.Helper()
		dsn := fmt.Sprintf("file:auth-users-%s?mode=memory&cache=shared", fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewUserStore(UserStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "test_auth_",
			IDGen: ident.NewDefaultGenerator(), Logger: telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	makeParams := func(prefix string) CreateUserParams {
		return CreateUserParams{Username: prefix + "-" + fake.Internet().User(), PasswordHash: fake.Lorem().Text(60)}
	}

	t.Run("creates indexed users, lists deterministically, and updates passwords", func(t *testing.T) {
		store := makeStore(t)
		firstParams := makeParams("first")
		secondParams := makeParams("second")
		first, err := store.Create(t.Context(), firstParams)
		require.NoError(t, err)
		second, err := store.Create(t.Context(), secondParams)
		require.NoError(t, err)
		byID, err := store.GetByID(t.Context(), first.ID)
		require.NoError(t, err)
		require.Equal(t, first.ID, byID.ID)
		require.Equal(t, first.Username, byID.Username)
		require.Equal(t, first.PasswordHash, byID.PasswordHash)
		require.True(t, first.CreatedAt.Equal(byID.CreatedAt))
		require.True(t, first.UpdatedAt.Equal(byID.UpdatedAt))
		byUsername, err := store.GetByUsername(t.Context(), second.Username)
		require.NoError(t, err)
		require.Equal(t, second.ID, byUsername.ID)
		require.Equal(t, second.Username, byUsername.Username)

		users, err := store.List(t.Context())
		require.NoError(t, err)
		require.Len(t, users, 2)
		require.LessOrEqual(t, users[0].Username, users[1].Username)
		newHash := fake.Lorem().Text(60)
		require.NoError(t, store.UpdatePassword(t.Context(), first.ID, newHash))
		updated, err := store.GetByID(t.Context(), first.ID)
		require.NoError(t, err)
		require.Equal(t, newHash, updated.PasswordHash)
		require.ErrorIs(t, store.UpdatePassword(t.Context(), fake.UUID().V4(), newHash), ErrUserNotFound)
	})

	t.Run("maps concurrent duplicate usernames to ErrUsernameExists", func(t *testing.T) {
		store := makeStore(t)
		params := makeParams("shared")
		const attempts = 8
		errorsByAttempt := make(chan error, attempts)
		var group sync.WaitGroup
		for range attempts {
			group.Go(func() { _, err := store.Create(t.Context(), params); errorsByAttempt <- err })
		}
		group.Wait()
		close(errorsByAttempt)
		successes := 0
		for err := range errorsByAttempt {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIs(t, err, ErrUsernameExists)
		}
		require.Equal(t, 1, successes)
	})

	t.Run("validates dependencies and maps missing users", func(t *testing.T) {
		_, err := NewUserStore(UserStoreDeps{})
		require.ErrorContains(t, err, "user id generator is required")
		_, err = NewUserStore(UserStoreDeps{IDGen: ident.NewDefaultGenerator()})
		require.ErrorContains(t, err, "auth user store logger is required")
		_, err = NewUserStore(UserStoreDeps{
			IDGen: ident.NewDefaultGenerator(), Logger: telemetry.RootTestLogger(),
		})
		require.ErrorContains(t, err, "auth sql database is required")

		store := makeStore(t)
		_, err = store.GetByID(t.Context(), fake.UUID().V4())
		require.ErrorIs(t, err, ErrUserNotFound)
		_, err = store.GetByUsername(t.Context(), fake.Internet().User())
		require.ErrorIs(t, err, ErrUserNotFound)
		users, err := store.List(t.Context())
		require.NoError(t, err)
		require.Empty(t, users)
	})

	t.Run("propagates database failures", func(t *testing.T) {
		store := makeStore(t)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		_, err = store.Create(t.Context(), makeParams("closed"))
		require.Error(t, err)
		_, err = store.GetByID(t.Context(), fake.UUID().V4())
		require.Error(t, err)
		_, err = store.GetByUsername(t.Context(), fake.Internet().User())
		require.Error(t, err)
		require.Error(t, store.UpdatePassword(t.Context(), fake.UUID().V4(), fake.Lorem().Text(60)))
		_, err = store.List(t.Context())
		require.Error(t, err)
		require.Error(t, store.AutoMigrate())
	})

	t.Run("validates connection settings before opening auth persistence", func(t *testing.T) {
		db, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		_, err = openAuthDatabase(db, ":memory:", "invalid-prefix-")
		require.Error(t, err)
		require.Error(t, validateTablePrefix("invalid-prefix-"))
		require.NoError(t, validateTablePrefix("auth_users_"))
	})
}
