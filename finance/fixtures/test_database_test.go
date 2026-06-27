package fixtures_test

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
		fmt.Sprintf("file:%s?mode=memory&cache=shared", "fixtures-"+fake.UUID().V4()),
	)
	if err != nil {
		t.Fatalf("open fixtures test database: %v", err)
	}
	migrateErr := persistence.NewMigrator(database).Migrate(t.Context())
	if migrateErr != nil {
		t.Fatalf("migrate fixtures test database: %v", migrateErr)
	}

	return database
}
