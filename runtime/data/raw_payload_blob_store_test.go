package data

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDatabaseRawPayloadBlobStore(t *testing.T) {
	fake := faker.New()

	makeStore := func(t *testing.T) *DatabaseStore {
		t.Helper()
		dsn := fmt.Sprintf("file:raw-payload-%s?mode=memory&cache=shared", fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewDatabaseStore(sqlDB, dsn, DatabaseStoreOpts{TablePrefix: "raw_payload_test_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store
	}

	randomWord := func(prefix string) string { return prefix + "-" + strings.ToLower(fake.Lorem().Word()) }

	t.Run("stores immutable bodies and reads bounded previews", func(t *testing.T) {
		store := makeStore(t)
		payloadID := randomWord("payload")
		body := []byte(strings.Repeat(randomWord("body"), 32))

		stored, err := store.StoreRawPayloadBody(t.Context(), payloadID, body)
		require.NoError(t, err)
		storedAgain, err := store.StoreRawPayloadBody(t.Context(), payloadID, []byte(randomWord("different")))
		require.NoError(t, err)
		require.Equal(t, stored, storedAgain)

		persistedBody, err := store.ReadRawPayloadBody(t.Context(), stored.Ref)
		require.NoError(t, err)
		require.Equal(t, body, persistedBody)

		preview, err := store.readRawPayloadBodyPreview(t.Context(), stored.Ref, len(body)/3)
		require.NoError(t, err)
		require.Equal(t, len(body), preview.sizeBytes)
		require.Equal(t, body[:len(body)/3], preview.preview)
		require.True(t, preview.truncated)
	})

	t.Run("rejects invalid inputs and canceled contexts", func(t *testing.T) {
		store := makeStore(t)
		_, err := store.StoreRawPayloadBody(t.Context(), "", []byte(randomWord("body")))
		require.ErrorIs(t, err, ErrValidation)
		_, err = store.StoreRawPayloadBody(t.Context(), randomWord("payload"), nil)
		require.ErrorIs(t, err, ErrValidation)
		_, err = store.ReadRawPayloadBody(t.Context(), "")
		require.ErrorIs(t, err, ErrValidation)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = store.StoreRawPayloadBody(ctx, randomWord("payload"), []byte(randomWord("body")))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("preserves binary bodies with PostgreSQL when configured", func(t *testing.T) {
		dsn := os.Getenv("SIGNAL_FOUNDRY_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("SIGNAL_FOUNDRY_POSTGRES_DSN is not configured")
		}
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := NewDatabaseStore(sqlDB, dsn, DatabaseStoreOpts{
			TablePrefix: "raw_payload_pg_" + strings.ReplaceAll(fake.UUID().V4(), "-", "") + "_",
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		body := append([]byte(randomWord("binary")), 0, byte(fake.RandomNumber(255)))
		stored, err := store.StoreRawPayloadBody(t.Context(), randomWord("payload"), body)
		require.NoError(t, err)
		read, err := store.ReadRawPayloadBody(t.Context(), stored.Ref)
		require.NoError(t, err)
		require.Equal(t, body, read)
	})
}
