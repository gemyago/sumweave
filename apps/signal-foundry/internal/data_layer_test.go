//go:build !release

package internal

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDataLayerConstructors(t *testing.T) {
	fake := faker.New()
	makeSQLiteMemoryDSN := func() string {
		return fmt.Sprintf("file:%s?mode=memory&cache=shared", "data-layer-"+fake.UUID().V4())
	}

	makeStore := func(t *testing.T) *data.DatabaseStore {
		t.Helper()
		dsn := makeSQLiteMemoryDSN()
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := newDataLayerStore(dataLayerStoreDeps{
			DatabaseDSN:         dsn,
			DatabaseTablePrefix: strings.ReplaceAll("data_"+fake.Lorem().Word(), "-", "_") + "_",
			SQLDB:               sqlDB,
		})
		require.NoError(t, err)

		return store
	}

	makeBlobStore := func(t *testing.T) *data.LocalRawPayloadBlobStore {
		t.Helper()
		blobStore, err := newDataRawPayloadBlobStore(dataLayerBlobStoreDeps{
			DataDir:                 t.TempDir(),
			RawPayloadBlobStorePath: filepath.Join("custom", fake.Lorem().Word()),
		})
		require.NoError(t, err)
		return blobStore
	}

	t.Run("newDataLayerStore", func(t *testing.T) {
		t.Run("opens sql db and registers it for shutdown", func(t *testing.T) {
			hooks := lifecycle.NewTestShutdownHooks()
			db, err := newDataLayerSQLDB(dataLayerSQLDBDeps{
				DatabaseDSN:   makeSQLiteMemoryDSN(),
				ShutdownHooks: hooks,
			})
			require.NoError(t, err)
			require.NoError(t, db.PingContext(t.Context()))
			require.NoError(t, hooks.PerformShutdown(t.Context()))
			require.ErrorContains(t, db.PingContext(t.Context()), "closed")
		})

		t.Run("wraps sql open errors", func(t *testing.T) {
			db, err := newDataLayerSQLDB(dataLayerSQLDBDeps{
				DatabaseDSN:   "   ",
				ShutdownHooks: lifecycle.NewTestShutdownHooks(),
			})
			require.Error(t, err)
			require.Nil(t, db)
			require.ErrorContains(t, err, "open data-layer sql database")
		})

		t.Run("creates store with configured dsn and prefix", func(t *testing.T) {
			store := makeStore(t)
			require.NotNil(t, store)
		})

		t.Run("returns wrapped error when dsn is missing", func(t *testing.T) {
			sqlDB, err := sqlconn.Open(makeSQLiteMemoryDSN())
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			store, err := newDataLayerStore(dataLayerStoreDeps{
				DatabaseDSN:         "",
				DatabaseTablePrefix: strings.ReplaceAll("data_"+fake.Lorem().Word(), "-", "_") + "_",
				SQLDB:               sqlDB,
			})
			require.Error(t, err)
			require.Nil(t, store)
			require.ErrorContains(t, err, "create data-layer database store")
		})
	})

	t.Run("newDataIngestionService", func(t *testing.T) {
		t.Run("creates service backed by the database store", func(t *testing.T) {
			service, err := newDataIngestionService(makeStore(t))
			require.NoError(t, err)
			require.NotNil(t, service)
		})

		t.Run("returns wrapped error when store is nil", func(t *testing.T) {
			service, err := newDataIngestionService(nil)
			require.Error(t, err)
			require.Nil(t, service)
			require.ErrorContains(t, err, "data-layer database store is required")
		})
	})

	t.Run("newDataReadService", func(t *testing.T) {
		t.Run("creates service backed by the database store", func(t *testing.T) {
			service, err := newDataReadService(makeStore(t))
			require.NoError(t, err)
			require.NotNil(t, service)
		})

		t.Run("returns wrapped error when store is nil", func(t *testing.T) {
			service, err := newDataReadService(nil)
			require.Error(t, err)
			require.Nil(t, service)
			require.ErrorContains(t, err, "data-layer database store is required")
		})
	})

	t.Run("newDataRawPayloadBlobStore", func(t *testing.T) {
		t.Run("defaults under data dir when path unset", func(t *testing.T) {
			dataDir := t.TempDir()
			blobStore, err := newDataRawPayloadBlobStore(dataLayerBlobStoreDeps{DataDir: dataDir})
			require.NoError(t, err)
			require.NotNil(t, blobStore)
		})

		t.Run("resolves relative configured paths from data dir", func(t *testing.T) {
			dataDir := t.TempDir()
			blobStore, err := newDataRawPayloadBlobStore(dataLayerBlobStoreDeps{
				DataDir:                 dataDir,
				RawPayloadBlobStorePath: filepath.Join("relative", fake.Lorem().Word()),
			})
			require.NoError(t, err)
			require.NotNil(t, blobStore)
		})
	})

	t.Run("newDataLineageService", func(t *testing.T) {
		t.Run("creates service backed by store and blob store", func(t *testing.T) {
			service, err := newDataLineageService(makeStore(t), makeBlobStore(t))
			require.NoError(t, err)
			require.NotNil(t, service)
		})

		t.Run("returns error when blob store is nil", func(t *testing.T) {
			service, err := newDataLineageService(makeStore(t), nil)
			require.Error(t, err)
			require.Nil(t, service)
			require.ErrorContains(t, err, "raw payload blob store is required")
		})
	})
}
