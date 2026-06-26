package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"gorm.io/gorm/clause"
)

type ListFXRatesParams struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
	StartDate     time.Time
	EndDate       time.Time
}

func (s *Store) SaveFXRates(ctx context.Context, rates []domain.FXRate) error {
	for _, rate := range rates {
		model := newFXRateModel(rate)
		if model.CreatedAt.IsZero() {
			model.CreatedAt = s.now().UTC()
		}
		if model.UpdatedAt.IsZero() {
			model.UpdatedAt = s.now().UTC()
		}
		err := s.db.WithContext(ctx).
			Table(model.TableName()).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: columnProvider},
					{Name: "base_currency"},
					{Name: "quote_currency"},
					{Name: "rate_date"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"rate_value",
					columnUpdatedAt,
				}),
			}).
			Create(&model).Error
		if err != nil {
			return fmt.Errorf("save fx rates: %w", err)
		}
	}
	return nil
}

func (s *Store) ListFXRates(
	ctx context.Context,
	params ListFXRatesParams,
) ([]domain.FXRate, error) {
	var models []fxRateModel
	query := s.db.WithContext(ctx).Table((fxRateModel{}).TableName())
	if provider := strings.TrimSpace(params.Provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if baseCurrency := strings.TrimSpace(params.BaseCurrency); baseCurrency != "" {
		query = query.Where("base_currency = ?", baseCurrency)
	}
	if quoteCurrency := strings.TrimSpace(params.QuoteCurrency); quoteCurrency != "" {
		query = query.Where("quote_currency = ?", quoteCurrency)
	}
	if !params.StartDate.IsZero() {
		query = query.Where("rate_date >= ?", params.StartDate.UTC())
	}
	if !params.EndDate.IsZero() {
		query = query.Where("rate_date <= ?", params.EndDate.UTC())
	}
	if err := query.Order(
		"rate_date ASC, provider ASC, base_currency ASC, quote_currency ASC",
	).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list fx rates: %w", err)
	}
	items := make([]domain.FXRate, 0, len(models))
	for _, model := range models {
		items = append(items, fxRateFromModel(model))
	}
	return items, nil
}
