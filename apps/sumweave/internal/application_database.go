package internal

import (
	"database/sql"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/lifecycle"
	"go.uber.org/dig"
)

type applicationDatabaseDeps struct {
	dig.In

	DatabaseDSN   string `name:"config.application.database.dsn"`
	ShutdownHooks *lifecycle.ShutdownHooks
}

func newApplicationSQLDB(deps applicationDatabaseDeps) (*sql.DB, error) {
	db, err := sqlconn.Open(deps.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open application sql database: %w", err)
	}
	deps.ShutdownHooks.RegisterNoCtx("application-sql-db", db.Close)
	return db, nil
}
