package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/startupmode"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/venueedge"
	"go.uber.org/dig"
)

type storeDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
}

type serviceDeps struct {
	dig.In

	Store        *Store
	Publisher    *appdispatch.Publisher
	IDGenerator  ident.Generator
	MaxIntervals int `name:"config.jobs.historicalBackfill.maxIntervals"`
	MaxPageSize  int `name:"config.jobs.historicalBackfill.maxPageSize"`
}

type workerDeps struct {
	dig.In

	Store         *Store
	Registry      *Registry
	Logger        *slog.Logger
	Lineage       *data.LineageService
	ReadService   *data.ReadService
	IngestionSvc  *data.IngestionService
	Enabled       bool          `name:"config.jobs.worker.enabled"`
	PollInterval  time.Duration `name:"config.jobs.worker.pollInterval"`
	MaxAttempts   int           `name:"config.jobs.worker.maxAttempts"`
	MaxConcurrent int           `name:"config.jobs.worker.maxConcurrentHistoricalBackfills"`
	DatabaseDSN   string        `name:"config.dataLayer.database.dsn"`
	TablePrefix   string        `name:"config.dataLayer.database.tablePrefix"`
}

type registryDeps struct {
	dig.In

	Lineage      *data.LineageService
	ReadService  *data.ReadService
	IngestionSvc *data.IngestionService
}

type startupDeps struct {
	dig.In

	Worker        *Worker
	Scheduler     *Scheduler
	ShutdownHooks *lifecycle.ShutdownHooks
	Enabled       bool                             `name:"config.jobs.worker.enabled"`
	AutoStart     *startupmode.JobsWorkerAutoStart `name:"internal.jobs.worker.autoStart" optional:"true"`
}

func newStoreFromDI(deps storeDeps) (*Store, error) {
	store, err := NewStore(deps.DatabaseDSN, StoreOpts{
		TablePrefix: deps.DatabaseTablePrefix + "jobs_",
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func newWorkerFromDI(deps workerDeps) (*Worker, error) {
	return NewWorker(WorkerDeps{
		Store:    deps.Store,
		Registry: deps.Registry,
		Logger:   deps.Logger,
		Config: WorkerConfig{
			Enabled:                         deps.Enabled,
			PollInterval:                    deps.PollInterval,
			MaxAttempts:                     deps.MaxAttempts,
			MaxConcurrentHistoricalBackfill: deps.MaxConcurrent,
		},
		DispatchConfig: DispatchConfig{
			DatabaseDSN: deps.DatabaseDSN,
			JobsDSN:     deps.DatabaseDSN,
			TablePrefix: deps.TablePrefix,
		},
	})
}

func newPublisherFromDI(deps storeDeps) (*appdispatch.Publisher, error) {
	return appdispatch.NewPublisher(appdispatch.Config{
		DatabaseDSN: deps.DatabaseDSN,
		TablePrefix: deps.DatabaseTablePrefix,
	})
}

func newRegistryFromDI(deps registryDeps) (*Registry, error) {
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
	registry := NewRegistry()
	if registerErr := RegisterHistoricalBackfillHandler(registry, runner); registerErr != nil {
		return nil, registerErr
	}
	return registry, nil
}

func newServiceFromDI(deps serviceDeps, _ *Worker, registry *Registry) (*Service, error) {
	return NewService(ServiceDeps{
		Store:       deps.Store,
		IDGenerator: deps.IDGenerator,
		Publisher:   deps.Publisher,
		Limits: HistoricalBackfillLimits{
			MaxIntervals: deps.MaxIntervals,
			MaxPageSize:  deps.MaxPageSize,
		},
		Registry: registry,
	})
}

func newSchedulerFromDI(store *Store, service *Service) (*Scheduler, error) {
	return NewScheduler(SchedulerDeps{Store: store, Service: service})
}

func startWorker(ctx context.Context, deps startupDeps) error {
	autoStart := true
	if deps.AutoStart != nil {
		autoStart = deps.AutoStart.Enabled
	}
	if !deps.Enabled || !autoStart {
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
		newPublisherFromDI,
		newRegistryFromDI,
		newWorkerFromDI,
		newServiceFromDI,
		newSchedulerFromDI,
	); err != nil {
		return err
	}
	return container.Invoke(func(deps startupDeps) error {
		return startWorker(ctx, deps)
	})
}
