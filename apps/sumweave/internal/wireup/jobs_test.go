package wireup

import (
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildProcessRoots(t *testing.T) {
	fake := faker.New()
	prepareSchemas := func(t *testing.T) {
		t.Helper()
		applicationDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", applicationDSN)
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		migration, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NoError(t, migration.Migrate(t.Context()))
	}

	t.Run("worker builds observed finance handlers without HTTP or scheduler", func(t *testing.T) {
		prepareSchemas(t)
		root, err := BuildWorker(t.Context(), WorkerOptions{Environment: "test"})
		require.NoError(t, err)
		defer func() { require.NoError(t, root.Close(t.Context())) }()
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

	t.Run("scheduler publishes due finance schedules without worker or HTTP", func(t *testing.T) {
		prepareSchemas(t)
		root, err := BuildScheduler(t.Context(), SchedulerOptions{Environment: "test"})
		require.NoError(t, err)
		defer func() { require.NoError(t, root.Close(t.Context())) }()
		enqueued, err := root.EnqueueDue(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, enqueued)
	})

	t.Run("rejects root settings before opening process resources", func(t *testing.T) {
		_, err := BuildWorker(t.Context(), WorkerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = BuildScheduler(t.Context(), SchedulerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "production"})
		require.NoError(t, err)
		_, err = values.WorkerRoot("production")
		require.ErrorContains(t, err, "application database dsn")
	})
}
