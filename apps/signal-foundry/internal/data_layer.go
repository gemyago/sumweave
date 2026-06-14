package internal

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gemyago/signal-foundry/runtime/data"
	"go.uber.org/dig"
)

type dataLayerStoreDeps struct {
	dig.In

	DatabaseDSN         string `name:"config.dataLayer.database.dsn"`
	DatabaseTablePrefix string `name:"config.dataLayer.database.tablePrefix"`
}

type dataLayerBlobStoreDeps struct {
	dig.In

	DataDir                 string `name:"config.dataDir"`
	RawPayloadBlobStorePath string `name:"config.dataLayer.rawPayloadBlobStore.path"`
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

func newDataRawPayloadBlobStore(deps dataLayerBlobStoreDeps) (*data.LocalRawPayloadBlobStore, error) {
	basePath := strings.TrimSpace(deps.RawPayloadBlobStorePath)
	if basePath == "" {
		basePath = filepath.Join(deps.DataDir, "raw-payloads")
	} else if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(deps.DataDir, basePath)
	}

	blobStore, err := data.NewLocalRawPayloadBlobStore(basePath)
	if err != nil {
		return nil, fmt.Errorf("create raw payload blob store: %w", err)
	}

	return blobStore, nil
}

func newDataLineageService(
	store *data.DatabaseStore,
	blobStore *data.LocalRawPayloadBlobStore,
) (*data.LineageService, error) {
	if store == nil {
		return nil, errors.New("data-layer database store is required")
	}
	if blobStore == nil {
		return nil, errors.New("raw payload blob store is required")
	}

	return data.NewLineageService(data.LineageServiceDeps{
		Store:     store,
		BlobStore: blobStore,
	})
}
