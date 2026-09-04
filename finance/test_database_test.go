package finance

import (
	"database/sql"
	"os"
	"testing"

	"github.com/gemyago/sumweave/finance/persistence"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDatabase(t *testing.T) *persistence.Database {
	t.Helper()

	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Fatal("SUMWEAVE_POSTGRES_TEST_DSN is required for database tests")
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open finance test sql database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Fatalf("close finance test sql database: %v", closeErr)
		}
	})
	database, err := persistence.NewDatabase(sqlDB, dsn)
	if err != nil {
		t.Fatalf("open finance test database: %v", err)
	}
	return database
}
