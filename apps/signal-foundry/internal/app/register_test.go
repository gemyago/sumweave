package app

import (
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/runtime/audit"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/stretchr/testify/require"
)

func TestPersistenceConstructorsDoNotAutoMigrate(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "app.sqlite")
	sharedDB, openErr := sqlconn.Open(dsn)
	require.NoError(t, openErr)
	t.Cleanup(func() { require.NoError(t, sharedDB.Close()) })

	deps := strategyWorkspaceStoreDeps{
		DatabaseDSN:         dsn,
		DatabaseTablePrefix: "signal_foundry_data_",
		SQLDB:               sharedDB,
	}

	t.Run("strategy artifact store", func(t *testing.T) {
		store, err := newStrategyArtifactStore(deps)
		require.NoError(t, err)

		_, err = store.List(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("strategy version registry", func(t *testing.T) {
		artifactStore, err := rtstrategy.NewArtifactDatabaseStore(
			deps.SQLDB,
			deps.DatabaseDSN,
			rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: deps.DatabaseTablePrefix + "strategy_"},
		)
		require.NoError(t, err)

		_, err = newStrategyVersionRegistryService(deps, artifactStore)
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("evaluation governor policy store", func(t *testing.T) {
		store, err := newEvaluationGovernorPolicyStore(deps)
		require.NoError(t, err)

		_, err = store.GetActive(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("evaluation audit store", func(t *testing.T) {
		store, err := newEvaluationAuditStore(deps)
		require.NoError(t, err)

		_, err = store.QueryTraces(t.Context(), audit.TraceQuery{})
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("evaluation execution store", func(t *testing.T) {
		store, err := newEvaluationExecutionStore(deps)
		require.NoError(t, err)

		_, err = store.GetCommand(t.Context(), "missing-command")
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})

	t.Run("evaluation backtest store", func(t *testing.T) {
		store, err := newEvaluationBacktestStore(deps)
		require.NoError(t, err)

		_, err = store.GetBacktestRun(t.Context(), "missing-backtest")
		require.Error(t, err)
		require.ErrorContains(t, err, "no such table")
	})
}
