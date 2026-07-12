package persistence

import (
	"context"
	"errors"
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
		if rate.RateDate.IsZero() {
			return errors.New("save fx rates: rate timestamp is required")
		}
	}
	for _, rate := range rates {
		model := newFXRateModel(rate)
		if model.CreatedAt.IsZero() {
			model.CreatedAt = s.now()
		}
		if model.UpdatedAt.IsZero() {
			model.UpdatedAt = s.now()
		}
		err := s.db.WithContext(ctx).
			Table(model.TableName()).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: columnProvider},
					{Name: "base_currency"},
					{Name: "quote_currency"},
					{Name: "rate_at"},
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
	if !params.StartDate.IsZero() && !params.EndDate.IsZero() && params.StartDate.After(params.EndDate) {
		return nil, errors.New("list fx rates: start date must not be after end date")
	}
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
		query = applyInstantAtOrAfter(query, "rate_at", params.StartDate)
	}
	if !params.EndDate.IsZero() {
		query = applyInstantAtOrBefore(query, "rate_at", params.EndDate)
	}
	if err := query.Order(
		"rate_at ASC, provider ASC, base_currency ASC, quote_currency ASC",
	).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list fx rates: %w", err)
	}
	items := make([]domain.FXRate, 0, len(models))
	for _, model := range models {
		items = append(items, fxRateFromModel(model))
	}
	return items, nil
}
