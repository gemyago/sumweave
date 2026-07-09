package internal

import (
	"database/sql"
	"fmt"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/lifecycle"
	"go.uber.org/dig"
)

type dataLayerSQLDBDeps struct {
	dig.In

	DatabaseDSN   string `name:"config.dataLayer.database.dsn"`
	ShutdownHooks *lifecycle.ShutdownHooks
}

func newDataLayerSQLDB(deps dataLayerSQLDBDeps) (*sql.DB, error) {
	db, err := sqlconn.Open(deps.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open data-layer sql database: %w", err)
	}
	deps.ShutdownHooks.RegisterNoCtx("data-layer-sql-db", db.Close)
	return db, nil
}
