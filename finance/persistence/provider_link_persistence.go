package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"gorm.io/gorm"
)

const columnConsumedAt = "consumed_at"

type ProviderLinkPersistence struct {
	*Store
}

func NewProviderLinkPersistence(store *Store) *ProviderLinkPersistence {
	return &ProviderLinkPersistence{Store: store}
}

func (p *ProviderLinkPersistence) SavePendingStart(
	ctx context.Context,
	start domain.PendingBankConnectionLinkStart,
) (domain.PendingBankConnectionLinkStart, error) {
	return p.Store.SavePendingBankConnectionLinkStart(ctx, start)
}

func (p *ProviderLinkPersistence) ConsumePendingStart(
	ctx context.Context,
	request providers.ConsumePendingStartRequest,
) (*domain.PendingBankConnectionLinkStart, error) {
	lookup := pendingBankConnectionLinkStartModel{}
	provider := strings.TrimSpace(string(request.ProviderID))
	result := p.Store.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND connector_id = ? AND state = ? AND "+
				columnConsumedAt+" IS NULL AND "+expiresAfterPredicate(p.Store.db),
			strings.TrimSpace(request.TenantID),
			strings.TrimSpace(request.ActorUserID),
			provider,
			strings.TrimSpace(string(request.ConnectorID)),
			strings.TrimSpace(request.State),
			request.ConsumedAt,
		).
		Updates(map[string]any{columnConsumedAt: request.ConsumedAt, columnUpdatedAt: request.ConsumedAt})
	if result.Error != nil {
		return nil, fmt.Errorf("consume pending start: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, providers.ErrPendingStartNotFound
	}
	if err := p.Store.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND connector_id = ? AND state = ? AND "+columnConsumedAt+" = ?",
			strings.TrimSpace(request.TenantID),
			strings.TrimSpace(request.ActorUserID),
			provider,
			strings.TrimSpace(string(request.ConnectorID)),
			strings.TrimSpace(request.State),
			request.ConsumedAt,
		).
		First(&lookup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, providers.ErrPendingStartNotFound
		}
		return nil, fmt.Errorf("get consumed pending start: %w", err)
	}
	start := pendingBankConnectionLinkStartFromModel(lookup)
	return &start, nil
}

func (p *ProviderLinkPersistence) RestorePendingStart(
	ctx context.Context,
	request providers.RestorePendingStartRequest,
) error {
	lookup := pendingBankConnectionLinkStartModel{}
	result := p.Store.db.WithContext(ctx).
		Table(lookup.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND connector_id = ? AND state = ? AND "+columnConsumedAt+" IS NOT NULL",
			strings.TrimSpace(request.TenantID),
			strings.TrimSpace(request.ActorUserID),
			strings.TrimSpace(string(request.ProviderID)),
			strings.TrimSpace(string(request.ConnectorID)),
			strings.TrimSpace(request.State),
		).
		Updates(map[string]any{columnConsumedAt: nil, columnUpdatedAt: request.RestoredAt})
	if result.Error != nil {
		return fmt.Errorf("restore pending start: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return providers.ErrPendingStartNotFound
	}
	return nil
}

func (p *ProviderLinkPersistence) GetPendingStartByState(
	ctx context.Context,
	providerID domain.ProviderID,
	state string,
) (*domain.PendingBankConnectionLinkStart, error) {
	return p.Store.GetPendingBankConnectionLinkStartByState(ctx, string(providerID), state)
}

func (p *ProviderLinkPersistence) SaveBankConnection(
	ctx context.Context,
	connection domain.BankConnection,
) (domain.BankConnection, error) {
	return p.Store.SaveBankConnection(ctx, connection)
}

// SaveLinkedConnection stores a newly encrypted secret and its connection as one unit.
// A matching non-empty provider reference is idempotent.
func (p *ProviderLinkPersistence) SaveLinkedConnection(
	ctx context.Context,
	connection domain.BankConnection,
	secret domain.ConnectionSecret,
) (domain.BankConnection, error) {
	var saved domain.BankConnection
	err := p.Store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, found, lookupErr := findProviderLinkConnection(tx, connection)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			saved = existing
			return nil
		}

		secretModel := newConnectionSecretModel(secret)
		if secretModel.CreatedAt.IsZero() {
			secretModel.CreatedAt = p.Store.now()
		}
		if secretModel.UpdatedAt.IsZero() {
			secretModel.UpdatedAt = secretModel.CreatedAt
		}
		if createErr := tx.Table(secretModel.TableName()).Create(&secretModel).Error; createErr != nil {
			return fmt.Errorf("create connection secret: %w", createErr)
		}

		connectionModel := newBankConnectionModel(connection)
		if createErr := tx.Table(connectionModel.TableName()).Create(&connectionModel).Error; createErr != nil {
			return fmt.Errorf("create bank connection: %w", createErr)
		}
		saved = bankConnectionFromModel(connectionModel)
		return nil
	})
	if err == nil {
		return saved, nil
	}

	existing, found, lookupErr := p.recoverProviderLinkConnection(ctx, connection)
	if lookupErr != nil {
		return domain.BankConnection{}, lookupErr
	}
	if found {
		return existing, nil
	}
	return domain.BankConnection{}, fmt.Errorf("save linked connection: %w", err)
}

