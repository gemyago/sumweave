//go:build postgres_test

package fixtures_test

import (
	"os"
	"testing"

	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/gemyago/sumweave/finance/persistence"
)

func openTestDatabase(t *testing.T) *persistence.Database {
	t.Helper()

	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Fatal("SUMWEAVE_POSTGRES_TEST_DSN is required for postgres_test")
	}
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
	return database
}
