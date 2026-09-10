package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
)

type CashFlowGroupBy = domain.CashFlowGroupBy

const (
	CashFlowGroupByDay   = domain.CashFlowGroupByDay
	CashFlowGroupByMonth = domain.CashFlowGroupByMonth
)

type CashFlowSeriesBucket = domain.CashFlowSeriesBucket

type CashFlowSeriesParams struct {
	TenantID   string
	StartDate  time.Time
	EndDate    time.Time
	GroupBy    CashFlowGroupBy
	FXProvider string
}

type CashFlowSeriesStore struct{ db *Database }

func NewCashFlowSeriesStore(database *Database) *CashFlowSeriesStore {
	return &CashFlowSeriesStore{db: database}
}

func NewCashFlowSeriesStoreFromStore(store *Store) *CashFlowSeriesStore {
	return &CashFlowSeriesStore{db: &Database{db: store.db}}
}

type cashFlowSeriesRow struct {
	RowKind                  string    `gorm:"column:row_kind"`
	DisplayCurrency          string    `gorm:"column:display_currency"`
	BucketStart              time.Time `gorm:"column:bucket_start"`
	BucketEnd                time.Time `gorm:"column:bucket_end"`
	IncomeMinor              int64     `gorm:"column:income_minor"`
	ExpenseMinor             int64     `gorm:"column:expense_minor"`
	Provider                 string    `gorm:"column:provider"`
	BaseCurrency             string    `gorm:"column:base_currency"`
	QuoteCurrency            string    `gorm:"column:quote_currency"`
	AffectedTransactionCount int       `gorm:"column:affected_transaction_count"`
}

func (s *CashFlowSeriesStore) GetCashFlowSeries(
	ctx context.Context,
	params CashFlowSeriesParams,
) (domain.CashFlowSeries, error) {
	query, err := cashFlowSeriesQuery(params.GroupBy)
	if err != nil {
		return domain.CashFlowSeries{}, err
	}
	rows := make([]cashFlowSeriesRow, 0)
	if queryErr := s.db.db.WithContext(ctx).Raw(
		query,
		params.StartDate,
		params.EndDate,
		params.TenantID,
		params.FXProvider,
	).Scan(&rows).Error; queryErr != nil {
		return domain.CashFlowSeries{}, fmt.Errorf("get cash-flow series: %w", queryErr)
	}
	series := domain.CashFlowSeries{
		Period:    domain.CashFlowPeriod{StartDate: params.StartDate, EndDate: params.EndDate},
		GroupBy:   params.GroupBy,
		Complete:  true,
		MissingFX: make([]domain.CashFlowMissingFXDiagnostic, 0),
		Buckets:   make([]domain.CashFlowSeriesBucket, 0),
	}
	for _, row := range rows {
		series.DisplayCurrency = row.DisplayCurrency
		if row.RowKind == "bucket" {
			series.Buckets = append(series.Buckets, domain.CashFlowSeriesBucket{
				StartDate: row.BucketStart, EndDate: row.BucketEnd,
				IncomeMinor: row.IncomeMinor, ExpenseMinor: row.ExpenseMinor,
			})
			continue
		}
		series.Complete = false
		series.MissingFX = append(series.MissingFX, domain.CashFlowMissingFXDiagnostic{
			Provider: row.Provider, BaseCurrency: row.BaseCurrency, QuoteCurrency: row.QuoteCurrency,
			AffectedTransactionCount: row.AffectedTransactionCount,
		})
	}
	return series, nil
}

func cashFlowSeriesQuery(groupBy CashFlowGroupBy) (string, error) {
	var interval string
	switch groupBy {
	case CashFlowGroupByDay:
		interval = "1 day"
	case CashFlowGroupByMonth:
		interval = "1 month"
	default:
		return "", fmt.Errorf("unsupported cash-flow grouping %q", groupBy)
	}
	return fmt.Sprintf(cashFlowSeriesQueryTemplate, interval, interval, interval), nil
}

