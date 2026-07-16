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
		&connectionProviderAccountModel{},
		&balanceSnapshotModel{},
		&rawPayloadModel{},
		&providerEvidenceModel{},
		&bankConnectionSyncRunModel{},
		&providerSyncStateJournalModel{},
		&providerTransactionMatchModel{},
		&syntheticProviderStateModel{},
	}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.db.WithContext(ctx).AutoMigrate(financeSchemaModels()...); err != nil {
		return fmt.Errorf("auto-migrate finance schema: %w", err)
	}

	return nil
}
