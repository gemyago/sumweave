package wireup

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildMigration(t *testing.T) {
	fake := faker.New()

	t.Run("migrates without JWT or finance provider configuration", func(t *testing.T) {
		applicationDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		agentRuntimeDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		root, err := buildMigration(t.Context(), config.MigrationRootConfig{
			Environment:             "test",
			DefaultLogLevel:         "INFO",
			GracefulShutdownTimeout: time.Second,
			AgentRuntime: config.AgentRuntime{
				Storage:  config.AgentRuntimeStorage{Type: "database"},
				Database: config.Database{DSN: agentRuntimeDSN, TablePrefix: "runtime_"},
			},
			Application: config.Application{
				Database: config.Database{DSN: applicationDSN, TablePrefix: "migration_"},
			},
		})
		require.NoError(t, err)
		require.NoError(t, root.Migrate(t.Context()))

		database, err := sqlconn.Open(applicationDSN)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, database.Close()) })
		for _, table := range []string{"migration_auth_auth_users", "migration_jobs_jobs", "finance_tenants"} {
			var name string
			row := database.QueryRowContext(
				t.Context(),
				`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
				table,
			)
			require.NoError(t, row.Scan(&name))
			require.Equal(t, table, name)
		}
	})

	t.Run("loads typed configuration for the production root", func(t *testing.T) {
		applicationDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		agentRuntimeDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		t.Setenv("APP_APPLICATION_DATABASE_DSN", applicationDSN)
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", agentRuntimeDSN)
		root, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		shutdownErr := errors.New(fake.Lorem().Sentence(3))
		root.shutdownHooks.Register("test", func(_ context.Context) error { return shutdownErr })
		// The root always executes registered lifecycle cleanup after migrations.
		require.ErrorIs(t, root.Migrate(t.Context()), shutdownErr)
	})

	t.Run("cleans up before returning from a cancelled migration", func(t *testing.T) {
		applicationDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		agentRuntimeDSN := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
		root, err := buildMigration(t.Context(), config.MigrationRootConfig{
			Environment:             "test",
			DefaultLogLevel:         "INFO",
			GracefulShutdownTimeout: time.Second,
			AgentRuntime: config.AgentRuntime{
				Storage:  config.AgentRuntimeStorage{Type: "database"},
				Database: config.Database{DSN: agentRuntimeDSN, TablePrefix: "runtime_"},
			},
			Application: config.Application{
				Database: config.Database{DSN: applicationDSN, TablePrefix: "migration_"},
			},
		})
		require.NoError(t, err)
		cleanupResults := make(chan error, 1)
		root.shutdownHooks.Register("test-cleanup", func(ctx context.Context) error {
			cleanupResults <- ctx.Err()
			return nil
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.Error(t, root.Migrate(ctx))
		select {
		case cleanupErr := <-cleanupResults:
			require.NoError(t, cleanupErr)
		default:
			t.Fatal("migration returned before cleanup hook ran")
		}
	})

	t.Run("reports typed configuration load and validation failures", func(t *testing.T) {
		_, err := BuildMigration(t.Context(), MigrationOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		_, err = BuildMigration(t.Context(), MigrationOptions{Environment: "production"})
		require.ErrorContains(t, err, "application database dsn")
	})
}
