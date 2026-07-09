package data

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type sqliteIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestDatabaseStore(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	makeStore := func(t *testing.T, dsn string, tablePrefix string) *DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewDatabaseStore(sqlDB, dsn, DatabaseStoreOpts{
			TablePrefix: tablePrefix,
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

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

	makeInstrument := func(t *testing.T) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(randomWord("venue")),
			Symbol:     domain.Symbol(strings.ToUpper(randomWord("symbol"))),
			AssetClass: domain.AssetClassCrypto,
			Active:     fake.Bool(),
		})
		require.NoError(t, err)

		return instrument
	}

	makeCandle := func(t *testing.T, instrument domain.Instrument) domain.Candle {
		t.Helper()

		start := randomTime()
		timeRange, err := domain.NewTimeRange(start, start.Add(time.Duration(fake.IntBetween(1, 180))*time.Minute))
		require.NoError(t, err)

		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)

		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  timeRange,
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1, 1000),
			Low:        fake.Float64(2, 0, 1000),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(4, 0, 10000),
			Quality:    domain.DataQualityValidated,
			Provenance: provenance,
		})
		require.NoError(t, err)

		return candle
	}

	makeTrade := func(t *testing.T, instrument domain.Instrument) domain.Trade {
		t.Helper()

		provenance, err := domain.NewSourceProvenance(randomWord("source"), randomWord("record"))
		require.NoError(t, err)

		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  randomTime(),
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQualityRaw,
			Provenance: provenance,
		})
		require.NoError(t, err)

		return trade
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(
			t,
			store.db.WithContext(t.Context()).
				Table(tableName).
				Count(&count).Error,
		)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteIndexListRow
		require.NoError(
			t,
			store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error,
		)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteIndexInfoRow
			require.NoError(
				t,
				store.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error,
			)

			got := make([]string, 0, len(columns))
			for _, column := range columns {
				got = append(got, column.Name)
			}

			if slices.Equal(got, want) {
				return true
			}
		}

		return false
	}

	t.Run("NewDatabaseStore", func(t *testing.T) {
		t.Parallel()

		t.Run("creates a sqlite-backed store", func(t *testing.T) {
			t.Parallel()

			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			store, err := NewDatabaseStore(sqlDB, ":memory:", DatabaseStoreOpts{})
			require.NoError(t, err)
			require.NotNil(t, store)
		})

		t.Run("requires a dsn", func(t *testing.T) {
			t.Parallel()

			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			store, err := NewDatabaseStore(sqlDB, "", DatabaseStoreOpts{})
			require.Error(t, err)
			require.Nil(t, store)
		})

		t.Run("requires a sql database", func(t *testing.T) {
			t.Parallel()

			store, err := NewDatabaseStore(nil, ":memory:", DatabaseStoreOpts{})
			require.Error(t, err)
			require.Nil(t, store)
		})
	})

	t.Run("AutoMigrate", func(t *testing.T) {
		t.Parallel()

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()

		store, err := NewDatabaseStore(sqlDB, ":memory:", DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, store.AutoMigrate())
	})

	t.Run("migration uses explicit table and column names with unique natural-key indexes", func(t *testing.T) {
		t.Parallel()

		tablePrefix := strings.ReplaceAll(randomWord("sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var instrumentColumns []sqliteTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"instruments"),
			).Scan(&instrumentColumns).Error,
		)
		require.Equal(t, []string{
			"id",
			"venue",
			"symbol",
			"asset_class",
			"active",
			"created_at",
			"updated_at",
		}, columnNames(instrumentColumns))

		var candleColumns []sqliteTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"candles"),
			).Scan(&candleColumns).Error,
		)
		require.Equal(t, []string{
			"id",
			"instrument_id",
			"timeframe",
			"start_at",
			"end_at",
			"provenance_source",
			"provenance_identity_key",
			"open",
			"high",
			"low",
			"close",
			"volume",
			"quality",
			"provenance_record_id",
			"data_batch_id",
			"created_at",
			"updated_at",
		}, columnNames(candleColumns))

		var tradeColumns []sqliteTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"trades"),
			).Scan(&tradeColumns).Error,
		)
		require.Equal(t, []string{
			"id",
			"instrument_id",
			"event_time",
			"price",
			"size",
			"quality",
			"provenance_source",
			"provenance_identity_key",
			"provenance_record_id",
			"data_batch_id",
			"created_at",
			"updated_at",
		}, columnNames(tradeColumns))

		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"instruments", []string{"venue", "symbol"}),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"candles", []string{
				"instrument_id",
				"timeframe",
				"start_at",
				"end_at",
				"provenance_source",
				"provenance_identity_key",
			}),
		)
		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"trades", []string{
				"instrument_id",
				"provenance_source",
				"provenance_identity_key",
			}),
		)

		now := time.Now().UTC().Truncate(time.Second)
		require.NoError(
			t,
			store.db.Exec(
				fmt.Sprintf(
					"INSERT INTO %s (venue, symbol, asset_class, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
					`"`+tablePrefix+`instruments"`,
				),
				randomWord("venue"),
				strings.ToUpper(randomWord("symbol")),
				domain.AssetClassCrypto.String(),
				true,
				now,
				now,
			).Error,
		)
	})

	t.Run("instrument upsert updates one deterministic row", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		ctx := t.Context()
		instrument := makeInstrument(t)

		persisted, err := store.UpsertInstrument(ctx, instrument)
		require.NoError(t, err)
		require.Equal(t, instrument, persisted)

		updatedInput, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      instrument.Venue,
			Symbol:     instrument.Symbol,
			AssetClass: domain.AssetClassEquity,
			Active:     !instrument.Active,
		})
		require.NoError(t, err)

		updated, err := store.UpsertInstrument(ctx, updatedInput)
		require.NoError(t, err)
		require.Equal(t, updatedInput, updated)
		require.Equal(t, int64(1), readCount(t, store, "instruments"))

		lookedUp, err := store.LookupInstrument(ctx, instrument.Venue, instrument.Symbol)
		require.NoError(t, err)
		require.NotNil(t, lookedUp)
		require.Equal(t, updatedInput, *lookedUp)
	})

	t.Run("candle ingestion updates one row for the same source record", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		first := makeCandle(t, instrument)
		persistedFirst, err := service.IngestCandle(t.Context(), first)
		require.NoError(t, err)
		require.Equal(t, time.UTC, persistedFirst.TimeRange.Start.Location())
		require.Equal(t, time.UTC, persistedFirst.TimeRange.End.Location())

		second, err := domain.NewCandle(domain.CandleParams{
			Instrument: first.Instrument,
			Timeframe:  first.Timeframe,
			TimeRange:  first.TimeRange,
			Open:       first.Open + 1,
			High:       first.High + 1,
			Low:        first.Low + 1,
			Close:      first.Close + 1,
			Volume:     first.Volume + 1,
			Quality:    domain.DataQualitySuspect,
			Provenance: first.Provenance,
		})
		require.NoError(t, err)

		persistedSecond, err := service.IngestCandle(t.Context(), second)
		require.NoError(t, err)
		require.Equal(t, second, persistedSecond)
		require.Equal(t, int64(1), readCount(t, store, "candles"))

		rangeForQuery, err := domain.NewTimeRange(
			first.TimeRange.Start.Add(-time.Minute),
			first.TimeRange.End.Add(time.Minute),
		)
		require.NoError(t, err)

		candles, err := store.QueryCandles(t.Context(), first.Instrument, first.Timeframe, rangeForQuery)
		require.NoError(t, err)
		require.Len(t, candles, 1)
		require.Equal(t, persistedSecond, candles[0])
		require.Equal(t, domain.DataQualitySuspect, candles[0].Quality)
		require.Equal(t, first.Provenance, candles[0].Provenance)
	})

	t.Run("candle ingestion preserves distinct rows for the same bucket from different sources", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		first := makeCandle(t, instrument)
		persistedFirst, err := service.IngestCandle(t.Context(), first)
		require.NoError(t, err)

		secondProvenance, err := domain.NewSourceProvenance(randomWord("source-two"), randomWord("record-two"))
		require.NoError(t, err)
		second, err := domain.NewCandle(domain.CandleParams{
			Instrument: first.Instrument,
			Timeframe:  first.Timeframe,
			TimeRange:  first.TimeRange,
			Open:       first.Open + 1,
			High:       first.High + 1,
			Low:        first.Low + 1,
			Close:      first.Close + 1,
			Volume:     first.Volume + 1,
			Quality:    domain.DataQualitySuspect,
			Provenance: secondProvenance,
		})
		require.NoError(t, err)

		persistedSecond, err := service.IngestCandle(t.Context(), second)
		require.NoError(t, err)
		require.Equal(t, int64(2), readCount(t, store, "candles"))

		rangeForQuery, err := domain.NewTimeRange(
			first.TimeRange.Start.Add(-time.Minute),
			first.TimeRange.End.Add(time.Minute),
		)
		require.NoError(t, err)

		candles, err := store.QueryCandles(t.Context(), first.Instrument, first.Timeframe, rangeForQuery)
		require.NoError(t, err)
		require.Equal(t, []domain.Candle{persistedFirst, persistedSecond}, candles)

		replayed, err := store.ReplayCandles(t.Context(), first.Instrument, first.Timeframe, rangeForQuery)
		require.NoError(t, err)
		require.Len(t, replayed, 2)
		require.Equal(t, persistedFirst, replayed[0].Candle)
		require.Equal(t, persistedSecond, replayed[1].Candle)
		require.NotEqual(t, replayed[0].Identity, replayed[1].Identity)
	})

	t.Run("trade ingestion is idempotent and persists provenance and quality", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		first := makeTrade(t, instrument)
		persistedFirst, err := service.IngestTrade(t.Context(), first)
		require.NoError(t, err)
		require.Equal(t, time.UTC, persistedFirst.EventTime.Location())

		second, err := domain.NewTrade(domain.TradeParams{
			Instrument: first.Instrument,
			EventTime:  first.EventTime,
			Price:      first.Price + 1,
			Size:       first.Size + 1,
			Quality:    domain.DataQualityValidated,
			Provenance: first.Provenance,
		})
		require.NoError(t, err)

		persistedSecond, err := service.IngestTrade(t.Context(), second)
		require.NoError(t, err)
		require.Equal(t, second, persistedSecond)
		require.Equal(t, int64(1), readCount(t, store, "trades"))

		rangeForQuery, err := domain.NewTimeRange(
			first.EventTime.Add(-time.Minute),
			first.EventTime.Add(time.Minute),
		)
		require.NoError(t, err)

		trades, err := store.QueryTrades(t.Context(), first.Instrument, rangeForQuery)
		require.NoError(t, err)
		require.Len(t, trades, 1)
		require.Equal(t, persistedSecond, trades[0])
		require.InDelta(t, second.Price, trades[0].Price, 1e-9)
		require.InDelta(t, second.Size, trades[0].Size, 1e-9)
		require.Equal(t, domain.DataQualityValidated, trades[0].Quality)
		require.Equal(t, first.Provenance, trades[0].Provenance)
	})

	t.Run("trade ingestion falls back to event-time identity when provenance record ID is blank", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		blankRecordProvenance, err := domain.NewSourceProvenance(randomWord("source-blank"), "")
		require.NoError(t, err)

		baseTime := randomTime().UTC().Truncate(time.Second)
		first, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  baseTime,
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQualityRaw,
			Provenance: blankRecordProvenance,
		})
		require.NoError(t, err)

		persistedFirst, err := service.IngestTrade(t.Context(), first)
		require.NoError(t, err)

		sameEventTime, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  first.EventTime,
			Price:      first.Price + 1,
			Size:       first.Size + 1,
			Quality:    domain.DataQualityValidated,
			Provenance: blankRecordProvenance,
		})
		require.NoError(t, err)

		persistedSameEventTime, err := service.IngestTrade(t.Context(), sameEventTime)
		require.NoError(t, err)
		require.Equal(t, sameEventTime, persistedSameEventTime)
		require.Equal(t, int64(1), readCount(t, store, "trades"))

		secondEventTime, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  first.EventTime.Add(time.Second),
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQualitySuspect,
			Provenance: blankRecordProvenance,
		})
		require.NoError(t, err)

		persistedSecondEventTime, err := service.IngestTrade(t.Context(), secondEventTime)
		require.NoError(t, err)
		require.Equal(t, int64(2), readCount(t, store, "trades"))

		rangeForQuery, err := domain.NewTimeRange(
			first.EventTime.Add(-time.Second),
			secondEventTime.EventTime.Add(time.Second),
		)
		require.NoError(t, err)

		trades, err := store.QueryTrades(t.Context(), instrument, rangeForQuery)
		require.NoError(t, err)
		require.Equal(t, []domain.Trade{persistedSameEventTime, persistedSecondEventTime}, trades)

		replayed, err := store.ReplayTrades(t.Context(), instrument, rangeForQuery)
		require.NoError(t, err)
		require.Len(t, replayed, 2)
		require.Equal(t, persistedSameEventTime, replayed[0].Trade)
		require.Equal(t, persistedSecondEventTime, replayed[1].Trade)
		require.NotEqual(t, replayed[0].Identity, replayed[1].Identity)
		require.Equal(t, persistedFirst.Provenance, persistedSameEventTime.Provenance)
	})

	t.Run("candle queries and replay use deterministic start-ordered half-open reads", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		baseStart := randomTime().UTC().Truncate(time.Minute)

		makeRange := func(offsetMinutes int) domain.TimeRange {
			t.Helper()

			timeRange, rangeErr := domain.NewTimeRange(
				baseStart.Add(time.Duration(offsetMinutes)*time.Minute),
				baseStart.Add(time.Duration(offsetMinutes+1)*time.Minute),
			)
			require.NoError(t, rangeErr)

			return timeRange
		}

		makeCandleAt := func(offsetMinutes int, quality domain.DataQuality, suffix string) domain.Candle {
			t.Helper()

			provenance, provenanceErr := domain.NewSourceProvenance(
				randomWord("source-"+suffix),
				randomWord("record-"+suffix),
			)
			require.NoError(t, provenanceErr)

			candle, candleErr := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange:  makeRange(offsetMinutes),
				Open:       fake.Float64(2, 1, 1000),
				High:       fake.Float64(2, 1, 1000),
				Low:        fake.Float64(2, 0, 1000),
				Close:      fake.Float64(2, 1, 1000),
				Volume:     fake.Float64(4, 0, 10000),
				Quality:    quality,
				Provenance: provenance,
			})
			require.NoError(t, candleErr)

			return candle
		}

		beforeRange := makeCandleAt(-1, domain.DataQualityRaw, "before")
		atStartLater := makeCandleAt(1, domain.DataQualityValidated, "later")
		atStartEarlier := makeCandleAt(0, domain.DataQualitySuspect, "earlier")
		atEnd := makeCandleAt(2, domain.DataQualityRaw, "end")

		for _, candle := range []domain.Candle{beforeRange, atStartLater, atStartEarlier, atEnd} {
			_, err = service.IngestCandle(t.Context(), candle)
			require.NoError(t, err)
		}

		readRange, err := domain.NewTimeRange(baseStart, baseStart.Add(2*time.Minute))
		require.NoError(t, err)

		candles, err := store.QueryCandles(t.Context(), instrument, domain.Timeframe1m, readRange)
		require.NoError(t, err)
		require.Equal(t, []domain.Candle{atStartEarlier, atStartLater}, candles)
		require.Equal(t, domain.DataQualitySuspect, candles[0].Quality)
		require.Equal(t, domain.DataQualityValidated, candles[1].Quality)

		firstReplay, err := store.ReplayCandles(t.Context(), instrument, domain.Timeframe1m, readRange)
		require.NoError(t, err)
		secondReplay, err := store.ReplayCandles(t.Context(), instrument, domain.Timeframe1m, readRange)
		require.NoError(t, err)
		require.Equal(t, firstReplay, secondReplay)
		require.Len(t, firstReplay, 2)
		require.Equal(t, atStartEarlier, firstReplay[0].Candle)
		require.Equal(t, atStartLater, firstReplay[1].Candle)
		require.NotZero(t, firstReplay[0].Identity)
		require.NotZero(t, firstReplay[1].Identity)
		require.NotEqual(t, firstReplay[0].Identity, firstReplay[1].Identity)
	})

	t.Run("trade queries and replay use event-time ordering with stable tie-breakers", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		service, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		baseTime := randomTime().UTC().Truncate(time.Second)

		makeTradeAt := func(offset time.Duration, quality domain.DataQuality, suffix string) domain.Trade {
			t.Helper()

			provenance, provenanceErr := domain.NewSourceProvenance(
				randomWord("source-"+suffix),
				randomWord("record-"+suffix),
			)
			require.NoError(t, provenanceErr)

			trade, tradeErr := domain.NewTrade(domain.TradeParams{
				Instrument: instrument,
				EventTime:  baseTime.Add(offset),
				Price:      fake.Float64(4, 1, 100000),
				Size:       fake.Float64(4, 0, 100000),
				Quality:    quality,
				Provenance: provenance,
			})
			require.NoError(t, tradeErr)

			return trade
		}

		beforeRange := makeTradeAt(-time.Second, domain.DataQualityRaw, "before")
		atStart := makeTradeAt(0, domain.DataQualityValidated, "start")
		sameTimeFirst := makeTradeAt(time.Second, domain.DataQualitySuspect, "same-first")
		sameTimeSecond := makeTradeAt(time.Second, domain.DataQualityRaw, "same-second")
		atEnd := makeTradeAt(3*time.Second, domain.DataQualityValidated, "end")

		for _, trade := range []domain.Trade{beforeRange, atStart, sameTimeFirst, sameTimeSecond, atEnd} {
			_, err = service.IngestTrade(t.Context(), trade)
			require.NoError(t, err)
		}

		readRange, err := domain.NewTimeRange(baseTime, baseTime.Add(3*time.Second))
		require.NoError(t, err)

		trades, err := store.QueryTrades(t.Context(), instrument, readRange)
		require.NoError(t, err)
		require.Equal(t, []domain.Trade{atStart, sameTimeFirst, sameTimeSecond}, trades)
		require.Equal(t, domain.DataQualityValidated, trades[0].Quality)
		require.Equal(t, domain.DataQualitySuspect, trades[1].Quality)
		require.Equal(t, domain.DataQualityRaw, trades[2].Quality)

		firstReplay, err := store.ReplayTrades(t.Context(), instrument, readRange)
		require.NoError(t, err)
		secondReplay, err := store.ReplayTrades(t.Context(), instrument, readRange)
		require.NoError(t, err)
		require.Equal(t, firstReplay, secondReplay)
		require.Len(t, firstReplay, 3)
		require.Equal(t, atStart, firstReplay[0].Trade)
		require.Equal(t, sameTimeFirst, firstReplay[1].Trade)
		require.Equal(t, sameTimeSecond, firstReplay[2].Trade)
		require.NotZero(t, firstReplay[0].Identity)
		require.NotZero(t, firstReplay[1].Identity)
		require.NotZero(t, firstReplay[2].Identity)
		require.NotEqual(t, firstReplay[1].Identity, firstReplay[2].Identity)
	})

	t.Run("read service replay methods preserve deterministic identities across repeated reads", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")
		ingestionService, err := NewIngestionService(IngestionServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		readService, err := NewReadService(ReadServiceDeps{
			InstrumentStore: store,
			CandleStore:     store,
			TradeStore:      store,
		})
		require.NoError(t, err)

		instrument := makeInstrument(t)
		baseStart := randomTime().UTC().Truncate(time.Minute)
		candleRange, err := domain.NewTimeRange(baseStart, baseStart.Add(time.Minute))
		require.NoError(t, err)
		tradeRange, err := domain.NewTimeRange(baseStart, baseStart.Add(2*time.Minute))
		require.NoError(t, err)

		candleProvenance, err := domain.NewSourceProvenance(
			randomWord("source-candle"),
			randomWord("record-candle"),
		)
		require.NoError(t, err)
		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  candleRange,
			Open:       fake.Float64(2, 1, 1000),
			High:       fake.Float64(2, 1, 1000),
			Low:        fake.Float64(2, 0, 1000),
			Close:      fake.Float64(2, 1, 1000),
			Volume:     fake.Float64(4, 0, 10000),
			Quality:    domain.DataQualityValidated,
			Provenance: candleProvenance,
		})
		require.NoError(t, err)

		tradeProvenance, err := domain.NewSourceProvenance(
			randomWord("source-trade"),
			randomWord("record-trade"),
		)
		require.NoError(t, err)
		trade, err := domain.NewTrade(domain.TradeParams{
			Instrument: instrument,
			EventTime:  baseStart.Add(time.Second),
			Price:      fake.Float64(4, 1, 100000),
			Size:       fake.Float64(4, 0, 100000),
			Quality:    domain.DataQualitySuspect,
			Provenance: tradeProvenance,
		})
		require.NoError(t, err)

		_, err = ingestionService.IngestCandle(t.Context(), candle)
		require.NoError(t, err)
		_, err = ingestionService.IngestTrade(t.Context(), trade)
		require.NoError(t, err)

		firstCandles, err := readService.ReplayCandles(
			t.Context(),
			instrument,
			domain.Timeframe1m,
			tradeRange,
		)
		require.NoError(t, err)
		secondCandles, err := readService.ReplayCandles(
			t.Context(),
			instrument,
			domain.Timeframe1m,
			tradeRange,
		)
		require.NoError(t, err)
		require.Equal(t, firstCandles, secondCandles)
		require.Len(t, firstCandles, 1)
		require.Equal(t, candle, firstCandles[0].Candle)

		firstTrades, err := readService.ReplayTrades(t.Context(), instrument, tradeRange)
		require.NoError(t, err)
		secondTrades, err := readService.ReplayTrades(t.Context(), instrument, tradeRange)
		require.NoError(t, err)
		require.Equal(t, firstTrades, secondTrades)
		require.Len(t, firstTrades, 1)
		require.Equal(t, trade, firstTrades[0].Trade)
	})
}

func columnNames(rows []sqliteTableInfoRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	return names
}
