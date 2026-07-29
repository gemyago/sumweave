package fixtures_test

import (
	"fmt"
	"testing"

	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
)

func openTestDatabase(t *testing.T) *persistence.Database {
	t.Helper()

	fake := faker.New()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "fixtures-"+fake.UUID().V4())
	sqlDB, err := sqlconn.Open(dsn)
	if err != nil {
		t.Fatalf("open fixtures test sql database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Fatalf("close fixtures test sql database: %v", closeErr)
		}
	})
	database, err := persistence.NewDatabase(sqlDB, dsn)
	if err != nil {
		t.Fatalf("open fixtures test database: %v", err)
	}
	migrateErr := persistence.NewMigrator(database).Migrate(t.Context())
	if migrateErr != nil {
		t.Fatalf("migrate fixtures test database: %v", migrateErr)
	}

	return database
}
