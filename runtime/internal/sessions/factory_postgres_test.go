//go:build postgres_test

package sessions

import (
	"log/slog"
	"testing"

	"github.com/gemyago/sumweave/runtime/internal/summarize"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session"
)

func TestNewSessionsStorageDatabase(t *testing.T) {
	fake := faker.New()
	deps := SessionServiceFactoryDeps{
		RootLogger:          slog.New(slog.DiscardHandler),
		DatabaseDSN:         postgresTestDSN(t),
		DatabaseTablePrefix: postgresTestTablePrefix,
		Summarizer:          summarize.NewTruncatingSummarizer(),
	}

	t.Run("database type selects the prepared database backend", func(t *testing.T) {
		storage, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:          deps.RootLogger,
			SessionStorageType:  "database",
			DatabaseDSN:         deps.DatabaseDSN,
			DatabaseTablePrefix: deps.DatabaseTablePrefix,
			Summarizer:          deps.Summarizer,
		})
		require.NoError(t, err)
		_, ok := metadataSyncInner(t, storage).(*DatabaseSessionsStorage)
		require.True(t, ok)

		created, err := storage.Create(t.Context(), &session.CreateRequest{
			AppName:   fake.Lorem().Word(),
			UserID:    fake.UUID().V4(),
			SessionID: fake.UUID().V4(),
		})
		require.NoError(t, err)
		require.NotNil(t, created)
	})

	t.Run("database flag selects the prepared database backend", func(t *testing.T) {
		storage, err := NewSessionsStorage(SessionServiceFactoryDeps{
			RootLogger:          deps.RootLogger,
			UseDatabaseStorage:  true,
			DatabaseDSN:         deps.DatabaseDSN,
			DatabaseTablePrefix: deps.DatabaseTablePrefix,
			Summarizer:          deps.Summarizer,
		})
		require.NoError(t, err)
		_, ok := metadataSyncInner(t, storage).(*DatabaseSessionsStorage)
		require.True(t, ok)
	})
}
