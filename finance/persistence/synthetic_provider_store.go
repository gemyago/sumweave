package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSyntheticProviderStateNotFound = errors.New("synthetic provider state not found")

type SyntheticProviderStateStore struct {
	db *gorm.DB
}

func NewSyntheticProviderStateStore(database *Database) *SyntheticProviderStateStore {
	return &SyntheticProviderStateStore{db: database.db}
}

func NewSyntheticProviderStateStoreFromStore(store *Store) *SyntheticProviderStateStore {
	if store == nil {
		return nil
	}
	return &SyntheticProviderStateStore{db: store.db}
}

func (s *SyntheticProviderStateStore) SaveSyntheticProviderState(
	ctx context.Context,
	state domain.SyntheticProviderState,
) (domain.SyntheticProviderState, error) {
	model := newSyntheticProviderStateModel(state)
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: columnProviderReference}},
			DoUpdates: clause.AssignmentColumns([]string{
				"state_json",
				columnUpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		return domain.SyntheticProviderState{}, fmt.Errorf("save synthetic provider state: %w", err)
	}
	return syntheticProviderStateFromModel(model), nil
}

func (s *SyntheticProviderStateStore) GetSyntheticProviderState(
	ctx context.Context,
	providerReference string,
) (*domain.SyntheticProviderState, error) {
	var model syntheticProviderStateModel
	if err := s.db.WithContext(ctx).
		Table(model.TableName()).
		Where("provider_reference = ?", strings.TrimSpace(providerReference)).
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSyntheticProviderStateNotFound
		}
		return nil, fmt.Errorf("get synthetic provider state: %w", err)
	}
	state := syntheticProviderStateFromModel(model)
	return &state, nil
}

func (s *SyntheticProviderStateStore) DeleteSyntheticProviderState(
	ctx context.Context,
	providerReference string,
) error {
	if err := s.db.WithContext(ctx).
		Table((syntheticProviderStateModel{}).TableName()).
		Where("provider_reference = ?", strings.TrimSpace(providerReference)).
		Delete(&syntheticProviderStateModel{}).Error; err != nil {
		return fmt.Errorf("delete synthetic provider state: %w", err)
	}
	return nil
}
