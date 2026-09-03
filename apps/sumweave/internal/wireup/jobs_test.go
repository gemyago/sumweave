//go:build postgres_test

package wireup

import (
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildProcessRoots(t *testing.T) {
	fake := faker.New()
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
		_, err = root.EnqueueDue(t.Context())
		require.NoError(t, err)
	})

	t.Run("rejects root settings before opening process resources", func(t *testing.T) {
		_, err := BuildWorker(t.Context(), WorkerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = BuildScheduler(t.Context(), SchedulerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", "")
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "production"})
		require.NoError(t, err)
		_, err = values.WorkerRoot("production")
		require.ErrorContains(t, err, "application database dsn")
	})
}
