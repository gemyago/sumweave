package jobs

import (
	"database/sql"
	"fmt"
	"log/slog"

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
	registry := NewRegistry()
	worker, err := NewWorker(WorkerDeps{
		Store:          store,
		Registry:       registry,
		Logger:         deps.Logger,
		Config:         deps.WorkerConfig,
		DispatchDB:     deps.SQLDB,
		DispatchConfig: DispatchConfig{DatabaseDSN: deps.DatabaseDSN, TablePrefix: deps.DatabaseTablePrefix},
	})
	if err != nil { // coverage-ignore // Worker owns this error behavior.
		return nil, fmt.Errorf("create jobs worker: %w", err)
	}
	service, err := NewService(ServiceDeps{
		Store:       store,
		IDGenerator: deps.IDGenerator,
		Publisher:   publisher,
		Registry:    registry,
	})
	if err != nil { // coverage-ignore // Service owns this error behavior.
		return nil, fmt.Errorf("create jobs service: %w", err)
	}
	scheduler, err := NewScheduler(SchedulerDeps{Store: store, Service: service})
	if err != nil { // coverage-ignore // Scheduler owns this error behavior.
		return nil, fmt.Errorf("create jobs scheduler: %w", err)
	}
	return &Module{Store: store, Service: service, Worker: worker, Scheduler: scheduler, Registry: registry}, nil
}
