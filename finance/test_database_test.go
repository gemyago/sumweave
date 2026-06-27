package finance

import (
	"fmt"
	"testing"

	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
)

func openTestDatabase(t *testing.T) *persistence.Database {
	t.Helper()

	fake := faker.New()
	database, err := persistence.OpenDatabase(
		fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-"+fake.UUID().V4()),
	)
	if err != nil {
		t.Fatalf("open finance test database: %v", err)
	}
	migrateErr := persistence.NewMigrator(database).Migrate(t.Context())
	if migrateErr != nil {
		t.Fatalf("migrate finance test database: %v", migrateErr)
	}

	return database
}
