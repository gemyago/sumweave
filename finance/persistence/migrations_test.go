package persistence

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type legacyBankConnectionModel struct {
	ID                   string    `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID             string    `gorm:"column:tenant_id;size:255;not null;index;index:idx_finance_bank_connections_created_order,priority:1;index:idx_finance_bank_connections_link_identity,unique,priority:1,where:provider_link_identity <> ''"`
	Provider             string    `gorm:"column:provider;size:255;not null;index:idx_finance_bank_connections_link_identity,unique,priority:2,where:provider_link_identity <> ''"`
	ConnectorID          string    `gorm:"column:connector_id;size:255;not null;default:'';index:idx_finance_bank_connections_link_identity,unique,priority:3,where:provider_link_identity <> ''"`
	DisplayName          string    `gorm:"column:display_name;size:255;not null"`
	ProviderReference    string    `gorm:"column:provider_reference;size:255;not null"`
	ProviderLinkIdentity string    `gorm:"column:provider_link_identity;size:255;not null;default:'';index:idx_finance_bank_connections_link_identity,unique,priority:4,where:provider_link_identity <> ''"`
	ExternalID           string    `gorm:"column:external_id;size:255;not null;default:''"`
	SecretID             string    `gorm:"column:secret_id;size:255;not null"`
	State                string    `gorm:"column:state;size:64;not null"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;index:idx_finance_bank_connections_created_order,priority:2"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null"`
}

func (legacyBankConnectionModel) TableName() string { return "finance_bank_connections" }

func TestMigrate(t *testing.T) {
	t.Run("runs on a clean test database", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-"+fake.UUID().V4())

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, NewMigrator(database).Migrate(t.Context()))
		require.True(t, database.db.Migrator().HasTable(&providerSnapshotModel{}))
		require.False(t, database.db.Migrator().HasTable("finance_provider_evidence"))
		require.False(t, database.db.Migrator().HasTable("finance_raw_payloads"))
	})

	t.Run("removes retired bank connection identity schema", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-bank-connections-"+fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, database.db.AutoMigrate(&legacyBankConnectionModel{}))

		require.NoError(t, NewMigrator(database).Migrate(t.Context()))
		migrator := database.db.Migrator()
		require.False(t, migrator.HasColumn(&bankConnectionModel{}, "external_id"))
		require.False(t, migrator.HasColumn(&bankConnectionModel{}, "provider_link_identity"))
		require.False(t, migrator.HasIndex(&bankConnectionModel{}, "idx_finance_bank_connections_link_identity"))
		require.True(t, migrator.HasIndex(&bankConnectionModel{}, "idx_finance_bank_connections_provider_reference"))
		require.NoError(t, NewMigrator(database).Migrate(t.Context()))
	})

	t.Run("reports retired bank connection identity cleanup errors", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-read-only-"+fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, database.db.AutoMigrate(&legacyBankConnectionModel{}))
		require.NoError(t, database.db.Exec("PRAGMA query_only = ON").Error)

		err = NewMigrator(database).Migrate(t.Context())
		require.ErrorContains(t, err, "remove retired bank connection identity schema")
	})

	t.Run("returns an error when the underlying connection is unavailable", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-err-"+fake.UUID().V4())

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)

		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = NewMigrator(database).Migrate(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "auto-migrate finance schema")
	})
}
