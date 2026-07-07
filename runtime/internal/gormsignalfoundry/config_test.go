package gormsignalfoundry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNewGormConfigForSignalFoundryTables(t *testing.T) {
	fake := faker.New()
	t.Run("empty TablePrefix means no prefix", func(t *testing.T) {
		cfg := NewGormConfigForSignalFoundryTables(GormSignalFoundryTablesOpts{})
		ns := requireNamingStrategy(t, cfg)
		require.Empty(t, ns.TablePrefix)
		require.False(t, cfg.TranslateError)
	})

	t.Run("custom TablePrefix is preserved", func(t *testing.T) {
		prefix := fake.Lorem().Word() + "_"
		cfg := NewGormConfigForSignalFoundryTables(GormSignalFoundryTablesOpts{TablePrefix: prefix})
		ns := requireNamingStrategy(t, cfg)
		require.Equal(t, prefix, ns.TablePrefix)
	})

	t.Run("TranslateError is set when requested", func(t *testing.T) {
		cfg := NewGormConfigForSignalFoundryTables(GormSignalFoundryTablesOpts{TranslateError: true})
		require.True(t, cfg.TranslateError)
		ns := requireNamingStrategy(t, cfg)
		require.Empty(t, ns.TablePrefix)
	})

	t.Run("logger enables slog-backed GORM logging", func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		cfg := NewGormConfigForSignalFoundryTables(GormSignalFoundryTablesOpts{Logger: logger})

		require.NotNil(t, cfg.Logger)

		cfg.Logger.Trace(
			context.Background(),
			time.Now().Add(-2*time.Second),
			func() (string, int64) { return "SELECT ? FROM test WHERE id = ?", 1 },
			nil,
		)

		require.Contains(t, logs.String(), `"gorm"`)
		require.Contains(t, logs.String(), `"sql":"SELECT ? FROM test WHERE id = ?"`)
	})
}

func requireNamingStrategy(t *testing.T, cfg *gorm.Config) schema.NamingStrategy {
	t.Helper()
	ns, ok := cfg.NamingStrategy.(schema.NamingStrategy)
	require.True(t, ok, "expected schema.NamingStrategy")
	return ns
}
