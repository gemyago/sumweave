package persistence

import (
	"context"
	"fmt"
)

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
		&fxRateModel{},
		&csvImportModel{},
		&bankConnectionModel{},
		&pendingBankConnectionLinkStartModel{},
		&bankConnectionScheduleModel{},
		&connectionProviderAccountModel{},
		&balanceSnapshotModel{},
		&rawPayloadModel{},
		&bankConnectionSyncRunModel{},
		&providerTransactionMatchModel{},
	}
}

func (s *Store) Migrate(ctx context.Context) error {
	if err := s.db.WithContext(ctx).AutoMigrate(financeSchemaModels()...); err != nil {
		return fmt.Errorf("auto-migrate finance schema: %w", err)
	}

	return nil
}
