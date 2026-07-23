package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBankConnectionNotFound                 = errors.New("bank connection not found")
	ErrPendingBankConnectionLinkStartNotFound = errors.New("pending bank connection link start not found")
	ErrBankConnectionScheduleNotFound         = errors.New("bank connection schedule not found")
	ErrBankConnectionSyncRunNotFound          = errors.New("bank connection sync run not found")
	ErrProviderTransactionMatchNotFound       = errors.New("provider transaction match not found")
)

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) SaveBankConnection(
	ctx context.Context,
	connection domain.BankConnection,
) (domain.BankConnection, error) {
	model := newBankConnectionModel(connection)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				columnTenantID,
				"provider",
				"connector_id",
				"display_name",
				"provider_reference",
				"external_id",
				"secret_id",
				columnState,
				"reauth_required_at",
				"reauth_reason",
				"last_sync_job_id",
				"last_sync_started_at",
				"last_successful_sync_at",
				"last_sync_error",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.BankConnection{}, fmt.Errorf("save bank connection: %w", err)
	}
	return bankConnectionFromModel(model), nil
}

func (s *Store) GetBankConnection(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnection, error) {
	var model bankConnectionModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ?", strings.TrimSpace(connectionID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBankConnectionNotFound
		}
		return nil, fmt.Errorf("get bank connection: %w", err)
	}
	connection := bankConnectionFromModel(model)
	return &connection, nil
}

func (s *Store) ListBankConnections(
	ctx context.Context,
	tenantID string,
) ([]domain.BankConnection, error) {
	var models []bankConnectionModel
	if err := s.db.WithContext(ctx).
		Table((bankConnectionModel{}).TableName()).
		Where("tenant_id = ?", strings.TrimSpace(tenantID)).
		Order("created_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list bank connections: %w", err)
	}
	items := make([]domain.BankConnection, 0, len(models))
	for _, model := range models {
		items = append(items, bankConnectionFromModel(model))
	}
	return items, nil
}

func (s *Store) DeleteBankConnection(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((bankConnectionModel{}).TableName()).
		Where("id = ?", strings.TrimSpace(connectionID)).
		Delete(&bankConnectionModel{}).Error; err != nil {
		return fmt.Errorf("delete bank connection: %w", err)
	}
	return nil
}

func (s *Store) SavePendingBankConnectionLinkStart(
	ctx context.Context,
	start domain.PendingBankConnectionLinkStart,
) (domain.PendingBankConnectionLinkStart, error) {
	model := newPendingBankConnectionLinkStartModel(start)
	if model.CreatedAt.IsZero() {
		model.CreatedAt = s.now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return domain.PendingBankConnectionLinkStart{}, fmt.Errorf(
			"save pending bank connection link start: %w",
			err,
		)
	}
	return pendingBankConnectionLinkStartFromModel(model), nil
}

func (s *Store) ConsumePendingBankConnectionLinkStart(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	provider string,
	state string,
	consumedAt time.Time,
) (*domain.PendingBankConnectionLinkStart, error) {
	lookup := pendingBankConnectionLinkStartModel{}
	trimmedTenantID := strings.TrimSpace(tenantID)
	trimmedActorUserID := strings.TrimSpace(actorUserID)
	trimmedProvider := strings.TrimSpace(provider)
	trimmedState := strings.TrimSpace(state)
	normalizedConsumedAt := consumedAt

	result := s.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND state = ? AND consumed_at IS NULL AND "+
				expiresAfterPredicate(s.db),
			trimmedTenantID,
			trimmedActorUserID,
			trimmedProvider,
			trimmedState,
			normalizedConsumedAt,
		).
		Updates(map[string]any{"consumed_at": normalizedConsumedAt, columnUpdatedAt: normalizedConsumedAt})
	if result.Error != nil {
		return nil, fmt.Errorf("consume pending bank connection link start: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrPendingBankConnectionLinkStartNotFound
	}

	if err := s.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND state = ? AND consumed_at = ?",
			trimmedTenantID,
			trimmedActorUserID,
			trimmedProvider,
			trimmedState,
			normalizedConsumedAt,
		).
		First(&lookup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPendingBankConnectionLinkStartNotFound
		}
		return nil, fmt.Errorf("get pending bank connection link start: %w", err)
	}
	start := pendingBankConnectionLinkStartFromModel(lookup)
	return &start, nil
}

