package data

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestLocalRawPayloadBlobStore(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	t.Run("stores and reads payload bodies with stable refs", func(t *testing.T) {
		t.Parallel()

		basePath := filepath.Join(t.TempDir(), randomWord("raw-payloads"))
		store, err := NewLocalRawPayloadBlobStore(basePath)
		require.NoError(t, err)

		payloadID := randomWord("payload")
		body := []byte(randomWord("body") + randomWord("suffix"))

		stored, err := store.StoreRawPayloadBody(t.Context(), payloadID, body)
		require.NoError(t, err)
		require.NotEmpty(t, stored.Ref)
		require.NotEmpty(t, stored.Hash)

		storedAgain, err := store.StoreRawPayloadBody(t.Context(), payloadID, []byte(randomWord("different")))
		require.NoError(t, err)
		require.Equal(t, stored, storedAgain)

		persistedBody, err := store.ReadRawPayloadBody(t.Context(), stored.Ref)
		require.NoError(t, err)
		require.Equal(t, body, persistedBody)

		info, err := os.Stat(filepath.Join(basePath, filepath.FromSlash(stored.Ref)))
		require.NoError(t, err)
		require.False(t, info.IsDir())

		blobDir := filepath.Dir(filepath.Join(basePath, filepath.FromSlash(stored.Ref)))
		entries, err := os.ReadDir(blobDir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, filepath.Base(stored.Ref), entries[0].Name())
	})

	t.Run("rejects invalid inputs and traversal refs", func(t *testing.T) {
		t.Parallel()

		store, err := NewLocalRawPayloadBlobStore(filepath.Join(t.TempDir(), randomWord("raw-payloads")))
		require.NoError(t, err)

		_, err = store.StoreRawPayloadBody(t.Context(), "", []byte(randomWord("body")))
		require.ErrorIs(t, err, ErrValidation)

		_, err = store.StoreRawPayloadBody(t.Context(), randomWord("payload"), nil)
		require.ErrorIs(t, err, ErrValidation)

		_, err = store.ReadRawPayloadBody(t.Context(), "../escape.blob")
		require.ErrorIs(t, err, ErrValidation)
	})

	t.Run("honors canceled contexts", func(t *testing.T) {
		t.Parallel()

		store, err := NewLocalRawPayloadBlobStore(filepath.Join(t.TempDir(), randomWord("raw-payloads")))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = store.StoreRawPayloadBody(ctx, randomWord("payload"), []byte(randomWord("body")))
		require.ErrorIs(t, err, context.Canceled)
	})
}
