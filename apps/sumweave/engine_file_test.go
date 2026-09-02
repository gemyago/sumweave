//go:build postgres_test

package sumweave_test

import (
	"testing"

	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/stretchr/testify/require"
)

func TestFileEngine(t *testing.T) {
	t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
	t.Setenv("APP_DATADIR", t.TempDir())
	engine, err := sumweave.NewEngine(sumweave.WithEngineEnv("test"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, engine.Close(t.Context())) })
	_, err = engine.GetToolsRegistry()
	require.NoError(t, err)
	require.NoError(t, engine.StartHTTPServer(t.Context(), sumweave.WithStartHTTPServerNoop(true)))
}
