package finance

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCashFlowSeriesContracts(t *testing.T) {
	makeParams := func(fake faker.Faker) CashFlowSeriesParams {
		start := time.Date(2026, time.July, fake.IntBetween(1, 10), 9, 30, 0, 0, time.FixedZone("series", 2*60*60))
		return CashFlowSeriesParams{
			ActorUserID: "actor-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			StartDate:   start,
			EndDate:     start.AddDate(0, 0, 1),
			GroupBy:     CashFlowGroupByDay,
		}
	}

	t.Run("validates required increasing ranges and supported bounded groups", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		require.NoError(t, ValidateCashFlowSeriesParams(params))

		params.StartDate = time.Time{}
		require.ErrorIs(t, ValidateCashFlowSeriesParams(params), ErrInvalidTimestampRange)

		params = makeParams(fake)
		params.EndDate = params.StartDate
		require.ErrorIs(t, ValidateCashFlowSeriesParams(params), ErrInvalidTimestampRange)

		params = makeParams(fake)
		params.GroupBy = CashFlowGroupBy("unsupported-" + fake.Lorem().Word())
		require.Error(t, ValidateCashFlowSeriesParams(params))

		params = makeParams(fake)
		params.EndDate = params.StartDate.AddDate(0, 0, 367)
		require.Error(t, ValidateCashFlowSeriesParams(params))

		monthStart := time.Date(2026, time.January, 31, 9, 30, 0, 0, time.FixedZone("monthly", 2*60*60))
		assert.Equal(t, 3, cashFlowBucketCount(monthStart, monthStart.AddDate(0, 2, 1), CashFlowGroupByMonth))
	})

	t.Run("authorizes the tenant before delegating a valid request", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		access := newMockreportingServiceStore(t)
		cashFlows := newMockcashFlowSeriesStore(t)
		access.EXPECT().IsTenantMember(mock.Anything, params.TenantID, params.ActorUserID).Return(true, nil).Once()
		expected := domain.CashFlowSeries{Complete: true}
		cashFlows.EXPECT().GetCashFlowSeries(mock.Anything, persistence.CashFlowSeriesParams{
			TenantID: params.TenantID, StartDate: params.StartDate, EndDate: params.EndDate,
			GroupBy: CashFlowGroupByDay, FXProvider: FXProviderFrankfurter,
		}).Return(expected, nil).Once()

		actual, err := NewReportingService(
			access,
			cashFlows,
			WithReportingServiceAccountBalanceStore(newMockaccountBalanceReadStore(t)),
		).GetCashFlowSeries(t.Context(), params)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("rejects a non-member without querying the series store", func(t *testing.T) {
		fake := faker.New()
		params := makeParams(fake)
		access := newMockreportingServiceStore(t)
		access.EXPECT().IsTenantMember(mock.Anything, params.TenantID, params.ActorUserID).Return(false, nil).Once()

		_, err := NewReportingService(
			access,
			newMockcashFlowSeriesStore(t),
			WithReportingServiceAccountBalanceStore(newMockaccountBalanceReadStore(t)),
		).GetCashFlowSeries(t.Context(), params)
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})
}
