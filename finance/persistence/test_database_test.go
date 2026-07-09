package persistence

import (
	"fmt"
	"testing"

	"github.com/gemyago/signal-foundry/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
)

func openTestDatabase(t *testing.T) *Database {
	t.Helper()

	fake := faker.New()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "persistence-"+fake.UUID().V4())
	sqlDB, err := sqlconn.Open(dsn)
	if err != nil {
		t.Fatalf("open persistence test sql database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Fatalf("close persistence test sql database: %v", closeErr)
		}
	})
	database, err := NewDatabase(sqlDB, dsn)
	if err != nil {
		t.Fatalf("open persistence test database: %v", err)
	}
	migrateErr := NewMigrator(database).Migrate(t.Context())
	if migrateErr != nil {
		t.Fatalf("migrate persistence test database: %v", migrateErr)
	}

	return database
}
