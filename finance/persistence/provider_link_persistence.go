package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
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

func (p *ProviderLinkPersistence) ListBankConnections(
	ctx context.Context,
	tenantID string,
) ([]domain.BankConnection, error) {
	return p.Store.ListBankConnections(ctx, tenantID)
}

func (p *ProviderLinkPersistence) SaveRawPayload(
	ctx context.Context,
	payload domain.RawPayload,
) (domain.RawPayload, error) {
	return p.Store.SaveRawPayload(ctx, payload)
}
