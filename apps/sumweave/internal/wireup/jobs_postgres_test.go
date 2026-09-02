//go:build postgres_test

package wireup

import (
	"testing"

	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/stretchr/testify/require"
)

func TestPostgresProcessRoots(t *testing.T) {
	t.Chdir("../..")

	t.Run("worker builds observed finance handlers without HTTP or scheduler", func(t *testing.T) {
		root, err := BuildWorker(t.Context(), WorkerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		require.NotNil(t, root.Worker)
		require.NotNil(t, root.Registry)
		for _, topic := range []string{
			financepkg.FXRatesRefreshCommandTopic,
			financepkg.TransactionCSVImportCommandTopic,
			financepkg.BankConnectionSyncCommandTopic,
		} {
			_, handlerErr := root.Registry.Handler(topic)
			require.NoError(t, handlerErr)
		}
	})

	t.Run("scheduler builds prepared finance schedules without worker or HTTP", func(t *testing.T) {
		root, err := BuildScheduler(t.Context(), SchedulerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, root.Close(t.Context())) })
		require.NotNil(t, root)
	})
}
