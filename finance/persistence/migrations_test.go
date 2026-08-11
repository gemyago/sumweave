package persistence

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyRawPayloadModel struct {
	ID               string    `gorm:"column:id;size:255;not null;primaryKey"`
	ConnectionID     string    `gorm:"column:connection_id;size:255;not null"`
	Scope            string    `gorm:"column:scope;size:64;not null"`
	ProviderObjectID string    `gorm:"column:provider_object_id;size:255;not null;default:''"`
	PayloadJSON      string    `gorm:"column:payload_json;type:text;not null"`
	CapturedAt       time.Time `gorm:"column:captured_at;not null"`
}

func (legacyRawPayloadModel) TableName() string { return "finance_raw_payloads" }

type legacyProviderEvidenceModel struct {
	ID                   string    `gorm:"column:id;size:255;not null;primaryKey"`
	TenantID             string    `gorm:"column:tenant_id;size:255;not null"`
	ConnectionID         string    `gorm:"column:connection_id;size:255;not null"`
	FinanceAccountID     string    `gorm:"column:finance_account_id;size:255;not null;default:''"`
	FinanceTransactionID string    `gorm:"column:finance_transaction_id;size:255;not null;default:''"`
	Subject              string    `gorm:"column:subject;size:64;not null;default:''"`
	Scope                string    `gorm:"column:scope;size:64;not null"`
	ProviderObjectID     string    `gorm:"column:provider_object_id;size:255;not null;default:''"`
	PayloadJSON          string    `gorm:"column:payload_json;type:text;not null"`
	CapturedAt           time.Time `gorm:"column:captured_at;not null"`
}

