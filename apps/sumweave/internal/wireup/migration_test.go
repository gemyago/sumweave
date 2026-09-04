package wireup

import (
	"strings"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildMigration(t *testing.T) {
	fake := faker.New()
	t.Chdir("../..")

	t.Run("builds and runs the direct migration root for the prepared PostgreSQL schema", func(t *testing.T) {
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "test"})
		require.NoError(t, err)
		rootConfig, err := values.MigrationRoot("test")
		require.NoError(t, err)
		migrationDSN := strings.Replace(
			rootConfig.Application.Database.DSN,
			"sumweave_runtime:sumweave_runtime_local",
			"sumweave_migrator:sumweave_migrator_local",
			1,
		)
		t.Setenv("APP_APPLICATION_DATABASE_DSN", migrationDSN)
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", migrationDSN)
		root, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NotNil(t, root.migrator)
		require.NoError(t, root.Migrate(t.Context()))
	})

	t.Run("uses the local default environment for the prepared PostgreSQL schema", func(t *testing.T) {
		root, err := BuildMigration(t.Context(), MigrationOptions{})
		require.NoError(t, err)
		require.NoError(t, root.shutdownHooks.PerformShutdown(t.Context()))
	})

	t.Run("reports typed configuration load and validation failures", func(t *testing.T) {
		_, err := BuildMigration(t.Context(), MigrationOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", "")
		_, err = BuildMigration(t.Context(), MigrationOptions{Environment: "production"})
		require.ErrorContains(t, err, "application database dsn")
	})
}
