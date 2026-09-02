package wireup

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildMigration(t *testing.T) {
	fake := faker.New()

	t.Run("loads typed configuration before running the bootstrap-owned migration", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		root, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		shutdownErr := errors.New(fake.Lorem().Sentence(3))
		root.shutdownHooks.Register("test", func(_ context.Context) error { return shutdownErr })
		require.ErrorIs(t, root.shutdownHooks.PerformShutdown(t.Context()), shutdownErr)
	})

	t.Run("uses the local default environment for file runtime configuration", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		root, err := BuildMigration(t.Context(), MigrationOptions{})
		require.NoError(t, err)
		require.NoError(t, root.shutdownHooks.PerformShutdown(t.Context()))
	})

	t.Run("cleans up a file-runtime migration after application schema setup", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		root, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		require.NoError(t, root.Migrate(t.Context()))
	})

	t.Run("joins registered cleanup errors after file-runtime migration", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"))
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		root, err := BuildMigration(t.Context(), MigrationOptions{Environment: "test"})
		require.NoError(t, err)
		shutdownErr := errors.New(fake.Lorem().Sentence(3))
		root.shutdownHooks.Register("test", func(context.Context) error { return shutdownErr })
		require.ErrorIs(t, root.Migrate(t.Context()), shutdownErr)
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