func (s *Store) RestorePendingBankConnectionLinkStart(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	provider string,
	state string,
	restoredAt time.Time,
) error {
	lookup := pendingBankConnectionLinkStartModel{}
	trimmedTenantID := strings.TrimSpace(tenantID)
	trimmedActorUserID := strings.TrimSpace(actorUserID)
	trimmedProvider := strings.TrimSpace(provider)
	trimmedState := strings.TrimSpace(state)
	normalizedRestoredAt := restoredAt

	result := s.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND state = ? AND consumed_at IS NOT NULL",
			trimmedTenantID,
			trimmedActorUserID,
			trimmedProvider,
			trimmedState,
		).
		Updates(map[string]any{"consumed_at": nil, columnUpdatedAt: normalizedRestoredAt})
	if result.Error != nil {
		return fmt.Errorf("restore pending bank connection link start: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPendingBankConnectionLinkStartNotFound
	}
	return nil
}

func (s *Store) GetPendingBankConnectionLinkStartByState(
	ctx context.Context,
	provider string,
	state string,
) (*domain.PendingBankConnectionLinkStart, error) {
	var model pendingBankConnectionLinkStartModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where(
			"provider = ? AND state = ?",
			strings.TrimSpace(provider),
			strings.TrimSpace(state),
		).
		Order("created_at DESC, id DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPendingBankConnectionLinkStartNotFound
		}
		return nil, fmt.Errorf("get pending bank connection link start by state: %w", err)
	}
	start := pendingBankConnectionLinkStartFromModel(model)
	return &start, nil
}

func (s *Store) SaveBankConnectionSchedule(
	ctx context.Context,
	schedule domain.BankConnectionSchedule,
) (domain.BankConnectionSchedule, error) {
	model := newBankConnectionScheduleModel(schedule)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: columnConnectionID}},
			DoUpdates: clause.AssignmentColumns([]string{
				"interval_seconds",
				"next_run_at",
				"last_scheduled_at",
				"last_started_at",
				"last_completed_at",
				"last_job_id",
				"enabled",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.BankConnectionSchedule{}, fmt.Errorf("save bank connection schedule: %w", err)
	}
	return bankConnectionScheduleFromModel(model), nil
}

func (s *Store) GetBankConnectionSchedule(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnectionSchedule, error) {
	var model bankConnectionScheduleModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBankConnectionScheduleNotFound
		}
		return nil, fmt.Errorf("get bank connection schedule: %w", err)
	}
	schedule := bankConnectionScheduleFromModel(model)
	return &schedule, nil
}

func (s *Store) DeleteBankConnectionSchedule(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((bankConnectionScheduleModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&bankConnectionScheduleModel{}).Error; err != nil {
		return fmt.Errorf("delete bank connection schedule: %w", err)
	}
	return nil
}

func (s *Store) SaveConnectionProviderAccount(
	ctx context.Context,
	account domain.ConnectionProviderAccount,
) (domain.ConnectionProviderAccount, error) {
	model := newConnectionProviderAccountModel(account)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: columnConnectionID}, {Name: columnProviderAccountID}},
			DoUpdates: clause.AssignmentColumns([]string{
				"finance_account_id",
				columnName,
				columnCurrency,
				"iban",
				"masked_pan",
				"last_successful_sync_at",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.ConnectionProviderAccount{}, fmt.Errorf(
			"save connection provider account: %w",
			err,
		)
	}
	return connectionProviderAccountFromModel(model), nil
}

