package wireup

import (
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildProcessRoots(t *testing.T) {
	fake := faker.New()
	prepareApplicationSchemas := func(t *testing.T) {
		t.Helper()
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		migration, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NoError(t, migration.Migrate(t.Context()))
	}

	t.Run("worker and scheduler build after file-runtime application schema setup", func(t *testing.T) {
		prepareApplicationSchemas(t)
		worker, err := BuildWorker(t.Context(), WorkerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, worker.Close(t.Context())) })
		require.NotNil(t, worker.Worker)
		scheduler, err := BuildScheduler(t.Context(), SchedulerOptions{Environment: "test"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, scheduler.Close(t.Context())) })
		enqueued, err := scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, enqueued)
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
