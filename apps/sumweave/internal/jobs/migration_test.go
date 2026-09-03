//go:build postgres_test

package jobs

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

func TestStoreAutoMigrate(t *testing.T) {
	t.Run("runs against the bootstrap-prepared jobs table", func(t *testing.T) {
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })

		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
	})
}
