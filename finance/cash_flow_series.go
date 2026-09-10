package finance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

const maxCashFlowBuckets = 366

var ErrInvalidCashFlowGrouping = errors.New("invalid cash-flow grouping")

type CashFlowGroupBy = domain.CashFlowGroupBy

const (
	CashFlowGroupByDay   = domain.CashFlowGroupByDay
	CashFlowGroupByMonth = domain.CashFlowGroupByMonth
)

type CashFlowPeriod = domain.CashFlowPeriod
type CashFlowSeriesBucket = domain.CashFlowSeriesBucket
type CashFlowMissingFXDiagnostic = domain.CashFlowMissingFXDiagnostic
type CashFlowSeries = domain.CashFlowSeries

type CashFlowSeriesParams struct {
	ActorUserID string
	TenantID    string
	StartDate   time.Time
	EndDate     time.Time
	GroupBy     CashFlowGroupBy
}

type cashFlowSeriesStore interface {
	GetCashFlowSeries(ctx context.Context, params persistence.CashFlowSeriesParams) (domain.CashFlowSeries, error)
}

func ValidateCashFlowSeriesParams(params CashFlowSeriesParams) error {
	if err := ValidateRequiredTimestampRange(params.StartDate, params.EndDate); err != nil {
		return err
	}
	if !params.StartDate.Before(params.EndDate) {
		return fmt.Errorf("%w: start timestamp must be before end timestamp", ErrInvalidTimestampRange)
	}
	if err := validateCashFlowGroupBy(params.GroupBy); err != nil {
		return err
	}
	if cashFlowBucketCount(params.StartDate, params.EndDate, params.GroupBy) > maxCashFlowBuckets {
		return fmt.Errorf("cash-flow series may contain at most %d buckets", maxCashFlowBuckets)
	}
	return nil
}

func validateCashFlowGroupBy(groupBy CashFlowGroupBy) error {
	switch groupBy {
	case CashFlowGroupByDay, CashFlowGroupByMonth:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidCashFlowGrouping, groupBy)
	}
}

func cashFlowBucketCount(startDate time.Time, endDate time.Time, groupBy CashFlowGroupBy) int {
	count := 0
	for boundary := startDate; boundary.Before(endDate); count++ {
		switch groupBy {
		case CashFlowGroupByDay:
			boundary = boundary.AddDate(0, 0, 1)
		case CashFlowGroupByMonth:
			boundary = addCashFlowMonth(boundary)
		}
	}
	return count
}

func addCashFlowMonth(boundary time.Time) time.Time {
	firstOfNextMonth := time.Date(
		boundary.Year(),
		boundary.Month()+1,
		1,
		boundary.Hour(),
		boundary.Minute(),
		boundary.Second(),
		boundary.Nanosecond(),
		boundary.Location(),
	)
	lastOfNextMonth := firstOfNextMonth.AddDate(0, 1, -1).Day()
	day := min(boundary.Day(), lastOfNextMonth)
	return time.Date(
		firstOfNextMonth.Year(),
		firstOfNextMonth.Month(),
		day,
		boundary.Hour(),
		boundary.Minute(),
		boundary.Second(),
		boundary.Nanosecond(),
		boundary.Location(),
	)
}
