package wireup

import (
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildJobs(t *testing.T) {
	fake := faker.New()

	t.Run("composes migrated SQLite jobs and finance before exposing execution capabilities", func(t *testing.T) {
		applicationDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", applicationDSN)
		migration, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NoError(t, migration.Migrate(t.Context()))

		root, err := BuildJobs(t.Context(), JobsOptions{Environment: "test"})
		require.NoError(t, err)
		require.NotNil(t, root.Worker)
		require.NotNil(t, root.Scheduler)

		_, err = root.Registry.Handler(jobspkg.JobType(financepkg.FXRefreshJobType))
		require.NoError(t, err)
		_, err = root.Registry.Handler(jobspkg.JobType(financepkg.CSVImportJobTypeTransactions))
		require.NoError(t, err)
		_, err = root.Registry.Handler(jobspkg.JobType(financepkg.BankConnectionSyncJobType))
		require.NoError(t, err)

		schedule, err := root.Store.GetSchedule(t.Context(), financepkg.FXDailyRefreshScheduleID)
		require.NoError(t, err)
		require.True(t, schedule.Enabled)
		enqueued, err := root.Scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		require.Equal(t, 1, enqueued)
		jobs, err := root.Store.List(t.Context(), jobspkg.ListParams{
			JobTypes: []jobspkg.JobType{jobspkg.JobType(financepkg.FXRefreshJobType)},
		})
		require.NoError(t, err)
		require.Len(t, jobs.Items, 1)
		require.NoError(t, root.Close(t.Context()))
		require.NoError(t, root.Close(t.Context()))
	})

	t.Run("rejects root settings before opening jobs resources", func(t *testing.T) {
		_, err := BuildJobs(t.Context(), JobsOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "production"})
		require.NoError(t, err)
		_, err = values.JobsRoot("production")
		require.ErrorContains(t, err, "application database dsn")
	})
}
