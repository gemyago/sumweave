package persistence

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Run("auto-migrates finance schema idempotently", func(t *testing.T) {
		database := openTestDatabase(t)
		migrator := NewMigrator(database)
		store := NewStore(database)

		require.NoError(t, migrator.Migrate(t.Context()))
		require.NoError(t, migrator.Migrate(t.Context()))

		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		rows, err := sqlDB.QueryContext(
			t.Context(),
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'finance_%' ORDER BY name",
		)
		require.NoError(t, err)
		defer rows.Close()

		var tableNames []string
		for rows.Next() {
			var tableName string
			require.NoError(t, rows.Scan(&tableName))
			tableNames = append(tableNames, tableName)
		}
		require.NoError(t, rows.Err())
		assert.NotContains(t, tableNames, "finance_schema_migrations")
		assert.Contains(t, tableNames, "finance_transactions")
		for _, tableName := range tableNames {
			for _, columnName := range tableColumns(t, store, tableName) {
				assert.False(t, strings.HasSuffix(columnName, "_unix_nano"), tableName+"."+columnName)
			}
		}
	})

	t.Run("keeps schema initialization portable across sqlite modes", func(t *testing.T) {
		fake := faker.New()
		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		database, err := NewDatabase(sqlDB, ":memory:")
		require.NoError(t, err)
		migrator := NewMigrator(database)
		require.NoError(t, migrator.Migrate(t.Context()))

		fileDSN := fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word())
		fileSQLDB, err := sqlconn.Open(fileDSN)
		require.NoError(t, err)
		defer func() { require.NoError(t, fileSQLDB.Close()) }()
		fileDatabase, err := NewDatabase(fileSQLDB, fileDSN)
		require.NoError(t, err)
		fileMigrator := NewMigrator(fileDatabase)
		require.NoError(t, fileMigrator.Migrate(t.Context()))
	})

	t.Run("surfaces auto-migrate failures", func(t *testing.T) {
		fake := faker.New()
		path := fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word())
		require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

		readOnlyDSN := fmt.Sprintf("file:%s?mode=ro", path)
		sqlDB, err := sqlconn.Open(readOnlyDSN)
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()
		database, err := NewDatabase(sqlDB, readOnlyDSN)
		require.NoError(t, err)
		migrator := NewMigrator(database)

		err = migrator.Migrate(t.Context())
		require.Error(t, err)
	})

	t.Run("preserves invite code uniqueness", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewStore(database)

		now := time.Date(2026, time.June, 21, 10, 0, 0, 0, time.UTC)
		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		inviteCode := "code-" + fake.UUID().V4()
		_, err = store.SaveTenantInvite(t.Context(), domain.TenantInvite{
			ID:              "invite-1-" + fake.UUID().V4(),
			TenantID:        tenant.ID,
			Code:            inviteCode,
			Recipient:       "recipient-1-" + fake.Internet().Email(),
			CreatedByUserID: "user-1-" + fake.UUID().V4(),
			CreatedAt:       now,
		})
		require.NoError(t, err)

		_, err = store.SaveTenantInvite(t.Context(), domain.TenantInvite{
			ID:              "invite-2-" + fake.UUID().V4(),
			TenantID:        tenant.ID,
			Code:            inviteCode,
			Recipient:       "recipient-2-" + fake.Internet().Email(),
			CreatedByUserID: "user-2-" + fake.UUID().V4(),
			CreatedAt:       now,
		})
		require.Error(t, err)
	})
}

func tableColumns(t *testing.T, store *Store, tableName string) []string {
	t.Helper()

	sqlDB, err := store.db.DB()
	require.NoError(t, err)

	rows, err := sqlDB.QueryContext(t.Context(), fmt.Sprintf("PRAGMA table_info('%s')", tableName))
	require.NoError(t, err)
	defer rows.Close()

	columns := make([]string, 0)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk))
		columns = append(columns, name)
	}
	require.NoError(t, rows.Err())
	return columns
}
