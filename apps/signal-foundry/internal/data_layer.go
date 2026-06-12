package internal

import (
	"errors"
	"fmt"

	"github.com/gemyago/signal-foundry/runtime/data"
	"go.uber.org/dig"
)

type dataLayerStoreDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
}

func newDataLayerStore(deps dataLayerStoreDeps) (*data.DatabaseStore, error) {
	store, err := data.NewDatabaseStore(
		deps.DatabaseDSN,
		data.DatabaseStoreOpts{
			TablePrefix: deps.DatabaseTablePrefix,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create data-layer database store: %w", err)
	}

	return store, nil
}

func newDataIngestionService(store *data.DatabaseStore) (*data.IngestionService, error) {
	if store == nil {
		return nil, errors.New("data-layer database store is required")
	}

	return data.NewIngestionService(data.IngestionServiceDeps{
		InstrumentStore: store,
		CandleStore:     store,
		TradeStore:      store,
	})
}

func newDataReadService(store *data.DatabaseStore) (*data.ReadService, error) {
	if store == nil {
		return nil, errors.New("data-layer database store is required")
	}

	return data.NewReadService(data.ReadServiceDeps{
		InstrumentStore: store,
		CandleStore:     store,
		TradeStore:      store,
	})
}
