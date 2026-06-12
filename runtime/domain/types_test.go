package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDomain(t *testing.T) {
	t.Parallel()

	fake := faker.New()

	randomWord := func(prefix string) string {
		return prefix + "-" + strings.ToLower(fake.Lorem().Word())
	}

	randomLocationTime := func() time.Time {
		zone := time.FixedZone(randomWord("zone"), fake.IntBetween(-11, 12)*3600)
		return time.Date(
			fake.IntBetween(2020, 2032),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			zone,
		)
	}

	validAssetClasses := []AssetClass{
		AssetClassCrypto,
		AssetClassEquity,
		AssetClassFX,
		AssetClassFuture,
		AssetClassIndex,
		AssetClassOption,
	}
	validTimeframes := []Timeframe{
		Timeframe1m,
		Timeframe5m,
		Timeframe15m,
		Timeframe1h,
		Timeframe4h,
		Timeframe1d,
	}
	validQualities := []DataQuality{
		DataQualityRaw,
		DataQualityValidated,
		DataQualitySuspect,
	}

	t.Run("constructors validate canonical strings and enums", func(t *testing.T) {
		t.Parallel()

		venueText := "  " + randomWord("venue") + "  "
		symbolText := "  " + strings.ToUpper(randomWord("symbol")) + "  "
		assetClassText := "  " + strings.ToUpper(
			validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)].String(),
		) + "  "
		timeframeText := "  " + strings.ToUpper(
			validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)].String(),
		) + "  "
		qualityText := "  " + strings.ToUpper(
			validQualities[fake.IntBetween(0, len(validQualities)-1)].String(),
		) + "  "

		venue, err := NewVenue(venueText)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(venueText), venue.String())

		symbol, err := NewSymbol(symbolText)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(symbolText), symbol.String())

		assetClass, err := NewAssetClass(assetClassText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(assetClassText)), assetClass.String())

		timeframe, err := NewTimeframe(timeframeText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(timeframeText)), timeframe.String())

		quality, err := NewDataQuality(qualityText)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(strings.TrimSpace(qualityText)), quality.String())

		_, err = NewAssetClass(randomWord("bad-asset-class"))
		require.Error(t, err)
		_, err = NewTimeframe(randomWord("bad-timeframe"))
		require.Error(t, err)
		_, err = NewDataQuality(randomWord("bad-quality"))
		require.Error(t, err)
	})

	t.Run("canonical records normalize UTC timestamps and compare as whole values", func(t *testing.T) {
		t.Parallel()

		venue, err := NewVenue(randomWord("venue"))
		require.NoError(t, err)
		symbol, err := NewSymbol(strings.ToUpper(randomWord("symbol")))
		require.NoError(t, err)
		assetClass := validAssetClasses[fake.IntBetween(0, len(validAssetClasses)-1)]
		instrument, err := NewInstrument(InstrumentParams{
			Venue:      venue,
			Symbol:     symbol,
			AssetClass: assetClass,
			Active:     fake.Bool(),
		})
		require.NoError(t, err)

		provenance, err := NewSourceProvenance(randomWord("source"), "  "+randomWord("record")+"  ")
		require.NoError(t, err)

		start := randomLocationTime()
		end := start.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)
		timeRange, err := NewTimeRange(start, end)
		require.NoError(t, err)
		require.Equal(t, time.UTC, timeRange.Start.Location())
		require.Equal(t, time.UTC, timeRange.End.Location())

		timeframe := validTimeframes[fake.IntBetween(0, len(validTimeframes)-1)]
		quality := validQualities[fake.IntBetween(0, len(validQualities)-1)]

		candle, err := NewCandle(CandleParams{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange:  timeRange,
			Open:       fake.Float64(2, 1, 9999),
			High:       fake.Float64(2, 1, 9999),
			Low:        fake.Float64(2, 1, 9999),
			Close:      fake.Float64(2, 1, 9999),
			Volume:     fake.Float64(4, 0, 99999),
			Quality:    quality,
			Provenance: provenance,
		})
		require.NoError(t, err)

		expectedCandle := Candle{
			Instrument: instrument,
			Timeframe:  timeframe,
			TimeRange: TimeRange{
				Start: start.UTC(),
				End:   end.UTC(),
			},
			Open:    candle.Open,
			High:    candle.High,
			Low:     candle.Low,
			Close:   candle.Close,
			Volume:  candle.Volume,
			Quality: quality,
			Provenance: SourceProvenance{
				Source:   provenance.Source,
				RecordID: strings.TrimSpace(provenance.RecordID),
			},
		}
		require.Equal(t, expectedCandle, candle)

		eventTime := randomLocationTime()
		trade, err := NewTrade(TradeParams{
			Instrument: instrument,
			EventTime:  eventTime,
			Price:      fake.Float64(4, 1, 99999),
			Size:       fake.Float64(6, 0, 99999),
			Quality:    quality,
			Provenance: provenance,
		})
		require.NoError(t, err)

		expectedTrade := Trade{
			Instrument: instrument,
			EventTime:  eventTime.UTC(),
			Price:      trade.Price,
			Size:       trade.Size,
			Quality:    quality,
			Provenance: SourceProvenance{
				Source:   provenance.Source,
				RecordID: strings.TrimSpace(provenance.RecordID),
			},
		}
		require.Equal(t, expectedTrade, trade)
		require.Equal(t, time.UTC, trade.EventTime.Location())
	})

	t.Run("domain structs remain persistence free", func(t *testing.T) {
		t.Parallel()

		forbiddenFields := map[string]struct{}{
			"ID":        {},
			"CreatedAt": {},
			"UpdatedAt": {},
			"DeletedAt": {},
		}
		structTypes := []reflect.Type{
			reflect.TypeFor[Instrument](),
			reflect.TypeFor[SourceProvenance](),
			reflect.TypeFor[TimeRange](),
			reflect.TypeFor[Candle](),
			reflect.TypeFor[Trade](),
		}

		for _, typ := range structTypes {
			_, hasValueTableName := typ.MethodByName("TableName")
			require.False(t, hasValueTableName, "%s should not expose TableName", typ.Name())

			_, hasPointerTableName := reflect.PointerTo(typ).MethodByName("TableName")
			require.False(t, hasPointerTableName, "%s should not expose pointer TableName", typ.Name())

			for _, field := range reflect.VisibleFields(typ) {
				require.Empty(t, field.Tag.Get("gorm"), "%s.%s should not carry gorm tags", typ.Name(), field.Name)

				_, forbidden := forbiddenFields[field.Name]
				require.False(t, forbidden, "%s.%s should not carry persistence-only fields", typ.Name(), field.Name)
			}
		}
	})
}
