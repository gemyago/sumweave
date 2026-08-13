package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
)

// Module groups the durable jobs capabilities assembled for an explicit
// wireup root.
type Module struct {
	Store     *Store
	Service   *Service
	Worker    *Worker
	Scheduler *Scheduler
	Registry  *Registry

	publisher *appdispatch.Publisher
	closeOnce sync.Once
	closeErr  error
}

// ModuleDeps contains the component-native collaborators and settings for the
// durable jobs module.
type ModuleDeps struct {
	SQLDB               *sql.DB
	DatabaseDSN         string
	DatabaseTablePrefix string
	Logger              *slog.Logger
	WorkerConfig        WorkerConfig
	IDGenerator         ident.Generator
}

// NewModule eagerly constructs durable jobs without starting a worker or
// scheduler loop. Commands own those loops after finance registration finishes.
func NewModule(deps ModuleDeps) (*Module, error) {
	store, err := NewStore(deps.SQLDB, deps.DatabaseDSN, StoreOpts{TablePrefix: deps.DatabaseTablePrefix + "jobs_"})
	if err != nil { // coverage-ignore // NewStore owns this error behavior.
		return nil, fmt.Errorf("create jobs store: %w", err)
	}
	publisher, err := appdispatch.NewPublisher(
		appdispatch.Config{DatabaseDSN: deps.DatabaseDSN, TablePrefix: deps.DatabaseTablePrefix},
		deps.SQLDB,
		deps.Logger,
	)
	if err != nil { // coverage-ignore // Publisher owns this error behavior.
		return nil, fmt.Errorf("create jobs dispatch publisher: %w", err)
	}
	routerFactory, err := appdispatch.NewRouterFactory(
		appdispatch.Config{DatabaseDSN: deps.DatabaseDSN, TablePrefix: deps.DatabaseTablePrefix},
		deps.SQLDB,
		publisher,
		deps.Logger,
	)
	if err != nil { // coverage-ignore // RouterFactory owns this error behavior.
		return nil, errors.Join(fmt.Errorf("create jobs router factory: %w", err), publisher.Close())
	}
	registry := NewRegistry()
	worker, err := NewWorker(WorkerDeps{
		Store:         store,
		Registry:      registry,
		Logger:        deps.Logger,
		Config:        deps.WorkerConfig,
		RouterFactory: routerFactory,
	})
	if err != nil { // coverage-ignore // Worker owns this error behavior.
		return nil, errors.Join(fmt.Errorf("create jobs worker: %w", err), publisher.Close())
	}
	service, err := NewService(ServiceDeps{
		Store:       store,
		IDGenerator: deps.IDGenerator,
		Publisher:   publisher,
		Registry:    registry,
	})
	if err != nil { // coverage-ignore // Service owns this error behavior.
		return nil, errors.Join(
			fmt.Errorf("create jobs service: %w", err),
			worker.Stop(context.Background()),
			publisher.Close(),
		)
	}
	scheduler, err := NewScheduler(SchedulerDeps{Store: store, Service: service})
	if err != nil { // coverage-ignore // Scheduler owns this error behavior.
		return nil, errors.Join(
			fmt.Errorf("create jobs scheduler: %w", err),
			worker.Stop(context.Background()),
			publisher.Close(),
		)
	}
	return &Module{
		Store: store, Service: service, Worker: worker, Scheduler: scheduler, Registry: registry, publisher: publisher,
	}, nil
}

// Close stops messaging before its owning wireup root closes the shared database.
func (module *Module) Close(ctx context.Context) error {
	if module == nil {
		return nil
	}
	module.closeOnce.Do(func() {
		module.closeErr = errors.Join(module.Worker.Stop(ctx), module.publisher.Close())
	})
	return module.closeErr
}
