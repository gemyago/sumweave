package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(database *Database) *Migrator {
	return &Migrator{db: database.db}
}

func financeSchemaModels() []any {
	return []any{
		&connectionSecretModel{},
		&fixtureBootstrapRunModel{},
		&fixtureScenarioRecordModel{},
		&tenantModel{},
		&tenantMembershipModel{},
		&tenantInviteModel{},
		&accountModel{},
		&categoryModel{},
		&tagModel{},
		&transactionModel{},
		&transactionTagModel{},
		&currentFXRateModel{},
		&csvImportModel{},
		&csvImportRowOutcomeModel{},
		&bankConnectionModel{},
		&pendingBankConnectionLinkStartModel{},
		&bankConnectionScheduleModel{},
		&fxRefreshScheduleModel{},
		&connectionProviderAccountModel{},
		&balanceSnapshotModel{},
		&providerSyncStateJournalModel{},
		&providerTransactionMatchModel{},
		&syntheticProviderStateModel{},
	}
}

func currentObservationSchemaModels() []any {
	return []any{&providerSnapshotModel{}}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.removeRetiredBankConnectionIdentitySchema(ctx); err != nil {
		return fmt.Errorf("remove retired bank connection identity schema: %w", err)
	}
	if err := m.db.WithContext(ctx).AutoMigrate(financeSchemaModels()...); err != nil {
		return fmt.Errorf("auto-migrate finance schema: %w", err)
	}
	if err := m.db.WithContext(ctx).AutoMigrate(currentObservationSchemaModels()...); err != nil {
		return fmt.Errorf("auto-migrate finance current observations: %w", err)
	}

	return nil
}

func (m *Migrator) removeRetiredBankConnectionIdentitySchema(ctx context.Context) error {
	db := m.db.WithContext(ctx)
	model := &bankConnectionModel{}
	if !db.Migrator().HasTable(model) {
		return nil
	}
	if db.Migrator().HasIndex(model, "idx_finance_bank_connections_link_identity") {
		if err := db.Migrator().DropIndex(model, "idx_finance_bank_connections_link_identity"); err != nil {
			return fmt.Errorf("drop retired link identity index: %w", err)
		}
	}
	for _, column := range []string{"provider_link_identity", "external_id"} {
		if db.Migrator().HasColumn(model, column) {
			if err := db.Migrator().DropColumn(model, column); err != nil {
				return fmt.Errorf("drop retired %s column: %w", column, err)
			}
		}
	}
	return nil
}
