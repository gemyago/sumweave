package jobs

import (
	"fmt"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreAutoMigrate(t *testing.T) {
	fake := faker.New()

	t.Run("removes alpha-only job storage with dependent indexes and generic schedules", func(t *testing.T) {
		dsn := fmt.Sprintf("file:jobs-migration-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })

		store, err := NewStore(db, dsn, StoreOpts{TablePrefix: "migration_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, store.db.Exec("ALTER TABLE migration_jobs ADD COLUMN idempotency_key TEXT").Error)
		require.NoError(t, store.db.Exec("ALTER TABLE migration_jobs ADD COLUMN input_json TEXT").Error)
		require.NoError(
			t,
			store.db.Exec("CREATE UNIQUE INDEX idx_jobs_idempotency ON migration_jobs (idempotency_key)").Error,
		)
		require.NoError(t, store.db.Exec("CREATE TABLE migration_job_schedules (id TEXT PRIMARY KEY)").Error)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, store.AutoMigrate())

		assert.False(t, store.db.Migrator().HasColumn(store.tableName, "idempotency_key"))
		assert.False(t, store.db.Migrator().HasColumn(store.tableName, "input_json"))
		assert.False(t, store.db.Migrator().HasIndex(store.tableName, "idx_jobs_idempotency"))
		assert.False(t, store.db.Migrator().HasTable("migration_job_schedules"))
	})
}
