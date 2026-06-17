package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"go.uber.org/dig"
)

type storeDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	AutoMigrate         bool   `name:"config.dataLayer.database.autoMigrate"`
}

type serviceDeps struct {
	dig.In

	Store        *Store
	IDGenerator  ident.Generator
	MaxIntervals int `name:"config.jobs.historicalBackfill.maxIntervals"`
	MaxPageSize  int `name:"config.jobs.historicalBackfill.maxPageSize"`
}

type workerDeps struct {
	dig.In

	Store         *Store
	Logger        *slog.Logger
	Lineage       *data.LineageService
	ReadService   *data.ReadService
	IngestionSvc  *data.IngestionService
	Enabled       bool          `name:"config.jobs.worker.enabled"`
	PollInterval  time.Duration `name:"config.jobs.worker.pollInterval"`
	MaxAttempts   int           `name:"config.jobs.worker.maxAttempts"`
	MaxConcurrent int           `name:"config.jobs.worker.maxConcurrentHistoricalBackfills"`
}

type startupDeps struct {
	dig.In

	Worker        *Worker
	ShutdownHooks *lifecycle.ShutdownHooks
	Enabled       bool `name:"config.jobs.worker.enabled"`
}

func newStoreFromDI(deps storeDeps) (*Store, error) {
	store, err := NewStore(deps.DatabaseDSN, StoreOpts{
		TablePrefix: deps.DatabaseTablePrefix + "jobs_",
	})
	if err != nil {
		return nil, err
	}
	if deps.AutoMigrate {
		if err = store.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("auto migrate jobs store: %w", err)
		}
	}
	return store, nil
}

func newWorkerFromDI(deps workerDeps) (*Worker, error) {
	ingestionFlow, err := venueedge.NewIngestionFlow(deps.IngestionSvc)
	if err != nil {
		return nil, fmt.Errorf("create venue ingestion flow: %w", err)
	}
	if deps.Lineage != nil {
		ingestionFlow.WithRawPayloadLineage(deps.Lineage)
	}
	runner, err := NewHistoricalBackfillRunner(deps.Lineage, deps.ReadService, ingestionFlow)
	if err != nil {
		return nil, err
	}
	return NewWorker(WorkerDeps{
		Store:  deps.Store,
		Runner: runner,
		Logger: deps.Logger,
		Config: WorkerConfig{
			Enabled:                         deps.Enabled,
			PollInterval:                    deps.PollInterval,
			MaxAttempts:                     deps.MaxAttempts,
			MaxConcurrentHistoricalBackfill: deps.MaxConcurrent,
		},
	})
}

func newServiceFromDI(deps serviceDeps, worker *Worker) (*Service, error) {
	return NewService(ServiceDeps{
		Store:       deps.Store,
		IDGenerator: deps.IDGenerator,
		Limits: HistoricalBackfillLimits{
			MaxIntervals: deps.MaxIntervals,
			MaxPageSize:  deps.MaxPageSize,
		},
		Wake: worker,
	})
}

func startWorker(ctx context.Context, deps startupDeps) error {
	if !deps.Enabled {
		return nil
	}
	if err := deps.Worker.Start(ctx); err != nil {
		return err
	}
	deps.ShutdownHooks.Register("jobs-worker", deps.Worker.Stop)
	return nil
}

func Register(ctx context.Context, container *dig.Container) error {
	if err := di.ProvideAll(
		container,
		newStoreFromDI,
		newWorkerFromDI,
		newServiceFromDI,
	); err != nil {
		return err
	}
	return container.Invoke(func(deps startupDeps) error {
		return startWorker(ctx, deps)
	})
}
