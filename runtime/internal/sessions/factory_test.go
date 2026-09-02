package sessions

import (
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/runtime/internal/summarize"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestNewSessionsStorage(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	sum := summarize.NewTruncatingSummarizer()
	fake := faker.New()

	t.Run("type file: metadata sync wrapper, file metadata, SaveMetadata and ListMetadata", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:            logger,
			SessionStorageType:    "file",
			SessionStorageBaseDir: t.TempDir(),
			Summarizer:            sum,
		})
		require.NoError(t, err)
		require.NotNil(t, ss)

		innerWrap := metadataSyncInner(t, ss)
		fileInner, ok := innerWrap.(*FileSessionsStorage)
		require.True(t, ok)
		require.NotNil(t, fileInner)

		ctx := t.Context()
		app := fake.Lorem().Word()
		user := fake.UUID().V4()
		sid := fake.UUID().V4()
		now := time.Now().UTC().Truncate(time.Second)
		require.NoError(t, ss.SaveMetadata(ctx, SessionMetadata{
			SessionID: sid,
			AppName:   app,
			UserID:    user,
			Title:     "t",
			CreatedAt: now,
			UpdatedAt: now,
		}))
		listed, err := ss.ListMetadata(ctx, ListSessionMetadataParams{
			AppName: app,
			UserID:  user,
			Limit:   10,
			Offset:  0,
		})
		require.NoError(t, err)
		require.Equal(t, 1, listed.Total)
		require.Equal(t, sid, listed.Sessions[0].SessionID)
	})

	t.Run("type database rejects empty database DSN without connecting", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:         logger,
			SessionStorageType: "database",
			Summarizer:         sum,
		})
		require.Error(t, err)
		require.Nil(t, ss)
	})

	t.Run("empty SessionStorageType is memory: non-nil and AutoMigrate no-op", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger: logger,
			Summarizer: sum,
		})
		require.NoError(t, err)
		require.NotNil(t, ss)
		require.NoError(t, ss.AutoMigrate())
		innerWrap := metadataSyncInner(t, ss)
		_, ok := innerWrap.(*MemorySessionsStorage)
		require.True(t, ok)
	})

	t.Run("UseFileStorage selects file backend", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:            logger,
			UseFileStorage:        true,
			SessionStorageBaseDir: t.TempDir(),
			Summarizer:            sum,
		})
		require.NoError(t, err)
		innerWrap := metadataSyncInner(t, ss)
		_, ok := innerWrap.(*FileSessionsStorage)
		require.True(t, ok)
	})

	t.Run("UseDatabaseStorage rejects empty database DSN without connecting", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:         logger,
			UseDatabaseStorage: true,
			Summarizer:         sum,
		})
		require.Error(t, err)
		require.Nil(t, ss)
	})

	t.Run("unsupported SessionStorageType returns error", func(t *testing.T) {
		ss, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:         logger,
			SessionStorageType: "invalid",
			Summarizer:         sum,
		})
		require.Error(t, err)
		require.Nil(t, ss)
		require.Contains(t, err.Error(), "unsupported value")
	})
}

// metadataSyncInner unwraps [NewSessionsStorage]'s [*MetadataSyncStorage] for tests in this package.
func metadataSyncInner(t *testing.T, ss SessionsStorage) SessionsStorage {
	t.Helper()
	wrap, ok := ss.(*MetadataSyncStorage)
	require.True(t, ok, "expected *MetadataSyncStorage")
	return wrap.inner
}
