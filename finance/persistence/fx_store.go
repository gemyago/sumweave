package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"gorm.io/gorm/clause"
)

type ListCurrentFXRatesParams struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
}

// ListFXRatesParams remains an in-process compatibility input. Date filters are intentionally ignored.
type ListFXRatesParams struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
	StartDate     time.Time
	EndDate       time.Time
}

type CurrentFXRateStore struct {
	db  *Database
	now func() time.Time
}

func NewCurrentFXRateStore(database *Database) *CurrentFXRateStore {
	return &CurrentFXRateStore{db: database, now: time.Now}
}

func NewCurrentFXRateStoreFromStore(store *Store) *CurrentFXRateStore {
	return &CurrentFXRateStore{db: &Database{db: store.db}, now: store.now}
}

func (s *CurrentFXRateStore) SaveCurrentFXRates(ctx context.Context, rates []domain.FXRate) error {
	for index := range rates {
		if rates[index].EffectiveAt.IsZero() {
			rates[index].EffectiveAt = rates[index].RateDate
		}
		if rates[index].LastSuccessfulRefreshAt.IsZero() {
			rates[index].LastSuccessfulRefreshAt = rates[index].EffectiveAt
		}
		if rates[index].EffectiveAt.IsZero() || rates[index].LastSuccessfulRefreshAt.IsZero() {
			return errors.New("save current fx rates: effective and refresh timestamps are required")
		}
	}
	for _, rate := range rates {
		model := newCurrentFXRateModel(rate)
		if model.CreatedAt.IsZero() {
			model.CreatedAt = s.now()
		}
		if model.UpdatedAt.IsZero() {
			model.UpdatedAt = s.now()
		}
		if err := s.db.db.WithContext(ctx).Table(model.TableName()).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: columnProvider}, {Name: "base_currency"}, {Name: "quote_currency"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"effective_at", "last_successful_refresh_at", "rate_value", columnUpdatedAt,
			}),
		}).Create(&model).Error; err != nil {
			return fmt.Errorf("save current fx rates: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveFXRates(ctx context.Context, rates []domain.FXRate) error {
	return NewCurrentFXRateStoreFromStore(s).SaveCurrentFXRates(ctx, rates)
}

func (s *Store) SaveCurrentFXRates(ctx context.Context, rates []domain.FXRate) error {
	return NewCurrentFXRateStoreFromStore(s).SaveCurrentFXRates(ctx, rates)
}

func (s *Store) ListFXRates(ctx context.Context, params ListFXRatesParams) ([]domain.FXRate, error) {
	return NewCurrentFXRateStoreFromStore(s).ListCurrentFXRates(ctx, ListCurrentFXRatesParams{
		Provider: params.Provider, BaseCurrency: params.BaseCurrency, QuoteCurrency: params.QuoteCurrency,
	})
}

func (s *Store) ListCurrentFXRates(ctx context.Context, params ListCurrentFXRatesParams) ([]domain.FXRate, error) {
	return NewCurrentFXRateStoreFromStore(s).ListCurrentFXRates(ctx, params)
}

func (s *CurrentFXRateStore) ListCurrentFXRates(
	ctx context.Context,
	params ListCurrentFXRatesParams,
) ([]domain.FXRate, error) {
	var models []currentFXRateModel
	query := s.db.db.WithContext(ctx).Table((currentFXRateModel{}).TableName())
	if provider := strings.TrimSpace(params.Provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if baseCurrency := strings.TrimSpace(params.BaseCurrency); baseCurrency != "" {
		query = query.Where("base_currency = ?", baseCurrency)
	}
	if quoteCurrency := strings.TrimSpace(params.QuoteCurrency); quoteCurrency != "" {
		query = query.Where("quote_currency = ?", quoteCurrency)
	}
	if err := query.Order("provider ASC, base_currency ASC, quote_currency ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list current fx rates: %w", err)
	}
	items := make([]domain.FXRate, 0, len(models))
	for _, model := range models {
		items = append(items, currentFXRateFromModel(model))
	}
	return items, nil
}

type RequiredFXPair struct {
	BaseCurrency  string
	QuoteCurrency string
}

type FXPairDiscoveryStore struct{ db *Database }

func NewFXPairDiscoveryStore(database *Database) *FXPairDiscoveryStore {
	return &FXPairDiscoveryStore{db: database}
}

func (s *FXPairDiscoveryStore) ListRequiredFXPairs(ctx context.Context) ([]RequiredFXPair, error) {
	type row struct {
		BaseCurrency  string `gorm:"column:base_currency"`
		QuoteCurrency string `gorm:"column:quote_currency"`
	}
	rows := make([]row, 0)
	for _, table := range []string{"finance_accounts", "finance_transactions"} {
		var tableRows []row
		if err := s.db.db.WithContext(ctx).Table(table + " item").
			Select("item.currency AS base_currency, tenant.display_currency AS quote_currency").
			Joins("JOIN finance_tenants tenant ON tenant.id = item.tenant_id").
			Where("tenant.archived_at IS NULL").
			Where("item.currency <> tenant.display_currency").
			Scan(&tableRows).Error; err != nil {
			return nil, fmt.Errorf("list required fx pairs: %w", err)
		}
		rows = append(rows, tableRows...)
	}
	pairsByKey := make(map[string]RequiredFXPair, len(rows))
	for _, row := range rows {
		baseCurrency := strings.ToUpper(strings.TrimSpace(row.BaseCurrency))
		quoteCurrency := strings.ToUpper(strings.TrimSpace(row.QuoteCurrency))
		if baseCurrency == "" || quoteCurrency == "" || baseCurrency == quoteCurrency {
			continue
		}
		pairsByKey[baseCurrency+"|"+quoteCurrency] = RequiredFXPair{
			BaseCurrency: baseCurrency, QuoteCurrency: quoteCurrency,
		}
	}
	pairs := make([]RequiredFXPair, 0, len(pairsByKey))
	for _, pair := range pairsByKey {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].QuoteCurrency != pairs[j].QuoteCurrency {
			return pairs[i].QuoteCurrency < pairs[j].QuoteCurrency
		}
		return pairs[i].BaseCurrency < pairs[j].BaseCurrency
	})
	return pairs, nil
}
