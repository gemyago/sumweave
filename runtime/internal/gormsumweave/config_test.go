package gormsumweave

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

func TestNewGormConfigForSumweaveTables(t *testing.T) {
	fake := faker.New()
	t.Run("empty TablePrefix means no prefix", func(t *testing.T) {
		cfg := NewGormConfigForSumweaveTables(GormSumweaveTablesOpts{})
		ns := requireNamingStrategy(t, cfg)
		require.Empty(t, ns.TablePrefix)
		require.False(t, cfg.TranslateError)
	})

	t.Run("custom TablePrefix is preserved", func(t *testing.T) {
		prefix := fake.Lorem().Word() + "_"
		cfg := NewGormConfigForSumweaveTables(GormSumweaveTablesOpts{TablePrefix: prefix})
		ns := requireNamingStrategy(t, cfg)
		require.Equal(t, prefix, ns.TablePrefix)
	})

	t.Run("TranslateError is set when requested", func(t *testing.T) {
		cfg := NewGormConfigForSumweaveTables(GormSumweaveTablesOpts{TranslateError: true})
		require.True(t, cfg.TranslateError)
		ns := requireNamingStrategy(t, cfg)
		require.Empty(t, ns.TablePrefix)
	})

	t.Run("NowFunc returns microsecond-precise values without changing location", func(t *testing.T) {
		location := time.FixedZone(fake.Lorem().Word(), 2*60*60)
		clockValue := time.Date(2026, time.September, 3, 19, 20, 30, 123456789, location)
		cfg := NewGormConfigForSumweaveTables(GormSumweaveTablesOpts{
			NowFunc: func() time.Time { return clockValue },
		})

		require.Equal(t, clockValue.Truncate(time.Microsecond), cfg.NowFunc())
		require.Same(t, location, cfg.NowFunc().Location())
	})

	t.Run("logger enables slog-backed GORM logging", func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		cfg := NewGormConfigForSumweaveTables(GormSumweaveTablesOpts{Logger: logger})

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
