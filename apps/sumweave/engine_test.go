//go:build !release

package sumweave_test

import (
	"testing"

	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine(t *testing.T) {
	makeEngine := func(t *testing.T) *sumweave.Engine {
		t.Helper()
		t.Setenv("APP_DATADIR", t.TempDir())
		engine, err := sumweave.NewEngine(sumweave.WithEngineEnv("test"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, engine.Close(t.Context())) })
		return engine
	}
	t.Run("GetToolsRegistry", func(t *testing.T) {
		t.Run("returns non-nil registry from explicit HTTP state", func(t *testing.T) {
			engine := makeEngine(t)
			require.NotNil(t, engine)

			reg, err := engine.GetToolsRegistry()
			require.NoError(t, err)
			assert.NotNil(t, reg)
		})

		t.Run("returns same registry instance on multiple calls", func(t *testing.T) {
			engine := makeEngine(t)
			require.NotNil(t, engine)

			reg1, err := engine.GetToolsRegistry()
			require.NoError(t, err)

			reg2, err := engine.GetToolsRegistry()
			require.NoError(t, err)

			assert.Same(t, reg1, reg2)
		})
	})

	t.Run("HTTP route and controller wireup", func(t *testing.T) {
		engine := makeEngine(t)
		require.NoError(t, engine.StartHTTPServer(t.Context(), sumweave.WithStartHTTPServerNoop(true)))
		require.NoError(t, engine.Close(t.Context()))
	})

	t.Run("Close releases a constructed engine without starting and is idempotent", func(t *testing.T) {
		engine := makeEngine(t)

		require.NoError(t, engine.Close(t.Context()))
		require.NoError(t, engine.Close(t.Context()))
	})

	t.Run("File-backed agent runtime storage", func(t *testing.T) {
		t.Setenv("APP_AGENTRUNTIME_STORAGE_TYPE", "file")
		engine := makeEngine(t)

		_, err := engine.GetToolsRegistry()
		require.NoError(t, err)
		require.NoError(t, engine.StartHTTPServer(t.Context(), sumweave.WithStartHTTPServerNoop(true)))
	})
}
