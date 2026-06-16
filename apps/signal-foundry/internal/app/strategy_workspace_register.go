package app

import (
	"context"
	"fmt"

	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"go.uber.org/dig"
)

type strategyWorkspaceStoreDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
	AutoMigrate         bool   `name:"config.dataLayer.database.autoMigrate"`
}

func newStrategyArtifactStore(deps strategyWorkspaceStoreDeps) (*rtstrategy.ArtifactDatabaseStore, error) {
	store, err := rtstrategy.NewArtifactDatabaseStore(deps.DatabaseDSN, rtstrategy.ArtifactDatabaseStoreOpts{
		TablePrefix: deps.DatabaseTablePrefix + "strategy_",
	})
	if err != nil {
		return nil, fmt.Errorf("create strategy artifact store: %w", err)
	}
	if deps.AutoMigrate {
		if err = store.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("auto migrate strategy artifact store: %w", err)
		}
	}

	return store, nil
}

func newStrategyVersionRegistryService(
	deps strategyWorkspaceStoreDeps,
	artifactStore *rtstrategy.ArtifactDatabaseStore,
) (*rtstrategy.VersionRegistryService, error) {
	service, err := rtstrategy.NewVersionRegistryService(deps.DatabaseDSN, rtstrategy.VersionRegistryServiceDeps{
		ArtifactStore: artifactStore,
		TablePrefix:   deps.DatabaseTablePrefix + "strategy_",
	})
	if err != nil {
		return nil, fmt.Errorf("create strategy version registry service: %w", err)
	}
	if deps.AutoMigrate {
		if err = service.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("auto migrate strategy version registry service: %w", err)
		}
	}
	if _, err = service.EnsureDemoStrategyVersions(context.Background()); err != nil {
		return nil, fmt.Errorf("seed demo strategy versions: %w", err)
	}

	return service, nil
}
