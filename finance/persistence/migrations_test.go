package persistence

import (
	"fmt"
	"testing"

	"github.com/gemyago/signal-foundry/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Run("runs on a clean test database", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-"+fake.UUID().V4())

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, NewMigrator(database).Migrate(t.Context()))
	})

	t.Run("returns an error when the underlying connection is unavailable", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-err-"+fake.UUID().V4())

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = NewMigrator(database).Migrate(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "auto-migrate finance schema")
	})
}
