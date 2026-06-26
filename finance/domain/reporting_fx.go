package domain

import "time"

type FXRate struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
	RateDate      time.Time
	Rate          float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
