package data

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestCandleAvailabilityQueries(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomTime := func() time.Time {
		return time.Date(
			fake.IntBetween(2021, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	t.Run("NewCandleAvailabilityListQuery", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes optional exact filters with default limit and decoded cursor", func(t *testing.T) {
			t.Parallel()

			cursorTime := randomTime().UTC()
			query, err := NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{
				Venue:      domain.Venue("  " + randomWord("venue") + "  "),
				Symbol:     domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
				AssetClass: domain.AssetClass("  CRYPTO  "),
				Cursor: encodeCandleAvailabilityListCursor(
					cursorTime,
					domain.Venue(randomWord("cursor-venue")),
					domain.Symbol(strings.ToUpper(randomWord("cursor-symbol"))),
					domain.AssetClassCrypto,
				),
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(query.Venue.String()), query.Venue.String())
			require.Equal(t, strings.TrimSpace(query.Symbol.String()), query.Symbol.String())
			require.Equal(t, domain.AssetClassCrypto, query.AssetClass)
			require.Equal(t, defaultCandleAvailabilityLimit, query.Limit)
			require.True(t, cursorTime.Equal(query.cursor.LatestEnd))
			require.NotEmpty(t, query.Cursor)
		})

		t.Run("accepts explicit limit and empty optional filters", func(t *testing.T) {
			t.Parallel()

			query, err := NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{Limit: 1})
			require.NoError(t, err)
			require.Equal(t, 1, query.Limit)
			require.Zero(t, query.Venue)
			require.Zero(t, query.Symbol)
			require.Zero(t, query.AssetClass)
		})

		t.Run("rejects invalid limits asset class and cursor deterministically", func(t *testing.T) {
			t.Parallel()

			_, err := NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{
				AssetClass: domain.AssetClass(randomWord("asset-class")),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: candle availability query asset class is invalid")

			_, err = NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{Limit: -1})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: candle availability query limit must be between 1 and 200",
			)

			_, err = NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{
				Limit: maxCandleAvailabilityLimit + fake.IntBetween(1, 100),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: candle availability query limit must be between 1 and 200",
			)

			_, err = NewCandleAvailabilityListQuery(CandleAvailabilityListQueryParams{
				Cursor: randomWord("cursor"),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: candle availability query cursor is invalid")
		})

		t.Run("supports only known timeframe metadata for default slices", func(t *testing.T) {
			t.Parallel()

			_, err := candleAvailabilityTimeframeDuration(domain.Timeframe(randomWord("timeframe")))
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: candle availability timeframe is invalid")
		})
	})
}
