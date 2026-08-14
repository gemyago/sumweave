package jobs

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	fake := faker.New()
	dsn := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
	db, err := sqlconn.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	dispatchConfig := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "module_"}
	require.NoError(t, appdispatch.AutoMigrate(t.Context(), dispatchConfig, db))

	module, err := NewModule(ModuleDeps{
		SQLDB: db, DatabaseDSN: dsn, DatabaseTablePrefix: "module_",
		Logger: slog.New(slog.DiscardHandler), IDGenerator: ident.NewMockGenerator(),
	})
	require.NoError(t, err)

	t.Run("constructs enqueue and worker capabilities without starting consumption", func(t *testing.T) {
		require.NotNil(t, module.Service)
		require.NotNil(t, module.Worker)
		var offsets int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM module_app_dispatch_offsets`,
		).Scan(&offsets))
		assert.Zero(t, offsets)
	})

	t.Run("closes messaging resources idempotently", func(t *testing.T) {
		require.NoError(t, module.Close(t.Context()))
		require.NoError(t, module.Close(t.Context()))
	})
}
