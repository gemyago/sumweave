package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("should load local config with default opts", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts())
		require.NoError(t, err)

		require.Equal(t, "DEBUG", cfg.GetString("defaultLogLevel"))
		require.Equal(t, "https://api.monobank.ua", cfg.GetString("finance.providers.monobank.baseURL"))
	})
	t.Run("dev flow regression: test config still loads core HTTP defaults", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts().WithEnv("test"))
		require.NoError(t, err)
		require.Equal(t, 4501, cfg.GetInt("httpServer.port"))
	})
	t.Run("should fail if no default config is found", func(t *testing.T) {
		opts := NewLoadOpts()
		opts.defaultConfigFileName = "not-existing.yaml"
		cfg := New()
		err := Load(cfg, opts)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("should load env specific config", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts().WithEnv("production"))
		require.NoError(t, err)

		require.Equal(t, "INFO", cfg.GetString("defaultLogLevel"))
		require.Empty(t, cfg.GetString("dataLayer.database.dsn"))
	})
	t.Run("should return error if config is not found", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts().WithEnv("not-existing"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("should load data layer defaults", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts().WithEnv("test"))
		require.NoError(t, err)

		require.Equal(t, ":memory:", cfg.GetString("dataLayer.database.dsn"))
		require.Equal(t, "signal_foundry_data_", cfg.GetString("dataLayer.database.tablePrefix"))
	})
	t.Run("loads explicit app-root-relative local paths without rewriting", func(t *testing.T) {
		cfg := New()
		require.NoError(t, Load(cfg, NewLoadOpts().WithEnv("local")))

		require.Equal(t, "data", cfg.GetString("dataDir"))
		require.Equal(t, "data/data-layer.db", cfg.GetString("dataLayer.database.dsn"))
		require.Equal(t, "data/data-layer.db", cfg.GetString("finance.fixtures.database.dsn"))
		require.Equal(t, "../../.platform-agents", cfg.GetString("workspacefs.platformAgentsPath"))
		require.Equal(t, []string{"../../.platform-agents/skills"}, cfg.GetStringSlice("skills.paths"))
	})
}
