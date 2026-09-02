//go:build postgres_test

package agent

import (
	"os"
	"testing"

	"github.com/gemyago/sumweave/runtime/internal"
	lp "github.com/gemyago/sumweave/runtime/internal/llmproviders"
	"github.com/stretchr/testify/require"
)

const postgresTestTablePrefix = "sumweave_runtime_"

func postgresTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	require.NotEmpty(t, dsn, "SUMWEAVE_POSTGRES_TEST_DSN is required for postgres_test")
	return dsn
}

func TestDatabaseServices(t *testing.T) {
	logger := internal.RootTestLogger()
	dsn := postgresTestDSN(t)

	t.Run("database agent profiles service uses prepared runtime tables", func(t *testing.T) {
		svc, err := NewDatabaseAgentProfilesService(dsn, logger, postgresTestTablePrefix)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})

	t.Run("database providers config service uses prepared runtime tables", func(t *testing.T) {
		svc, err := NewDatabaseProvidersConfigService(dsn, logger, postgresTestTablePrefix)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})

	t.Run("runner uses prepared database storage", func(t *testing.T) {
		profiles, err := NewDatabaseAgentProfilesService(dsn, logger, postgresTestTablePrefix)
		require.NoError(t, err)

		runner, err := NewRunner(RunnerArgs{
			ProvidersConfigService: lp.NewMockProvidersConfigService(t),
			AgentProfilesService:   profiles,
		},
			WithLogger(logger),
			WithDatabaseStorage(dsn),
			WithDatabaseTablePrefix(postgresTestTablePrefix),
		)
		require.NoError(t, err)
		require.NotNil(t, runner)
	})
}
