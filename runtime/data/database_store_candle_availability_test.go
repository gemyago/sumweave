package data

import (
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDatabaseStoreCandleAvailability(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	makeStore := func(t *testing.T) *DatabaseStore {
		t.Helper()

		store, err := NewDatabaseStore(":memory:", DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeInstrument := func(t *testing.T, venue, symbol string) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(venue),
			Symbol:     domain.Symbol(symbol),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeCandle := func(
		t *testing.T,
		instrument domain.Instrument,
		timeframe domain.Timeframe,
		start time.Time,
	) domain.Candle {
		t.Helper()

		duration, err := candleAvailabilityTimeframeDuration(timeframe)
		require.NoError(t, err)

		timeRange, err := domain.NewTimeRange(start, start.Add(duration))
		require.NoError(t, err)

		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)

		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange:  timeRange,
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1, 1000),
			Low:        fake.Float64(2, 0, 1000),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(2, 0, 1000),
			Quality:    domain.DataQualityValidated,
			Provenance: provenance,
		})
		require.NoError(t, err)

		return candle
	}

	makeIngestionService := func(t *testing.T, store *DatabaseStore) *IngestionService {
		t.Helper()

		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		return service
	}

	createRawOnlyPayload := func(t *testing.T, store *DatabaseStore, venue, symbol string) {
		t.Helper()

		startedAt := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		run, err := NewIngestionRun(IngestionRunParams{
			ID:          randomWord("run"),
			Source:      randomWord("source"),
			Venue:       domain.Venue(venue),
			Status:      IngestionRunStatusSucceeded,
			StartedAt:   startedAt,
			CompletedAt: startedAt.Add(time.Minute),
			RecordCount: 1,
		})
		require.NoError(t, err)
		_, err = store.UpsertIngestionRun(t.Context(), run)
		require.NoError(t, err)

		timeRange, err := domain.NewTimeRange(startedAt, startedAt.Add(time.Minute))
		require.NoError(t, err)
		payload, err := NewRawVenuePayload(RawVenuePayloadParams{
			ID:                 randomWord("payload"),
			IngestionRunID:     run.ID,
			Source:             run.Source,
			Venue:              run.Venue,
			Endpoint:           "/candles",
			RequestType:        randomWord("request"),
			RequestPayloadHash: randomWord("request-hash"),
			RequestMetadata:    map[string]string{"scope": randomWord("scope")},
			RequestAt:          startedAt,
			ResponseAt:         startedAt.Add(10 * time.Second),
			HTTPStatus:         200,
			ResponseBodyHash:   randomWord("body-hash"),
			PayloadBodyRef:     "payloads/" + randomWord("body-ref"),
			Instrument: &BatchInstrumentRef{
				Symbol:     domain.Symbol(symbol),
				AssetClass: domain.AssetClassCrypto,
			},
			Timeframe:  domain.Timeframe1m,
			TimeRange:  &timeRange,
			ReceivedAt: startedAt.Add(20 * time.Second),
		})
		require.NoError(t, err)
		_, err = store.UpsertRawVenuePayload(t.Context(), payload)
		require.NoError(t, err)
	}

	t.Run("groups persisted candles and excludes raw-only and candle-less entries", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t)
		ingestionService := makeIngestionService(t, store)
		base := time.Date(2024, time.January, 2, 10, 0, 0, 0, time.UTC)

		browseable := makeInstrument(t, "venue-browseable", strings.ToUpper(randomWord("symbol")))
		_, err := store.UpsertInstrument(
			t.Context(),
			makeInstrument(t, "venue-empty", strings.ToUpper(randomWord("symbol-empty"))),
		)
		require.NoError(t, err)
		createRawOnlyPayload(t, store, "venue-raw-only", strings.ToUpper(randomWord("symbol-raw")))

		for _, candle := range []domain.Candle{
			makeCandle(t, browseable, domain.Timeframe1m, base),
			makeCandle(t, browseable, domain.Timeframe1m, base.Add(2*time.Minute)),
			makeCandle(t, browseable, domain.Timeframe5m, base),
		} {
			_, err = ingestionService.IngestCandle(t.Context(), candle)
			require.NoError(t, err)
		}

		result, err := store.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{})
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		require.NotNil(t, result.DefaultSelection)

		item := result.Items[0]
		require.Equal(t, browseable.Venue, item.Venue)
		require.Equal(t, browseable.Symbol, item.Symbol)
		require.Equal(t, browseable.AssetClass, item.AssetClass)
		require.Len(t, item.Timeframes, 2)
		require.Equal(t, []domain.Timeframe{domain.Timeframe1m, domain.Timeframe5m}, []domain.Timeframe{
			item.Timeframes[0].Timeframe,
			item.Timeframes[1].Timeframe,
		})
		require.Equal(t, base, item.Timeframes[0].StartAt)
		require.Equal(t, base.Add(3*time.Minute), item.Timeframes[0].EndAt)
		require.EqualValues(t, 2, item.Timeframes[0].Count)
		require.Equal(t, base, item.Timeframes[1].StartAt)
		require.Equal(t, base.Add(5*time.Minute), item.Timeframes[1].EndAt)
		require.EqualValues(t, 1, item.Timeframes[1].Count)
		require.Equal(t, domain.Timeframe5m, item.DefaultSlice.Timeframe)
		require.Equal(t, base, item.DefaultSlice.StartAt)
		require.Equal(t, base.Add(5*time.Minute), item.DefaultSlice.EndAt)

		filtered, err := store.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{
			Venue:      domain.Venue("  venue-browseable  "),
			Symbol:     domain.Symbol("  " + browseable.Symbol.String() + "  "),
			AssetClass: domain.AssetClass("  CRYPTO  "),
			Limit:      1,
		})
		require.NoError(t, err)
		require.Len(t, filtered.Items, 1)
		require.Equal(t, browseable.Symbol, filtered.Items[0].Symbol)
	})

	t.Run("orders paginates and derives deterministic default selections", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t)
		ingestionService := makeIngestionService(t, store)

		newest := makeInstrument(t, "venue-c", strings.ToUpper(randomWord("symbol-c")))
		alpha := makeInstrument(t, "venue-a", strings.ToUpper(randomWord("symbol-a")))
		beta := makeInstrument(t, "venue-b", strings.ToUpper(randomWord("symbol-b")))

		newestLatestStart := time.Date(2024, time.January, 4, 12, 0, 0, 0, time.UTC)
		for _, candle := range []domain.Candle{
			makeCandle(t, newest, domain.Timeframe1m, time.Date(2024, time.January, 3, 0, 0, 0, 0, time.UTC)),
			makeCandle(t, newest, domain.Timeframe1m, newestLatestStart),
			makeCandle(t, newest, domain.Timeframe5m, newestLatestStart.Add(-4*time.Minute)),
			makeCandle(t, alpha, domain.Timeframe1m, time.Date(2024, time.January, 3, 0, 9, 0, 0, time.UTC)),
			makeCandle(t, beta, domain.Timeframe1m, time.Date(2024, time.January, 3, 0, 9, 0, 0, time.UTC)),
		} {
			_, err := ingestionService.IngestCandle(t.Context(), candle)
			require.NoError(t, err)
		}

		firstPage, err := store.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{Limit: 2})
		require.NoError(t, err)
		require.Len(t, firstPage.Items, 2)
		require.NotEmpty(t, firstPage.NextCursor)
		require.NotNil(t, firstPage.DefaultSelection)

		require.Equal(t, newest.Symbol, firstPage.Items[0].Symbol)
		require.Equal(t, alpha.Symbol, firstPage.Items[1].Symbol)
		require.Equal(t, newest.Venue, firstPage.DefaultSelection.Venue)
		require.Equal(t, newest.Symbol, firstPage.DefaultSelection.Symbol)
		require.Equal(t, newest.AssetClass, firstPage.DefaultSelection.AssetClass)
		require.Equal(t, domain.Timeframe1m, firstPage.DefaultSelection.Timeframe)
		require.Equal(t, newestLatestStart.Add(time.Minute), firstPage.DefaultSelection.EndAt)
		require.Equal(
			t,
			firstPage.DefaultSelection.EndAt.Add(-500*time.Minute),
			firstPage.DefaultSelection.StartAt,
		)

		secondPage, err := store.ListCandleAvailability(t.Context(), CandleAvailabilityListQuery{
			Limit:  2,
			Cursor: firstPage.NextCursor,
		})
		require.NoError(t, err)
		require.Len(t, secondPage.Items, 1)
		require.Nil(t, secondPage.DefaultSelection)
		require.Empty(t, secondPage.NextCursor)
		require.Equal(t, beta.Symbol, secondPage.Items[0].Symbol)
		require.Equal(t, domain.Timeframe1m, firstPage.Items[0].DefaultSlice.Timeframe)
		require.Equal(
			t,
			time.Date(2024, time.January, 3, 0, 9, 0, 0, time.UTC),
			firstPage.Items[1].DefaultSlice.StartAt,
		)
	})
}
