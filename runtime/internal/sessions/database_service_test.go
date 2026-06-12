package sessions

import (
	"strconv"
	"testing"

	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session/database"
)

func TestDatabaseSessionServiceViaADK(t *testing.T) {
	fake := faker.New()
	t.Run("creates new database session service", func(t *testing.T) {
		cfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(gormsignalfoundry.GormSignalFoundryTablesOpts{})
		svc, err := database.NewSessionService(gormsignalfoundry.NewGormDialector(":memory:"), cfg)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})

	t.Run("should fail if can't connect to database", func(t *testing.T) {
		badDSN := "postgres://localhost:" +
			strconv.Itoa(fake.RandomNumber(10000)) + "/" + fake.Lorem().Word()
		badCfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(gormsignalfoundry.GormSignalFoundryTablesOpts{})
		svc, err := database.NewSessionService(gormsignalfoundry.NewGormDialector(badDSN), badCfg)
		require.Error(t, err)
		require.Nil(t, svc)
	})
}
