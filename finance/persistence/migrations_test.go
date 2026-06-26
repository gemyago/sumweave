package persistence

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Run("auto-migrates finance schema idempotently", func(t *testing.T) {
		fake := faker.New()
		store, err := NewStore(fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()))
		require.NoError(t, err)

		require.NoError(t, store.Migrate(t.Context()))
		require.NoError(t, store.Migrate(t.Context()))

		schemaModels := financeSchemaModels()
		require.Len(t, schemaModels, 20)

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
		assert.Contains(t, tableNames, "finance_connection_secrets")
		assert.Contains(t, tableNames, "finance_transactions")
		assert.Contains(t, tableNames, "finance_csv_imports")
		assert.Contains(t, tableNames, "finance_pending_bank_link_starts")
	})

	t.Run("keeps schema initialization portable across sqlite modes", func(t *testing.T) {
		fake := faker.New()
		store, err := NewStore(":memory:")
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))

		fileStore, err := NewStore(fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()))
		require.NoError(t, err)
		require.NoError(t, fileStore.Migrate(t.Context()))
	})

	t.Run("surfaces auto-migrate failures", func(t *testing.T) {
		fake := faker.New()
		path := fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word())
		require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

		store, err := NewStore(fmt.Sprintf("file:%s?mode=ro", path))
		require.NoError(t, err)

		err = store.Migrate(t.Context())
		require.Error(t, err)
	})

	t.Run("preserves invite code uniqueness", func(t *testing.T) {
		fake := faker.New()
		store, err := NewStore(fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()))
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))

		now := time.Date(2026, time.June, 21, 10, 0, 0, 0, time.UTC)
		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err = store.SaveTenant(t.Context(), tenant)
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

	t.Run("preserves prior composite index shapes", func(t *testing.T) {
		fake := faker.New()
		store, err := NewStore(fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()))
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))

		assert.Equal(
			t,
			[]string{"code"},
			indexColumns(t, store, "finance_tenant_invites", "idx_finance_tenant_invites_code"),
		)
		assert.True(t, indexUnique(t, store, "finance_tenant_invites", "idx_finance_tenant_invites_code"))
		assert.Equal(
			t,
			[]string{"tenant_id", "actor_user_id", "provider", "state"},
			indexColumns(
				t,
				store,
				"finance_pending_bank_link_starts",
				"idx_finance_pending_bank_link_starts_lookup",
			),
		)
		assert.True(
			t,
			indexUnique(
				t,
				store,
				"finance_pending_bank_link_starts",
				"idx_finance_pending_bank_link_starts_lookup",
			),
		)
		assert.Equal(
			t,
			[]string{"connection_id", "captured_at"},
			indexColumns(t, store, "finance_balance_snapshots", "idx_finance_balance_snapshots_connection_id"),
		)
		assert.Equal(
			t,
			[]string{"connection_id", "captured_at"},
			indexColumns(t, store, "finance_raw_payloads", "idx_finance_raw_payloads_connection_id"),
		)
		assert.Equal(
			t,
			[]string{"connection_id", "provider_account_id", "provider_transaction_id"},
			indexColumns(
				t,
				store,
				"finance_provider_transaction_matches",
				"idx_finance_provider_transaction_matches_provider_id",
			),
		)
		assert.Equal(
			t,
			[]string{"connection_id", "provider_account_id", "fingerprint"},
			indexColumns(
				t,
				store,
				"finance_provider_transaction_matches",
				"idx_finance_provider_transaction_matches_fingerprint",
			),
		)
	})
}

func indexColumns(t *testing.T, store *Store, tableName string, indexName string) []string {
	t.Helper()

	sqlDB, err := store.db.DB()
	require.NoError(t, err)

	rows, err := sqlDB.QueryContext(t.Context(), fmt.Sprintf("PRAGMA index_info('%s')", indexName))
	require.NoError(t, err)
	defer rows.Close()

	columns := make([]string, 0)
	for rows.Next() {
		var seqno int
		var cid int
		var name string
		require.NoError(t, rows.Scan(&seqno, &cid, &name))
		columns = append(columns, name)
	}
	require.NoError(t, rows.Err())
	require.NotEmptyf(t, columns, "expected index %s on %s", indexName, tableName)

	return columns
}

func indexUnique(t *testing.T, store *Store, tableName string, indexName string) bool {
	t.Helper()

	sqlDB, err := store.db.DB()
	require.NoError(t, err)

	rows, err := sqlDB.QueryContext(t.Context(), fmt.Sprintf("PRAGMA index_list('%s')", tableName))
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		require.NoError(t, rows.Scan(&seq, &name, &unique, &origin, &partial))
		if name == indexName {
			return unique == 1
		}
	}
	require.NoError(t, rows.Err())
	require.Failf(t, "missing index", "index %s not found on table %s", indexName, tableName)
	return false
}
