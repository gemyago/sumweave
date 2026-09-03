package sessions

import (
	"strconv"
	"testing"

	"github.com/gemyago/sumweave/runtime/internal/gormsumweave"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session/database"
)

func TestDatabaseSessionServiceViaADK(t *testing.T) {
	fake := faker.New()
	t.Run("creates new database session service", func(t *testing.T) {
		cfg := gormsumweave.NewGormConfigForSumweaveTables(postgresTestTablesOpts())
		svc, err := database.NewSessionService(gormsumweave.NewGormDialector(postgresTestDSN(t)), cfg)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})

	t.Run("should fail if can't connect to database", func(t *testing.T) {
		badDSN := "postgres://localhost:" +
			strconv.Itoa(fake.RandomNumber(10000)) + "/" + fake.Lorem().Word()
		badCfg := gormsumweave.NewGormConfigForSumweaveTables(postgresTestTablesOpts())
		svc, err := database.NewSessionService(gormsumweave.NewGormDialector(badDSN), badCfg)
		require.Error(t, err)
		require.Nil(t, svc)
	})
}