func (legacyProviderEvidenceModel) TableName() string { return "finance_provider_evidence" }

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

	t.Run("keeps the latest duplicate current observations before adding identity indexes", func(t *testing.T) {
		fake := faker.New()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-legacy-"+fake.UUID().V4())
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		database, err := NewDatabase(sqlDB, dsn)
		require.NoError(t, err)
		require.NoError(t, database.db.AutoMigrate(&legacyRawPayloadModel{}, &legacyProviderEvidenceModel{}))

		earliest := time.Date(2026, time.July, 17, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		latest := earliest.Add(time.Minute)
		rawIdentity := legacyRawPayloadModel{
			ConnectionID:     "connection-" + fake.UUID().V4(),
			Scope:            "transaction",
			ProviderObjectID: "provider-object-" + fake.UUID().V4(),
		}
		for _, rawPayload := range []legacyRawPayloadModel{
			{
				ID:               "raw-earliest-" + fake.UUID().V4(),
				ConnectionID:     rawIdentity.ConnectionID,
				Scope:            rawIdentity.Scope,
				ProviderObjectID: rawIdentity.ProviderObjectID,
				PayloadJSON:      `{"value":"earliest"}`,
				CapturedAt:       earliest,
			},
			{
				ID:               "raw-latest-" + fake.UUID().V4(),
				ConnectionID:     rawIdentity.ConnectionID,
				Scope:            rawIdentity.Scope,
				ProviderObjectID: rawIdentity.ProviderObjectID,
				PayloadJSON:      `{"value":"latest"}`,
				CapturedAt:       latest,
			},
		} {
			require.NoError(t, database.db.Create(&rawPayload).Error)
		}

		evidenceIdentity := legacyProviderEvidenceModel{
			TenantID:             "tenant-" + fake.UUID().V4(),
			ConnectionID:         rawIdentity.ConnectionID,
			FinanceAccountID:     "account-" + fake.UUID().V4(),
			FinanceTransactionID: "transaction-" + fake.UUID().V4(),
			Subject:              "transaction",
			Scope:                "transaction",
			ProviderObjectID:     rawIdentity.ProviderObjectID,
		}
		for _, evidence := range []legacyProviderEvidenceModel{
			{
				ID:                   "evidence-earliest-" + fake.UUID().V4(),
				TenantID:             evidenceIdentity.TenantID,
				ConnectionID:         evidenceIdentity.ConnectionID,
				FinanceAccountID:     evidenceIdentity.FinanceAccountID,
				FinanceTransactionID: evidenceIdentity.FinanceTransactionID,
				Subject:              evidenceIdentity.Subject,
				Scope:                evidenceIdentity.Scope,
				ProviderObjectID:     evidenceIdentity.ProviderObjectID,
				PayloadJSON:          `{"value":"earliest"}`,
				CapturedAt:           earliest,
			},
			{
				ID:                   "evidence-latest-" + fake.UUID().V4(),
				TenantID:             evidenceIdentity.TenantID,
				ConnectionID:         evidenceIdentity.ConnectionID,
				FinanceAccountID:     evidenceIdentity.FinanceAccountID,
				FinanceTransactionID: evidenceIdentity.FinanceTransactionID,
				Subject:              evidenceIdentity.Subject,
				Scope:                evidenceIdentity.Scope,
				ProviderObjectID:     evidenceIdentity.ProviderObjectID,
				PayloadJSON:          `{"value":"latest"}`,
				CapturedAt:           latest,
			},
		} {
			require.NoError(t, database.db.Create(&evidence).Error)
		}

		require.NoError(t, NewMigrator(database).Migrate(t.Context()))
		var rawPayloads []rawPayloadModel
		require.NoError(t, database.db.Find(&rawPayloads).Error)
		require.Len(t, rawPayloads, 1)
		require.JSONEq(t, `{"value":"latest"}`, rawPayloads[0].PayloadJSON)
		var providerEvidence []providerEvidenceModel
		require.NoError(t, database.db.Find(&providerEvidence).Error)
		require.Len(t, providerEvidence, 1)
		require.JSONEq(t, `{"value":"latest"}`, providerEvidence[0].PayloadJSON)
	})

	t.Run("reports duplicate observation cleanup errors", func(t *testing.T) {
		fake := faker.New()
		makeDatabase := func(t *testing.T, model any) *Database {
			t.Helper()
			dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", "finance-migrate-cleanup-"+fake.UUID().V4())
			sqlDB, err := sqlconn.Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			database, err := NewDatabase(sqlDB, dsn)
			require.NoError(t, err)
			require.NoError(t, database.db.AutoMigrate(model))
			return database
		}
		addRawDuplicates := func(t *testing.T, database *Database) {
			t.Helper()
			connectionID := "connection-" + fake.UUID().V4()
			providerObjectID := "provider-object-" + fake.UUID().V4()
			capturedAt := time.Date(2026, time.July, 17, 11, 0, 0, 0, time.FixedZone("test", 2*60*60))
			makeRawPayload := func(id string, capturedAt time.Time) legacyRawPayloadModel {
				return legacyRawPayloadModel{
					ID:               id,
					ConnectionID:     connectionID,
					Scope:            "transaction",
					ProviderObjectID: providerObjectID,
					PayloadJSON:      `{}`,
					CapturedAt:       capturedAt,
				}
			}
			for _, payload := range []legacyRawPayloadModel{
				makeRawPayload("raw-first-"+fake.UUID().V4(), capturedAt),
				makeRawPayload("raw-second-"+fake.UUID().V4(), capturedAt.Add(time.Minute)),
			} {
				require.NoError(t, database.db.Create(&payload).Error)
			}
		}
		addEvidenceDuplicates := func(t *testing.T, database *Database) {
			t.Helper()
			capturedAt := time.Date(2026, time.July, 17, 11, 0, 0, 0, time.FixedZone("test", 2*60*60))
			evidence := legacyProviderEvidenceModel{
				TenantID:             "tenant-" + fake.UUID().V4(),
				ConnectionID:         "connection-" + fake.UUID().V4(),
				FinanceAccountID:     "account-" + fake.UUID().V4(),
				FinanceTransactionID: "transaction-" + fake.UUID().V4(),
				Subject:              "transaction",
				Scope:                "transaction",
				ProviderObjectID:     "provider-object-" + fake.UUID().V4(),
			}
			makeEvidence := func(id string, capturedAt time.Time) legacyProviderEvidenceModel {
				item := evidence
				item.ID = id
				item.PayloadJSON = `{}`
				item.CapturedAt = capturedAt
				return item
			}
			for _, item := range []legacyProviderEvidenceModel{
				makeEvidence("evidence-first-"+fake.UUID().V4(), capturedAt),
				makeEvidence("evidence-second-"+fake.UUID().V4(), capturedAt.Add(time.Minute)),
			} {
				require.NoError(t, database.db.Create(&item).Error)
			}
		}

		t.Run("raw payload deletion", func(t *testing.T) {
			database := makeDatabase(t, &legacyRawPayloadModel{})
			addRawDuplicates(t, database)
			expectedErr := errors.New("delete raw payload")
			database.db.Callback().Delete().Before("gorm:delete").Register(
				"test:fail-raw-payload-delete-"+fake.UUID().V4(),
				func(db *gorm.DB) {
					if db.Statement.Table == (rawPayloadModel{}).TableName() {
						db.AddError(expectedErr)
					}
				},
			)
			err := NewMigrator(database).Migrate(t.Context())
			require.ErrorIs(t, err, expectedErr)
		})

		t.Run("provider evidence deletion", func(t *testing.T) {
			database := makeDatabase(t, &legacyProviderEvidenceModel{})
			addEvidenceDuplicates(t, database)
			expectedErr := errors.New("delete provider evidence")
			database.db.Callback().Delete().Before("gorm:delete").Register(
				"test:fail-provider-evidence-delete-"+fake.UUID().V4(),
				func(db *gorm.DB) {
					if db.Statement.Table == (providerEvidenceModel{}).TableName() {
						db.AddError(expectedErr)
					}
				},
			)
			err := NewMigrator(database).Migrate(t.Context())
			require.ErrorIs(t, err, expectedErr)
		})
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
