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
	fence bool
	db    *gorm.DB
	now   func() time.Time
}

type ProviderSyncStateJournalOption func(*ProviderSyncStateJournalStore)

func WithSyncStateAuthorizationFence() ProviderSyncStateJournalOption {
	return func(s *ProviderSyncStateJournalStore) { s.fence = true }
}

func NewProviderSyncStateJournalStore(
	store *Store,
	opts ...ProviderSyncStateJournalOption,
) *ProviderSyncStateJournalStore {
	s := &ProviderSyncStateJournalStore{
		db:  store.db,
		now: store.now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	if !s.fence {
		return s.appendState(ctx, s.db, state)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		connection, err := lockBankConnection(ctx, tx, state.Connection.ConnectionID)
		if err != nil {
			return err
		}
		if connection.ProviderReference != state.Connection.ProviderReference {
			return domain.ErrBankAuthorizationStale
		}
		if connection.State != domain.BankConnectionStateActive {
			return domain.ErrBankReauthRequired
		}
		return s.appendState(ctx, tx, state)
	})
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

func (s *ProviderSyncStateJournalStore) appendState(
	ctx context.Context,
	tx *gorm.DB,
	state domain.ProviderSyncState,
) error {
	model := newProviderSyncStateJournalModel(state, s.now())
	if err := tx.WithContext(ctx).Table(model.TableName()).Create(&model).Error; err != nil {
		return fmt.Errorf("append provider sync state journal: %w", err)
	}
	return nil
}
