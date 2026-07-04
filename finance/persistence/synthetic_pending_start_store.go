package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
)

type SyntheticPendingStartStore struct {
	db *gorm.DB
}

func NewSyntheticPendingStartStore(database *Database) *SyntheticPendingStartStore {
	return &SyntheticPendingStartStore{db: database.db}
}

func NewSyntheticPendingStartStoreFromStore(store *Store) *SyntheticPendingStartStore {
	if store == nil {
		return nil
	}
	return &SyntheticPendingStartStore{db: store.db}
}

func (s *SyntheticPendingStartStore) GetPendingSyntheticStart(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	state string,
	now time.Time,
) (*domain.PendingBankConnectionLinkStart, error) {
	var model pendingBankConnectionLinkStartModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where(
			"tenant_id = ? AND actor_user_id = ? AND provider = ? AND connector_id = ? AND state = ? AND consumed_at IS NULL AND expires_at > ?",
			strings.TrimSpace(tenantID),
			strings.TrimSpace(actorUserID),
			string(domain.ProviderIDSynthetic),
			string(domain.ProviderConnectorIDSynthetic),
			strings.TrimSpace(state),
			now.UTC(),
		).
		Order("created_at DESC, id DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPendingBankConnectionLinkStartNotFound
		}
		return nil, fmt.Errorf("get pending synthetic start: %w", err)
	}
	start := pendingBankConnectionLinkStartFromModel(model)
	return &start, nil
}
