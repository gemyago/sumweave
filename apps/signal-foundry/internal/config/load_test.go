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
	t.Run("dev flow regression: default config has no ui location set", func(t *testing.T) {
		cfg := New()
		err := Load(cfg, NewLoadOpts().WithEnv("test"))
		require.NoError(t, err)

		// httpServer.uiLocation must default to empty so no UI build artifacts are required.
		require.Empty(t, cfg.GetString("httpServer.uiLocation"))
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
		require.Empty(t, cfg.GetString("dataLayer.rawPayloadBlobStore.path"))
	})
}
