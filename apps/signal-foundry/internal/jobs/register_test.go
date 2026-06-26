package jobs

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/startupmode"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestRegisterHelpers(t *testing.T) {
	makeDataServices := func(t *testing.T) (*data.IngestionService, *data.ReadService, *data.LineageService) {
		t.Helper()
		store, err := data.NewDatabaseStore(
			filepath.Join(t.TempDir(), "data.sqlite"),
			data.DatabaseStoreOpts{TablePrefix: "jobs_test_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		ingestionService, err := data.NewIngestionService(data.IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)
		readService, err := data.NewReadService(data.ReadServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)
		blobStore, err := data.NewLocalRawPayloadBlobStore(filepath.Join(t.TempDir(), "payloads"))
		require.NoError(t, err)
		lineageService, err := data.NewLineageService(data.LineageServiceDeps{Store: store, BlobStore: blobStore})
		require.NoError(t, err)
		return ingestionService, readService, lineageService
	}

	t.Run("helper constructors and startup wiring succeed", func(t *testing.T) {
		ingestionService, readService, lineageService := makeDataServices(t)
		dsn := filepath.Join(t.TempDir(), "jobs.sqlite")
		registry, err := newRegistryFromDI(registryDeps{
			Lineage:      lineageService,
			ReadService:  readService,
			IngestionSvc: ingestionService,
		})
		require.NoError(t, err)
		store, err := newStoreFromDI(storeDeps{
			DatabaseDSN:         dsn,
			DatabaseTablePrefix: "pref_",
		})
		require.NoError(t, err)
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dsn,
			TablePrefix: "pref_",
		}))
		publisher, err := newPublisherFromDI(storeDeps{DatabaseDSN: dsn, DatabaseTablePrefix: "pref_"})
		require.NoError(t, err)
		defer func() { require.NoError(t, publisher.Close()) }()
		worker, err := newWorkerFromDI(workerDeps{
			Store:        store,
			Registry:     registry,
			Logger:       slog.Default(),
			Lineage:      lineageService,
			ReadService:  readService,
			IngestionSvc: ingestionService,
			Enabled:      true,
			DatabaseDSN:  dsn,
			TablePrefix:  "pref_",
		})
		require.NoError(t, err)
		service, err := newServiceFromDI(
			serviceDeps{
				Store:        store,
				Publisher:    publisher,
				IDGenerator:  ident.NewMockGenerator(),
				MaxIntervals: 10,
				MaxPageSize:  100,
			},
			worker,
			registry,
		)
		require.NoError(t, err)
		require.NotNil(t, service)
		hooks := lifecycle.NewTestShutdownHooks()
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, startWorker(context.Background(), startupDeps{
			Worker:        worker,
			ShutdownHooks: hooks,
			Enabled:       false,
			AutoStart:     &startupmode.JobsWorkerAutoStart{Enabled: true},
		}))
		assert.False(t, hooks.HasHook("jobs-worker", worker.Stop))
		require.NoError(t, startWorker(context.Background(), startupDeps{
			Worker:        worker,
			ShutdownHooks: hooks,
			Enabled:       true,
			AutoStart:     &startupmode.JobsWorkerAutoStart{Enabled: true},
		}))
		assert.True(t, hooks.HasHook("jobs-worker", worker.Stop))
		require.NoError(t, worker.Stop(t.Context()))
	})

	t.Run("startup wiring can disable auto-start while keeping worker enabled", func(t *testing.T) {
		ingestionService, readService, lineageService := makeDataServices(t)
		dsn := filepath.Join(t.TempDir(), "jobs.sqlite")
		registry, err := newRegistryFromDI(registryDeps{
			Lineage:      lineageService,
			ReadService:  readService,
			IngestionSvc: ingestionService,
		})
		require.NoError(t, err)
		store, err := newStoreFromDI(storeDeps{
			DatabaseDSN:         dsn,
			DatabaseTablePrefix: "pref_",
		})
		require.NoError(t, err)
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{
			DatabaseDSN: dsn,
			TablePrefix: "pref_",
		}))
		worker, err := newWorkerFromDI(workerDeps{
			Store:        store,
			Registry:     registry,
			Logger:       slog.Default(),
			Lineage:      lineageService,
			ReadService:  readService,
			IngestionSvc: ingestionService,
			Enabled:      true,
			DatabaseDSN:  dsn,
			TablePrefix:  "pref_",
		})
		require.NoError(t, err)
		hooks := lifecycle.NewTestShutdownHooks()
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, startWorker(context.Background(), startupDeps{
			Worker:        worker,
			ShutdownHooks: hooks,
			Enabled:       true,
			AutoStart:     &startupmode.JobsWorkerAutoStart{Enabled: false},
		}))
		assert.False(t, hooks.HasHook("jobs-worker", worker.Stop))
	})

	t.Run("register wires the service into dig", func(t *testing.T) {
		container := dig.New()
		ingestionService, readService, lineageService := makeDataServices(t)
		require.NoError(t, container.Provide(
			func() string { return filepath.Join(t.TempDir(), "jobs.sqlite") },
			dig.Name("config.dataLayer.database.dsn"),
		))
		require.NoError(t, container.Provide(
			func() string { return "pref_" },
			dig.Name("config.dataLayer.database.tablePrefix"),
		))
		require.NoError(t, container.Provide(
			func() int { return 10 },
			dig.Name("config.jobs.historicalBackfill.maxIntervals"),
		))
		require.NoError(t, container.Provide(
			func() int { return 100 },
			dig.Name("config.jobs.historicalBackfill.maxPageSize"),
		))
		require.NoError(t, container.Provide(
			func() bool { return false },
			dig.Name("config.jobs.worker.enabled"),
		))
		require.NoError(t, container.Provide(
			func() *startupmode.JobsWorkerAutoStart {
				return &startupmode.JobsWorkerAutoStart{Enabled: false}
			},
			dig.Name("internal.jobs.worker.autoStart"),
		))
		require.NoError(t, container.Provide(
			func() time.Duration { return time.Second },
			dig.Name("config.jobs.worker.pollInterval"),
		))
		require.NoError(t, container.Provide(
			func() int { return 3 },
			dig.Name("config.jobs.worker.maxAttempts"),
		))
		require.NoError(t, container.Provide(
			func() int { return 1 },
			dig.Name("config.jobs.worker.maxConcurrentHistoricalBackfills"),
		))
		require.NoError(t, container.Provide(slog.Default))
		require.NoError(t, container.Provide(func() ident.Generator { return ident.NewMockGenerator() }))
		require.NoError(t, container.Provide(func() *data.IngestionService { return ingestionService }))
		require.NoError(t, container.Provide(func() *data.ReadService { return readService }))
		require.NoError(t, container.Provide(func() *data.LineageService { return lineageService }))
		require.NoError(t, container.Provide(lifecycle.NewTestShutdownHooks))
		require.NoError(t, Register(context.Background(), container))
		var resolved *Service
		require.NoError(t, container.Invoke(func(service *Service) { resolved = service }))
		require.NotNil(t, resolved)
		var worker *Worker
		var hooks *lifecycle.ShutdownHooks
		require.NoError(t, container.Invoke(func(resolvedWorker *Worker, shutdownHooks *lifecycle.ShutdownHooks) {
			worker = resolvedWorker
			hooks = shutdownHooks
		}))
		assert.False(t, hooks.HasHook("jobs-worker", worker.Stop))
	})

	t.Run("helper constructors report dependency failures", func(t *testing.T) {
		_, err := newStoreFromDI(storeDeps{})
		require.Error(t, err)
		store, err := newStoreFromDI(storeDeps{
			DatabaseDSN:         filepath.Join(t.TempDir(), "jobs.sqlite"),
			DatabaseTablePrefix: "pref_",
		})
		require.NoError(t, err)
		_, err = newWorkerFromDI(workerDeps{Store: store, Logger: slog.Default()})
		require.Error(t, err)
		container := dig.New()
		err = Register(context.Background(), container)
		require.Error(t, err)
	})
}