func (s *Store) ListConnectionProviderAccounts(
	ctx context.Context,
	connectionID string,
) ([]domain.ConnectionProviderAccount, error) {
	var models []connectionProviderAccountModel
	if err := s.db.WithContext(ctx).
		Table((connectionProviderAccountModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Order("created_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list connection provider accounts: %w", err)
	}
	items := make([]domain.ConnectionProviderAccount, 0, len(models))
	for _, model := range models {
		items = append(items, connectionProviderAccountFromModel(model))
	}
	return items, nil
}

func (s *Store) DeleteConnectionProviderAccounts(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((connectionProviderAccountModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&connectionProviderAccountModel{}).Error; err != nil {
		return fmt.Errorf("delete connection provider accounts: %w", err)
	}
	return nil
}

func (s *Store) SaveBalanceSnapshot(
	ctx context.Context,
	snapshot domain.BalanceSnapshot,
) (domain.BalanceSnapshot, error) {
	model := newBalanceSnapshotModel(snapshot)
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return domain.BalanceSnapshot{}, fmt.Errorf("save balance snapshot: %w", err)
	}
	return balanceSnapshotFromModel(model), nil
}

func (s *Store) ListBalanceSnapshots(
	ctx context.Context,
	connectionID string,
) ([]domain.BalanceSnapshot, error) {
	var models []balanceSnapshotModel
	if err := s.db.WithContext(ctx).
		Table((balanceSnapshotModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Order("captured_at DESC, id DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}
	items := make([]domain.BalanceSnapshot, 0, len(models))
	for _, model := range models {
		items = append(items, balanceSnapshotFromModel(model))
	}
	return items, nil
}

func (s *Store) DeleteBalanceSnapshots(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((balanceSnapshotModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&balanceSnapshotModel{}).Error; err != nil {
		return fmt.Errorf("delete balance snapshots: %w", err)
	}
	return nil
}

func (s *Store) SaveRawPayload(
	ctx context.Context,
	payload domain.RawPayload,
) (domain.RawPayload, error) {
	sanitizedPayload, sanitizeErr := domain.SanitizeProviderEvidenceJSON(payload.PayloadJSON)
	if sanitizeErr != nil {
		return domain.RawPayload{}, fmt.Errorf("sanitize raw payload: %w", sanitizeErr)
	}
	payload.PayloadJSON = sanitizedPayload
	model := newRawPayloadModel(payload)
	if saveErr := s.db.WithContext(ctx).Table(model.TableName()).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "connection_id"},
			{Name: "scope"},
			{Name: "provider_object_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"payload_json": model.PayloadJSON,
			"captured_at":  model.CapturedAt,
		}),
		Where: latestProviderObservationClause(),
	}).Create(&model).Error; saveErr != nil {
		return domain.RawPayload{}, fmt.Errorf("save raw payload: %w", saveErr)
	}
	var persisted rawPayloadModel
	if readErr := s.db.WithContext(ctx).Table(model.TableName()).
		Where(
			"connection_id = ? AND scope = ? AND provider_object_id = ?",
			model.ConnectionID,
			model.Scope,
			model.ProviderObjectID,
		).
		First(&persisted).Error; readErr != nil {
		return domain.RawPayload{}, fmt.Errorf("read saved raw payload: %w", readErr)
	}
	return rawPayloadFromModel(persisted), nil
}

func (s *Store) ListRawPayloads(
	ctx context.Context,
	connectionID string,
) ([]domain.RawPayload, error) {
	var models []rawPayloadModel
	if err := s.db.WithContext(ctx).
		Table((rawPayloadModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Order("captured_at ASC, id ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list raw payloads: %w", err)
	}
	items := make([]domain.RawPayload, 0, len(models))
	for _, model := range models {
		items = append(items, rawPayloadFromModel(model))
	}
	return items, nil
}

func (s *Store) DeleteRawPayloads(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((rawPayloadModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&rawPayloadModel{}).Error; err != nil {
		return fmt.Errorf("delete raw payloads: %w", err)
	}
	return nil
}

func (s *Store) SaveBankConnectionSyncRun(
	ctx context.Context,
	run domain.BankConnectionSyncRun,
) (domain.BankConnectionSyncRun, error) {
	model := newBankConnectionSyncRunModel(run)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model).Error; err != nil {
		return domain.BankConnectionSyncRun{}, fmt.Errorf("save bank connection sync run: %w", err)
	}
	return bankConnectionSyncRunFromModel(model), nil
}

func (s *Store) ClaimBankConnectionSyncRun(
	ctx context.Context,
	run domain.BankConnectionSyncRun,
) (bool, error) {
	model := newBankConnectionSyncRunModel(run)
	result := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model)
	if result.Error != nil {
		return false, fmt.Errorf("claim bank connection sync run: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (s *Store) GetBankConnectionSyncRun(
	ctx context.Context,
	connectionID string,
	syncKey string,
) (*domain.BankConnectionSyncRun, error) {
	var model bankConnectionSyncRunModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("connection_id = ? AND sync_key = ?", strings.TrimSpace(connectionID), strings.TrimSpace(syncKey)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBankConnectionSyncRunNotFound
		}
		return nil, fmt.Errorf("get bank connection sync run: %w", err)
	}
	run := bankConnectionSyncRunFromModel(model)
	return &run, nil
}

func (s *Store) DeleteBankConnectionSyncRuns(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((bankConnectionSyncRunModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&bankConnectionSyncRunModel{}).Error; err != nil {
		return fmt.Errorf("delete bank connection sync runs: %w", err)
	}
	return nil
}

func (s *Store) GetProviderTransactionMatchByProviderID(
	ctx context.Context,
	connectionID string,
	providerAccountID string,
	providerTransactionID string,
) (*domain.ProviderTransactionMatch, error) {
	if strings.TrimSpace(providerTransactionID) == "" {
		return nil, ErrProviderTransactionMatchNotFound
	}
	var model providerTransactionMatchModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where(
			"connection_id = ? AND provider_account_id = ? AND provider_transaction_id = ?",
			strings.TrimSpace(connectionID),
			strings.TrimSpace(providerAccountID),
			strings.TrimSpace(providerTransactionID),
		).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderTransactionMatchNotFound
		}
		return nil, fmt.Errorf("get provider transaction match by provider id: %w", err)
	}
	match := providerTransactionMatchFromModel(model)
	return &match, nil
}

func (s *Store) GetProviderTransactionMatchByFingerprint(
	ctx context.Context,
	connectionID string,
	providerAccountID string,
	fingerprint string,
) (*domain.ProviderTransactionMatch, error) {
	if strings.TrimSpace(fingerprint) == "" {
		return nil, ErrProviderTransactionMatchNotFound
	}
	var model providerTransactionMatchModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where(
			"connection_id = ? AND provider_account_id = ? AND fingerprint = ?",
			strings.TrimSpace(connectionID),
			strings.TrimSpace(providerAccountID),
			strings.TrimSpace(fingerprint),
		).
		Order("updated_at DESC").
		Order("id DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderTransactionMatchNotFound
		}
		return nil, fmt.Errorf("get provider transaction match by fingerprint: %w", err)
	}
	match := providerTransactionMatchFromModel(model)
	return &match, nil
}

func (s *Store) SaveProviderTransactionMatch(
	ctx context.Context,
	match domain.ProviderTransactionMatch,
) (domain.ProviderTransactionMatch, error) {
	model := newProviderTransactionMatchModel(match)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"provider_transaction_id",
				columnFingerprint,
				"transaction_id",
				"status",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.ProviderTransactionMatch{}, fmt.Errorf(
			"save provider transaction match: %w",
			err,
		)
	}
	return providerTransactionMatchFromModel(model), nil
}

func (s *Store) DeleteProviderTransactionMatches(ctx context.Context, connectionID string) error {
	if err := s.db.WithContext(ctx).
		Table((providerTransactionMatchModel{}).TableName()).
		Where("connection_id = ?", strings.TrimSpace(connectionID)).
		Delete(&providerTransactionMatchModel{}).Error; err != nil {
		return fmt.Errorf("delete provider transaction matches: %w", err)
	}
	return nil
}
