//go:build !release

package signalfoundry_test

import (
	"testing"

	signalfoundry "github.com/gemyago/signal-foundry/apps/signal-foundry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine(t *testing.T) {
	t.Run("GetToolsRegistry", func(t *testing.T) {
		t.Run("returns non-nil registry from DI container", func(t *testing.T) {
			engine, err := signalfoundry.NewEngine(signalfoundry.WithEngineEnv("test"))
			require.NoError(t, err)
			require.NotNil(t, engine)

			reg, err := engine.GetToolsRegistry()
			require.NoError(t, err)
			assert.NotNil(t, reg)
		})

		t.Run("returns same registry instance on multiple calls", func(t *testing.T) {
			engine, err := signalfoundry.NewEngine(signalfoundry.WithEngineEnv("test"))
			require.NoError(t, err)
			require.NotNil(t, engine)

			reg1, err := engine.GetToolsRegistry()
			require.NoError(t, err)

			reg2, err := engine.GetToolsRegistry()
			require.NoError(t, err)

			assert.Same(t, reg1, reg2)
		})
	})
}
