package data

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRawPayloadBrowserQueries(t *testing.T) {
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
	timePointer := func(value time.Time) *time.Time { return &value }

	t.Run("NewRawPayloadMetadataListQuery", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes optional filters with decoded cursor and default limit", func(t *testing.T) {
			t.Parallel()

			start := randomTime()
			end := start.Add(5 * time.Minute)
			cursorTime := randomTime().UTC()
			cursorID := randomWord("payload")

			query, err := NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:          domain.Venue("  " + randomWord("venue") + "  "),
				Symbol:         domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
				AssetClass:     domain.AssetClass("  CRYPTO  "),
				Timeframe:      domain.Timeframe(" 1M "),
				StartAt:        timePointer(start),
				EndAt:          timePointer(end),
				IngestionRunID: "  " + randomWord("run") + "  ",
				EntityHint:     "  " + randomWord("entity") + "  ",
				Endpoint:       "  /info  ",
				RequestType:    "  " + randomWord("request") + "  ",
				Cursor:         encodeRawPayloadListCursor(cursorTime, cursorID),
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(query.Venue.String()), query.Venue.String())
			require.NotNil(t, query.Instrument)
			require.Equal(t, strings.TrimSpace(query.Instrument.Symbol.String()), query.Instrument.Symbol.String())
			require.Equal(t, domain.AssetClassCrypto, query.Instrument.AssetClass)
			require.Equal(t, domain.Timeframe1m, query.Timeframe)
			require.NotNil(t, query.TimeRange)
			require.Equal(t, start, query.TimeRange.Start)
			require.Equal(t, end, query.TimeRange.End)
			require.Equal(t, strings.TrimSpace(query.IngestionRunID), query.IngestionRunID)
			require.Equal(t, strings.TrimSpace(query.EntityHint), query.EntityHint)
			require.Equal(t, "/info", query.Endpoint)
			require.Equal(t, strings.TrimSpace(query.RequestType), query.RequestType)
			require.Equal(t, defaultRawPayloadMetadataLimit, query.Limit)
			require.True(t, cursorTime.Equal(query.cursor.ReceivedAt))
			require.Equal(t, cursorID, query.cursor.ID)
		})

		t.Run("caps limit and rejects invalid inputs with deterministic errors", func(t *testing.T) {
			t.Parallel()

			query, err := NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue: domain.Venue(randomWord("venue")),
				Limit: maxRawPayloadMetadataLimit + fake.IntBetween(1, 100),
			})
			require.NoError(t, err)
			require.Equal(t, maxRawPayloadMetadataLimit, query.Limit)

			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: raw payload query venue is required")

			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: "",
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: raw payload query instrument symbol and asset class must both be provided",
			)

			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:     domain.Venue(randomWord("venue")),
				Timeframe: domain.Timeframe(randomWord("timeframe")),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: raw payload query timeframe is invalid")

			zero := time.Time{}
			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:   domain.Venue(randomWord("venue")),
				StartAt: &zero,
			})
			require.ErrorIs(t, err, ErrValidation)
			require.ErrorContains(t, err, "time range start is required")

			start := randomTime()
			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:   domain.Venue(randomWord("venue")),
				StartAt: timePointer(start),
				EndAt:   timePointer(start),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: raw payload query time range end must be after start",
			)

			_, err = NewRawPayloadMetadataListQuery(RawPayloadMetadataListQueryParams{
				Venue:  domain.Venue(randomWord("venue")),
				Cursor: randomWord("cursor"),
			})
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(t, err, "data validation failed: raw payload query cursor is invalid")
		})
	})

	t.Run("NewCandleLinkedRawPayloadsQuery", func(t *testing.T) {
		t.Parallel()

		t.Run("canonicalizes exact candle natural key", func(t *testing.T) {
			t.Parallel()

			start := randomTime()
			end := start.Add(time.Minute)

			query, err := NewCandleLinkedRawPayloadsQuery(CandleLinkedRawPayloadsQueryParams{
				Venue:              domain.Venue("  " + randomWord("venue") + "  "),
				Symbol:             domain.Symbol("  " + strings.ToUpper(randomWord("symbol")) + "  "),
				AssetClass:         domain.AssetClass("  CRYPTO  "),
				Timeframe:          domain.Timeframe(" 1M "),
				StartAt:            start,
				EndAt:              end,
				ProvenanceSource:   "  " + randomWord("source") + "  ",
				ProvenanceIdentity: "  " + randomWord("identity") + "  ",
			})
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(query.Venue.String()), query.Venue.String())
			require.Equal(t, strings.TrimSpace(query.Symbol.String()), query.Symbol.String())
			require.Equal(t, domain.AssetClassCrypto, query.AssetClass)
			require.Equal(t, domain.Timeframe1m, query.Timeframe)
			require.Equal(t, start, query.TimeRange.Start)
			require.Equal(t, end, query.TimeRange.End)
			require.Equal(t, strings.TrimSpace(query.ProvenanceSource), query.ProvenanceSource)
			require.Equal(t, strings.TrimSpace(query.ProvenanceIdentity), query.ProvenanceIdentity)
		})

		t.Run("requires provenance fields deterministically", func(t *testing.T) {
			t.Parallel()

			start := randomTime()
			base := CandleLinkedRawPayloadsQueryParams{
				Venue:      domain.Venue(randomWord("venue")),
				Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
				AssetClass: domain.AssetClassCrypto,
				Timeframe:  domain.Timeframe1m,
				StartAt:    start,
				EndAt:      start.Add(time.Minute),
			}

			_, err := NewCandleLinkedRawPayloadsQuery(base)
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: candle raw payload query provenance source is required",
			)

			base.ProvenanceSource = randomWord("source")
			_, err = NewCandleLinkedRawPayloadsQuery(base)
			require.ErrorIs(t, err, ErrValidation)
			require.EqualError(
				t,
				err,
				"data validation failed: candle raw payload query provenance identity is required",
			)
		})
	})
}
