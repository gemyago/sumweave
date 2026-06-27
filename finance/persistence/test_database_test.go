package persistence

import (
	"fmt"
	"testing"

	"github.com/jaswdr/faker/v2"
)

func openTestDatabase(t *testing.T) *Database {
	t.Helper()

	fake := faker.New()
	database, err := OpenDatabase(
		fmt.Sprintf("file:%s?mode=memory&cache=shared", "persistence-"+fake.UUID().V4()),
	)
	if err != nil {
		t.Fatalf("open persistence test database: %v", err)
	}
	migrateErr := NewMigrator(database).Migrate(t.Context())
	if migrateErr != nil {
		t.Fatalf("migrate persistence test database: %v", migrateErr)
	}

	return database
}
