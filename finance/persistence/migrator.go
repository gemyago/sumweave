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
	if err := m.migrateModels(ctx, "auto-migrate finance schema", financeSchemaModels()...); err != nil {
		return err
	}
	if err := m.migrateModels(
		ctx,
		"auto-migrate finance current observations",
		currentObservationSchemaModels()...); err != nil {
		return err
	}

	return nil
}

func (m *Migrator) migrateModels(ctx context.Context, operation string, models ...any) error {
	if err := m.db.WithContext(ctx).AutoMigrate(models...); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return nil
}