const cashFlowSeriesQueryTemplate = `
WITH RECURSIVE request AS (
    SELECT ?::timestamptz AS start_date, ?::timestamptz AS end_date,
           ?::text AS tenant_id, ?::text AS fx_provider
), tenant AS (
    SELECT item.id AS tenant_id, item.display_currency, request.fx_provider
    FROM finance_tenants item
    JOIN request ON request.tenant_id = item.id
), bucket_indices AS (
    SELECT 0 AS bucket_index
    UNION ALL
    SELECT bucket_indices.bucket_index + 1
    FROM bucket_indices
    CROSS JOIN request
    WHERE request.start_date + (bucket_indices.bucket_index + 1) * interval '%s' < request.end_date
), buckets AS (
    SELECT tenant.tenant_id, tenant.display_currency, tenant.fx_provider,
           request.start_date + bucket_indices.bucket_index * interval '%s' AS bucket_start,
           LEAST(request.start_date + (bucket_indices.bucket_index + 1) * interval '%s', request.end_date) AS bucket_end
    FROM tenant
    CROSS JOIN request
    CROSS JOIN bucket_indices
), qualifying AS (
    SELECT buckets.tenant_id, buckets.display_currency, buckets.fx_provider,
           buckets.bucket_start, buckets.bucket_end,
           transaction_item.id AS transaction_id, transaction_item.currency,
           transaction_item.amount_minor, transaction_item.kind
    FROM buckets
    JOIN finance_transactions transaction_item
      ON transaction_item.tenant_id = buckets.tenant_id
     AND transaction_item.effective_at >= buckets.bucket_start
     AND transaction_item.effective_at < buckets.bucket_end
    JOIN finance_accounts account_item
      ON account_item.id = transaction_item.account_id
     AND account_item.tenant_id = buckets.tenant_id
    WHERE transaction_item.status = 'booked'
      AND transaction_item.hidden_at IS NULL
      AND account_item.hidden_at IS NULL
      AND transaction_item.amount_minor <> 0
      AND transaction_item.kind IN ('regular', 'expense', 'income', 'refund', 'transfer')
      AND NOT (
          transaction_item.kind = 'transfer'
          AND transaction_item.transfer_matched_at IS NOT NULL
          AND transaction_item.amount_minor <> 0
      )
), contributions AS (
    SELECT qualifying.tenant_id, qualifying.display_currency, qualifying.fx_provider,
           qualifying.bucket_start, qualifying.bucket_end, qualifying.transaction_id,
           qualifying.currency, qualifying.amount_minor, qualifying.kind,
           CASE
             WHEN qualifying.kind = 'refund' THEN 0
             WHEN qualifying.amount_minor > 0 THEN qualifying.amount_minor
             ELSE 0
           END AS income_native_minor,
           CASE
             WHEN qualifying.kind = 'refund' THEN -qualifying.amount_minor
             WHEN qualifying.amount_minor < 0 THEN -qualifying.amount_minor
             ELSE 0
           END AS expense_native_minor,
           current_rate.provider AS matched_provider,
           current_rate.rate_value
    FROM qualifying
    LEFT JOIN finance_current_fx_rates current_rate
      ON current_rate.provider = qualifying.fx_provider
     AND current_rate.base_currency = qualifying.currency
     AND current_rate.quote_currency = qualifying.display_currency
), bucket_rows AS (
    SELECT 'bucket'::text AS row_kind, buckets.display_currency,
           buckets.bucket_start, buckets.bucket_end,
           COALESCE(SUM(CASE
             WHEN contributions.currency = contributions.display_currency THEN contributions.income_native_minor
             WHEN contributions.matched_provider IS NOT NULL THEN ROUND(contributions.income_native_minor * contributions.rate_value)::bigint
             ELSE 0
           END), 0)::bigint AS income_minor,
           COALESCE(SUM(CASE
             WHEN contributions.currency = contributions.display_currency THEN contributions.expense_native_minor
             WHEN contributions.matched_provider IS NOT NULL THEN ROUND(contributions.expense_native_minor * contributions.rate_value)::bigint
             ELSE 0
           END), 0)::bigint AS expense_minor,
           ''::text AS provider, ''::text AS base_currency, ''::text AS quote_currency,
           0::int AS affected_transaction_count
    FROM buckets
    LEFT JOIN contributions
      ON contributions.tenant_id = buckets.tenant_id
     AND contributions.bucket_start = buckets.bucket_start
    GROUP BY buckets.display_currency, buckets.bucket_start, buckets.bucket_end
), missing_rows AS (
    SELECT 'missing'::text AS row_kind, qualifying.display_currency,
           MIN(qualifying.bucket_start) AS bucket_start, MIN(qualifying.bucket_end) AS bucket_end,
           0::bigint AS income_minor, 0::bigint AS expense_minor,
           qualifying.fx_provider AS provider, qualifying.currency AS base_currency,
           qualifying.display_currency AS quote_currency,
           COUNT(DISTINCT qualifying.transaction_id)::int AS affected_transaction_count
    FROM qualifying
    LEFT JOIN finance_current_fx_rates current_rate
      ON current_rate.provider = qualifying.fx_provider
     AND current_rate.base_currency = qualifying.currency
     AND current_rate.quote_currency = qualifying.display_currency
    WHERE qualifying.currency <> qualifying.display_currency
      AND current_rate.provider IS NULL
    GROUP BY qualifying.display_currency, qualifying.fx_provider, qualifying.currency
)
SELECT row_kind, display_currency, bucket_start, bucket_end, income_minor, expense_minor,
       provider, base_currency, quote_currency, affected_transaction_count
FROM bucket_rows
UNION ALL
SELECT row_kind, display_currency, bucket_start, bucket_end, income_minor, expense_minor,
       provider, base_currency, quote_currency, affected_transaction_count
FROM missing_rows
ORDER BY row_kind, bucket_start, provider, base_currency, quote_currency
`
