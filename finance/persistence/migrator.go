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
		&bankConnectionSyncRunModel{},
		&providerSyncStateJournalModel{},
		&providerTransactionMatchModel{},
		&syntheticProviderStateModel{},
	}
}

func currentObservationSchemaModels() []any {
	return []any{&rawPayloadModel{}, &providerEvidenceModel{}}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.removeRetiredBankConnectionIdentitySchema(ctx); err != nil {
		return fmt.Errorf("remove retired bank connection identity schema: %w", err)
	}
	if err := m.db.WithContext(ctx).AutoMigrate(financeSchemaModels()...); err != nil {
		return fmt.Errorf("auto-migrate finance schema: %w", err)
	}
	if err := m.compactCurrentObservations(ctx); err != nil {
		return fmt.Errorf("compact finance current observations: %w", err)
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

func (m *Migrator) compactCurrentObservations(ctx context.Context) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := compactRawPayloadObservations(tx); err != nil {
			return err
		}
		if err := compactProviderEvidenceObservations(tx); err != nil {
			return err
		}
		return nil
	})
}

func compactRawPayloadObservations(db *gorm.DB) error {
	if !db.Migrator().HasTable(&rawPayloadModel{}) ||
		db.Migrator().HasIndex(&rawPayloadModel{}, "idx_finance_raw_payloads_identity") {
		return nil
	}
	var observations []rawPayloadModel
	if err := db.Order("captured_at DESC, id DESC").Find(&observations).Error; err != nil {
		return fmt.Errorf("list raw payload observations: %w", err)
	}
	seen := make(map[rawPayloadObservationIdentity]struct{}, len(observations))
	for _, observation := range observations {
		identity := rawPayloadObservationIdentity{
			connectionID:     observation.ConnectionID,
			scope:            observation.Scope,
			providerObjectID: observation.ProviderObjectID,
		}
		if _, exists := seen[identity]; !exists {
			seen[identity] = struct{}{}
			continue
		}
		if err := db.Delete(&rawPayloadModel{}, "id = ?", observation.ID).Error; err != nil {
			return fmt.Errorf("delete duplicate raw payload observation: %w", err)
		}
	}
	return nil
}

type rawPayloadObservationIdentity struct {
	connectionID     string
	scope            string
	providerObjectID string
}

func compactProviderEvidenceObservations(db *gorm.DB) error {
	if !db.Migrator().HasTable(&providerEvidenceModel{}) ||
		db.Migrator().HasIndex(&providerEvidenceModel{}, "idx_finance_provider_evidence_identity") {
		return nil
	}
	var observations []providerEvidenceModel
	if err := db.Order("captured_at DESC, id DESC").Find(&observations).Error; err != nil {
		return fmt.Errorf("list provider evidence observations: %w", err)
	}
	seen := make(map[providerEvidenceObservationIdentity]struct{}, len(observations))
	for _, observation := range observations {
		identity := providerEvidenceObservationIdentity{
			tenantID:             observation.TenantID,
			connectionID:         observation.ConnectionID,
			subject:              observation.Subject,
			financeAccountID:     observation.FinanceAccountID,
			financeTransactionID: observation.FinanceTransactionID,
			scope:                observation.Scope,
			providerObjectID:     observation.ProviderObjectID,
		}
		if _, exists := seen[identity]; !exists {
			seen[identity] = struct{}{}
			continue
		}
		if err := db.Delete(&providerEvidenceModel{}, "id = ?", observation.ID).Error; err != nil {
			return fmt.Errorf("delete duplicate provider evidence observation: %w", err)
		}
	}
	return nil
}

type providerEvidenceObservationIdentity struct {
	tenantID             string
	connectionID         string
	subject              string
	financeAccountID     string
	financeTransactionID string
	scope                string
	providerObjectID     string
}
