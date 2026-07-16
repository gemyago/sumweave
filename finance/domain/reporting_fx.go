package domain

import "time"

type FXRate struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
	EffectiveAt   time.Time
	// RateDate is accepted only by legacy in-process callers; persistence and reporting use EffectiveAt.
	RateDate                time.Time
	LastSuccessfulRefreshAt time.Time
	Rate                    float64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}
