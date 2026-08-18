package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm"
)

type ProviderSyncStateJournalStore struct {
	db  *gorm.DB
	now func() time.Time
}

func NewProviderSyncStateJournalStore(store *Store) *ProviderSyncStateJournalStore {
	return &ProviderSyncStateJournalStore{
		db:  store.db,
		now: store.now,
	}
}

func (s *ProviderSyncStateJournalStore) LoadLastState(
	ctx context.Context,
	connection domain.ProviderConnectionRef,
) (*domain.ProviderSyncState, error) {
	var model providerSyncStateJournalModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("connection_id = ?", connection.ConnectionID).
		Order("created_at DESC, journal_id DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			//nolint:nilnil // Empty journal is a documented non-error result for this seam.
			return nil, nil
		}
		return nil, fmt.Errorf("load provider sync state journal: %w", err)
	}

	state := providerSyncStateFromJournalModel(model, connection)
	return &state, nil
}

func (s *ProviderSyncStateJournalStore) AppendSyncState(
	ctx context.Context,
	state domain.ProviderSyncState,
) error {
	model := newProviderSyncStateJournalModel(state, s.now())
	if err := s.db.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return fmt.Errorf("append provider sync state journal: %w", err)
	}

	return nil
}

func (s *ProviderSyncStateJournalStore) DeleteSyncStatesByConnection(
	ctx context.Context,
	connectionID string,
) error {
	model := providerSyncStateJournalModel{}
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("connection_id = ?", connectionID).
		Delete(&model).Error; err != nil {
		return fmt.Errorf("delete provider sync state journal: %w", err)
	}
	return nil
}
