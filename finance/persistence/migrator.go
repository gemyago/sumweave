package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Migrator struct {
	autoMigrator autoMigrator
}

func NewMigrator(database *Database) *Migrator {
	return &Migrator{autoMigrator: &gormAutoMigrator{db: database.db}}
}

type autoMigrator interface {
	AutoMigrate(context.Context, []any) error
}

type gormAutoMigrator struct {
	db *gorm.DB
}

func (m *gormAutoMigrator) AutoMigrate(ctx context.Context, models []any) error {
	return m.db.WithContext(ctx).AutoMigrate(models...)
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
	if err := m.autoMigrator.AutoMigrate(ctx, financeSchemaModels()); err != nil {
		return fmt.Errorf("auto-migrate finance schema: %w", err)
	}
	if err := m.autoMigrator.AutoMigrate(ctx, currentObservationSchemaModels()); err != nil {
		return fmt.Errorf("auto-migrate finance current observations: %w", err)
	}

	return nil
}