// SaveLinkedConnectionWithSnapshot persists the encrypted secret, durable
// connection, and final typed source snapshot in one database transaction.
func (p *ProviderLinkPersistence) SaveLinkedConnectionWithSnapshot(
	ctx context.Context,
	connection domain.BankConnection,
	secret domain.ConnectionSecret,
	snapshot *domain.ProviderSnapshot,
) (domain.BankConnection, error) {
	var saved domain.BankConnection
	connectionInsertFailed := false
	err := p.Store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, found, lookupErr := findProviderLinkConnection(tx, connection)
		if lookupErr != nil {
			return lookupErr
		}
		//nolint:nestif // Link persistence keeps the connection, secret, and snapshot transaction together.
		if found {
			saved = existing
		} else {
			secretModel := newConnectionSecretModel(secret)
			if secretModel.CreatedAt.IsZero() {
				secretModel.CreatedAt = p.Store.now()
			}
			if secretModel.UpdatedAt.IsZero() {
				secretModel.UpdatedAt = secretModel.CreatedAt
			}
			if createErr := tx.Table(secretModel.TableName()).Create(&secretModel).Error; createErr != nil {
				return fmt.Errorf("create connection secret: %w", createErr)
			}
			connectionModel := newBankConnectionModel(connection)
			if createErr := tx.Table(connectionModel.TableName()).Create(&connectionModel).Error; createErr != nil {
				connectionInsertFailed = true
				return fmt.Errorf("create bank connection: %w", createErr)
			}
			saved = bankConnectionFromModel(connectionModel)
		}
		return saveLinkedConnectionSnapshot(ctx, tx, saved.ID, snapshot)
	})
	if err == nil {
		return saved, nil
	}
	if !connectionInsertFailed {
		return domain.BankConnection{}, fmt.Errorf("save linked connection with snapshot: %w", err)
	}

	existing, found, lookupErr := p.recoverProviderLinkConnection(ctx, connection)
	if lookupErr != nil {
		return domain.BankConnection{}, lookupErr
	}
	if found {
		return existing, nil
	}
	return domain.BankConnection{}, fmt.Errorf("save linked connection with snapshot: %w", err)
}

func saveLinkedConnectionSnapshot(
	ctx context.Context,
	tx *gorm.DB,
	connectionID string,
	snapshot *domain.ProviderSnapshot,
) error {
	if snapshot == nil {
		return nil
	}
	attached := *snapshot
	attached.ConnectionID = connectionID
	if _, err := (&ProviderSnapshotStore{db: tx}).SaveProviderSnapshot(ctx, attached); err != nil {
		return fmt.Errorf("save linked connection provider snapshot: %w", err)
	}
	return nil
}

func (p *ProviderLinkPersistence) recoverProviderLinkConnection(
	ctx context.Context,
	connection domain.BankConnection,
) (domain.BankConnection, bool, error) {
	// A concurrent insert can win after this transaction's initial lookup. The
	// uniqueness index decides the winner; loading it here makes the finish retry-safe.
	return findProviderLinkConnection(p.Store.db.WithContext(ctx), connection)
}

func findProviderLinkConnection(
	db *gorm.DB,
	candidate domain.BankConnection,
) (domain.BankConnection, bool, error) {
	if candidate.ProviderReference == "" {
		return domain.BankConnection{}, false, nil
	}
	var model bankConnectionModel
	if err := db.Table((bankConnectionModel{}).TableName()).
		Where(
			"tenant_id = ? AND provider = ? AND connector_id = ? AND provider_reference = ?",
			strings.TrimSpace(candidate.TenantID),
			strings.TrimSpace(candidate.Provider),
			strings.TrimSpace(string(candidate.ConnectorID)),
			candidate.ProviderReference,
		).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.BankConnection{}, false, nil
		}
		return domain.BankConnection{}, false, fmt.Errorf("find provider link connection: %w", err)
	}
	return bankConnectionFromModel(model), true, nil
}

func (p *ProviderLinkPersistence) GetBankConnection(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnection, error) {
	return p.Store.GetBankConnection(ctx, connectionID)
}

func (p *ProviderLinkPersistence) UpdateBankConnectionDisplayName(
	ctx context.Context,
	tenantID string,
	connectionID string,
	displayName string,
	updatedAt time.Time,
) error {
	model := bankConnectionModel{}
	result := p.Store.db.WithContext(ctx).
		Table(model.TableName()).
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(connectionID), strings.TrimSpace(tenantID)).
		Updates(map[string]any{"display_name": displayName, columnUpdatedAt: updatedAt})
	if result.Error != nil {
		return fmt.Errorf("update bank connection display name: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrBankConnectionNotFound
	}
	return nil
}

func (p *ProviderLinkPersistence) ListBankConnections(
	ctx context.Context,
	tenantID string,
) ([]domain.BankConnection, error) {
	return p.Store.ListBankConnections(ctx, tenantID)
}
