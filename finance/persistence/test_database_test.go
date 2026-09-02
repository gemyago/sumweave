//go:build postgres_test

package persistence

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDatabase(t *testing.T) *Database {
	t.Helper()

	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Fatal("SUMWEAVE_POSTGRES_TEST_DSN is required for postgres_test")
	}
	sqlDB, err := sql.Open("pgx", dsn)
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
	return database
}
