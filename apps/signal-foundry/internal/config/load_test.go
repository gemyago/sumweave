package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("loads environment defaults and reports missing resources", func(t *testing.T) {
		cfg := New()
		require.NoError(t, Load(cfg, NewLoadOpts().WithEnv("test")))
		require.Equal(t, ":memory:", cfg.GetString("application.database.dsn"))
		cfg = New()
		require.NoError(t, Load(cfg, NewLoadOpts().WithEnv("production")))
		require.Equal(t, "INFO", cfg.GetString("defaultLogLevel"))
		missing := NewLoadOpts().WithEnv("missing")
		require.ErrorIs(t, Load(New(), missing), os.ErrNotExist)
		missing = NewLoadOpts()
		missing.defaultConfigFileName = "missing.yaml"
		require.ErrorIs(t, Load(New(), missing), os.ErrNotExist)
		assertOpts := NewLoadOpts()
		assertOpts.WithEnv("")
		require.Equal(t, "local", assertOpts.env)
	})
}
