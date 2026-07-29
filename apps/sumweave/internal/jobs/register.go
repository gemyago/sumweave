package jobs

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/di"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/startupmode"
	"go.uber.org/dig"
)

type storeDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.application.database.dsn"`
	DatabaseTablePrefix string `name:"config.application.database.tablePrefix"`
	SQLDB               *sql.DB
}
type serviceDeps struct {
	dig.In

	Store       *Store
	Publisher   *appdispatch.Publisher
	IDGenerator ident.Generator
	Registry    *Registry
}
type workerDeps struct {
	dig.In

	Store        *Store
	Registry     *Registry
	Logger       *slog.Logger
	Enabled      bool          `name:"config.jobs.worker.enabled"`
	PollInterval time.Duration `name:"config.jobs.worker.pollInterval"`
	MaxAttempts  int           `name:"config.jobs.worker.maxAttempts"`
	DatabaseDSN  string        `name:"config.application.database.dsn"`
	TablePrefix  string        `name:"config.application.database.tablePrefix"`
	SQLDB        *sql.DB
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
	return NewStore(deps.SQLDB, deps.DatabaseDSN, StoreOpts{TablePrefix: deps.DatabaseTablePrefix + "jobs_"})
}
func newPublisherFromDI(deps storeDeps, logger *slog.Logger) (*appdispatch.Publisher, error) {
	return appdispatch.NewPublisher(
		appdispatch.Config{DatabaseDSN: deps.DatabaseDSN, TablePrefix: deps.DatabaseTablePrefix},
		deps.SQLDB,
		logger,
	)
}
func newRegistryFromDI() *Registry { return NewRegistry() }
func newWorkerFromDI(deps workerDeps) (*Worker, error) {
	return NewWorker(
		WorkerDeps{
			Store:    deps.Store,
			Registry: deps.Registry,
			Logger:   deps.Logger,
			Config: WorkerConfig{
				Enabled:      deps.Enabled,
				PollInterval: deps.PollInterval,
				MaxAttempts:  deps.MaxAttempts,
			},
			DispatchDB:     deps.SQLDB,
			DispatchConfig: DispatchConfig{DatabaseDSN: deps.DatabaseDSN, TablePrefix: deps.TablePrefix},
		},
	)
}
func newServiceFromDI(deps serviceDeps, _ *Worker) (*Service, error) {
	return NewService(
		ServiceDeps{
			Store:       deps.Store,
			IDGenerator: deps.IDGenerator,
			Publisher:   deps.Publisher,
			Registry:    deps.Registry,
		},
	)
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
	return container.Invoke(func(deps startupDeps) error { return startWorker(ctx, deps) })
}
