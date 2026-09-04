package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("loads environment defaults and reports missing resources", func(t *testing.T) {
		cfg := New()
		require.NoError(t, load(cfg, NewLoadOpts().WithEnv("test")))
		require.Equal(
			t,
			"postgres://sumweave_runtime:sumweave_runtime_local@127.0.0.1:55432/sumweave_test?sslmode=disable",
			cfg.GetString("application.database.dsn"),
		)
		cfg = New()
		require.NoError(t, load(cfg, NewLoadOpts().WithEnv("production")))
		require.Equal(t, "INFO", cfg.GetString("defaultLogLevel"))
		missing := NewLoadOpts().WithEnv("missing")
		require.Error(t, load(New(), missing))
		missing = NewLoadOpts()
		missing.defaultConfigFileName = "missing.yaml"
		require.Error(t, load(New(), missing))
		assertOpts := NewLoadOpts()
		assertOpts.WithEnv("")
		require.Equal(t, "local", assertOpts.env)
	})
}
