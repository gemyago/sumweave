package auth

import (
	"crypto/sha256"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gofrs/uuid/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenStore(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) *AccessTokenStore {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		sqlDB, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewAccessTokenStore(AccessTokenStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "sumweave_", Logger: telemetry.RootTestLogger(),
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}
	makeToken := func(t *testing.T, userID string, name string, now time.Time) AccessToken {
		t.Helper()
		id, err := uuid.NewV7()
		require.NoError(t, err)
		secret := []byte(fake.Lorem().Text(64))
		digest := sha256.Sum256(secret)
		return AccessToken{
			ID: id.String(), UserID: userID, Name: name, Permission: AccessTokenPermissionReadOnly,
			SecretHash: digest[:], CreatedAt: now, UpdatedAt: now,
		}
	}
	newUUID := func(t *testing.T) string {
		t.Helper()
		id, err := uuid.NewV7()
		require.NoError(t, err)
		return id.String()
	}

	t.Run("creates, lists newest first, and gets token rows by ID", func(t *testing.T) {
		store := makeStore(t)
		userID := newUUID(t)
		now := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
		first := makeToken(t, userID, "first-"+fake.UUID().V4(), now)
		second := makeToken(t, userID, "second-"+fake.UUID().V4(), now.Add(time.Second))
		require.NoError(t, store.Create(t.Context(), first))
		require.NoError(t, store.Create(t.Context(), second))

		got, err := store.GetByID(t.Context(), first.ID)
		require.NoError(t, err)
		require.Equal(t, first.ID, got.ID)
		require.Equal(t, first.UserID, got.UserID)
		require.Equal(t, first.SecretHash, got.SecretHash)
		listed, err := store.ListByUserID(t.Context(), userID)
		require.NoError(t, err)
		found := make([]AccessToken, 0, 2)
		for _, token := range listed {
			if token.ID == first.ID || token.ID == second.ID {
				found = append(found, token)
			}
		}
		require.Equal(t, []string{second.ID, first.ID}, []string{found[0].ID, found[1].ID})
		_, err = store.GetByID(t.Context(), newUUID(t))
		require.ErrorIs(t, err, ErrAccessTokenNotFound)
	})

	t.Run("enforces active owner names, permits revoked name reuse, and isolates owners", func(t *testing.T) {
		store := makeStore(t)
		firstOwner, secondOwner := newUUID(t), newUUID(t)
		now := time.Now().Truncate(time.Microsecond)
		name := "shared-" + fake.UUID().V4()
		first := makeToken(t, firstOwner, name, now)
		require.NoError(t, store.Create(t.Context(), first))
		duplicate := makeToken(t, firstOwner, name, now.Add(time.Second))
		require.ErrorIs(t, store.Create(t.Context(), duplicate), ErrAccessTokenNameExists)
		otherOwner := makeToken(t, secondOwner, name, now.Add(time.Second))
		require.NoError(t, store.Create(t.Context(), otherOwner))
		require.NoError(t, store.Revoke(t.Context(), firstOwner, first.ID, now.Add(2*time.Second)))
		reused := makeToken(t, firstOwner, name, now.Add(3*time.Second))
		require.NoError(t, store.Create(t.Context(), reused))
		otherTokens, err := store.ListByUserID(t.Context(), secondOwner)
		require.NoError(t, err)
		require.Contains(t, otherTokens, otherOwner)
		require.NotContains(t, otherTokens, reused)
	})

	t.Run("revokes owned tokens idempotently and hides non-owned IDs", func(t *testing.T) {
		store := makeStore(t)
		owner, otherOwner := newUUID(t), newUUID(t)
		now := time.Now().Truncate(time.Microsecond)
		token := makeToken(t, owner, "revoke-"+fake.UUID().V4(), now)
		require.NoError(t, store.Create(t.Context(), token))
		firstRevocation := now.Add(time.Second)
		require.NoError(t, store.Revoke(t.Context(), owner, token.ID, firstRevocation))
		require.NoError(t, store.Revoke(t.Context(), owner, token.ID, firstRevocation.Add(time.Second)))
		stored, err := store.GetByID(t.Context(), token.ID)
		require.NoError(t, err)
		require.True(t, firstRevocation.Equal(*stored.RevokedAt))
		require.ErrorIs(t, store.Revoke(t.Context(), otherOwner, token.ID, firstRevocation), ErrAccessTokenNotFound)
	})

	t.Run("serializes rotation and rolls back revocation when replacement insertion fails", func(t *testing.T) {
		store := makeStore(t)
		owner := newUUID(t)
		now := time.Now().Truncate(time.Microsecond)
		original := makeToken(t, owner, "rotate-"+fake.UUID().V4(), now)
		require.NoError(t, store.Create(t.Context(), original))
		const attempts = 8
		results := make(chan error, attempts)
		replacementIDs := make([]string, attempts)
		for index := range replacementIDs {
			replacementIDs[index] = newUUID(t)
		}
		var group sync.WaitGroup
		for index := range attempts {
			group.Go(func() {
				digest := sha256.Sum256([]byte("replacement-" + replacementIDs[index]))
				_, err := store.Rotate(t.Context(), RotateAccessTokenParams{
					UserID:             owner,
					TokenID:            original.ID,
					ReplacementTokenID: replacementIDs[index],
					SecretHash:         digest[:],
					Now:                now.Add(time.Second),
				})
				results <- err
			})
		}
		group.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIs(t, err, ErrAccessTokenConflict)
		}
		require.Equal(t, 1, successes)

		rollbackOriginal := makeToken(t, owner, "rollback-"+fake.UUID().V4(), now)
		require.NoError(t, store.Create(t.Context(), rollbackOriginal))
		_, err := store.Rotate(t.Context(), RotateAccessTokenParams{
			UserID:             owner,
			TokenID:            rollbackOriginal.ID,
			ReplacementTokenID: newUUID(t),
			SecretHash:         []byte(fake.Lorem().Text(31)),
			Now:                now.Add(time.Second),
		})
		require.Error(t, err)
		stored, err := store.GetByID(t.Context(), rollbackOriginal.ID)
		require.NoError(t, err)
		require.Nil(t, stored.RevokedAt)
	})

	t.Run("runs a shallow migration smoke", func(t *testing.T) {
		store := makeStore(t)
		require.True(t, store.db.Migrator().HasTable(&accessTokenModel{}))
	})

	t.Run("validates dependencies and propagates persistence failures", func(t *testing.T) {
		_, err := NewAccessTokenStore(AccessTokenStoreDeps{})
		require.ErrorContains(t, err, "access token store logger is required")
		_, err = NewAccessTokenStore(AccessTokenStoreDeps{Logger: telemetry.RootTestLogger()})
		require.ErrorContains(t, err, "auth sql database is required")
		store := makeStore(t)
		owner := newUUID(t)
		now := time.Now().Truncate(time.Microsecond)
		token := makeToken(t, owner, "closed-"+fake.UUID().V4(), now)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		require.Error(t, store.AutoMigrate())
		_, err = store.ListByUserID(t.Context(), newUUID(t))
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrAccessTokenNotFound)
		require.Error(t, store.Create(t.Context(), token))
		_, err = store.GetByID(t.Context(), token.ID)
		require.Error(t, err)
		require.Error(t, store.Revoke(t.Context(), owner, token.ID, now))
		replacementID := newUUID(t)
		_, err = store.Rotate(t.Context(), RotateAccessTokenParams{
			UserID:             owner,
			TokenID:            token.ID,
			ReplacementTokenID: replacementID,
			SecretHash:         token.SecretHash,
			Now:                now,
		})
		require.Error(t, err)
	})

	t.Run("rejects malformed identifiers before database access", func(t *testing.T) {
		store := makeStore(t)
		ownerID := newUUID(t)
		token := makeToken(t, ownerID, "invalid-id-"+fake.UUID().V4(), time.Now())

		_, err := store.GetByID(t.Context(), "not-a-uuid")
		require.ErrorIs(t, err, ErrAccessTokenNotFound)
		_, err = store.ListByUserID(t.Context(), "not-a-uuid")
		require.Error(t, err)
		require.ErrorIs(t, store.Revoke(t.Context(), "not-a-uuid", token.ID, time.Now()), ErrAccessTokenNotFound)
		require.ErrorIs(t, store.Revoke(t.Context(), ownerID, "not-a-uuid", time.Now()), ErrAccessTokenNotFound)
		_, err = store.Rotate(t.Context(), RotateAccessTokenParams{
			UserID: "not-a-uuid", TokenID: token.ID, ReplacementTokenID: newUUID(t),
			SecretHash: token.SecretHash, Now: time.Now(),
		})
		require.ErrorIs(t, err, ErrAccessTokenNotFound)
		_, err = store.Rotate(t.Context(), RotateAccessTokenParams{
			UserID: ownerID, TokenID: "not-a-uuid", ReplacementTokenID: newUUID(t),
			SecretHash: token.SecretHash, Now: time.Now(),
		})
		require.ErrorIs(t, err, ErrAccessTokenNotFound)

		_, err = accessTokenToModel(token)
		require.NoError(t, err)
		token.ID = "not-a-uuid"
		_, err = accessTokenToModel(token)
		require.Error(t, err)
		token.ID = newUUID(t)
		token.UserID = "not-a-uuid"
		_, err = accessTokenToModel(token)
		require.Error(t, err)
	})
}
