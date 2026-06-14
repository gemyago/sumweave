package strategy

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestStrategyDSLV0(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) faker.Faker {
		t.Helper()

		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(t.Name()))

		return faker.NewWithSeedInt64(int64(hasher.Sum64()))
	}

	randomWord := func(t *testing.T, fake faker.Faker, prefix string) string {
		t.Helper()

		return prefix + "-" + strings.ToLower(fake.Lorem().Word()) + "-" + strconv.Itoa(fake.IntBetween(1000, 9999))
	}

	makeRawPayload := func(
		kind string,
		venue string,
		symbol string,
		assetClass string,
		timeframe string,
		active bool,
		fastWindow int,
		slowWindow int,
	) []byte {
		return fmt.Appendf(nil, `{
			"kind": %q,
			"instrument": {
				"venue": %q,
				"symbol": %q,
				"assetClass": %q,
				"active": %t
			},
			"timeframe": %q,
			"parameters": {
				"fastWindow": %d,
				"slowWindow": %d
			}
		}`,
			kind,
			venue,
			symbol,
			assetClass,
			active,
			timeframe,
			fastWindow,
			slowWindow,
		)
	}

	makeCanonicalInstrument := func(
		t *testing.T,
		venue string,
		symbol string,
		assetClass string,
		active bool,
	) domain.Instrument {
		t.Helper()

		canonicalVenue, err := domain.NewVenue(venue)
		require.NoError(t, err)

		canonicalSymbol, err := domain.NewSymbol(symbol)
		require.NoError(t, err)

		canonicalAssetClass, err := domain.NewAssetClass(assetClass)
		require.NoError(t, err)

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      canonicalVenue,
			Symbol:     canonicalSymbol,
			AssetClass: canonicalAssetClass,
			Active:     active,
		})
		require.NoError(t, err)

		return instrument
	}

	t.Run("parseDSLV0", func(t *testing.T) {
		t.Parallel()

		t.Run("accepts valid moving average crossover payload", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := "  " + randomWord(t, fake, "venue") + "  "
			symbol := "  " + strings.ToUpper(randomWord(t, fake, "symbol")) + "  "
			assetClass := "  CRYPTO  "
			timeframe := " 1H "
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			canonical, err := parseDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				venue,
				symbol,
				assetClass,
				timeframe,
				active,
				fastWindow,
				slowWindow,
			))

			require.NoError(t, err)
			require.Equal(t, canonicalStrategyDSLV0{
				Instrument: makeCanonicalInstrument(t, venue, symbol, assetClass, active),
				Timeframe:  domain.Timeframe1h,
				Kind:       domain.StrategyKindMovingAverageCrossover,
				Parameters: MovingAverageCrossoverParams{
					FastWindow: fastWindow,
					SlowWindow: slowWindow,
				},
			}, canonical)
		})

		t.Run("rejects unsupported strategy kind", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			canonical, err := parseDSLV0(makeRawPayload(
				randomWord(t, fake, "kind"),
				randomWord(t, fake, "venue"),
				strings.ToUpper(randomWord(t, fake, "symbol")),
				domain.AssetClassCrypto.String(),
				domain.Timeframe1m.String(),
				fake.Bool(),
				fake.IntBetween(1, 10),
				fake.IntBetween(11, 20),
			))

			require.ErrorIs(t, err, ErrValidation)
			require.ErrorContains(t, err, "invalid strategy kind")
			require.Equal(t, canonicalStrategyDSLV0{}, canonical)
		})

		t.Run("rejects invalid instrument or timeframe", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			testCases := []struct {
				name    string
				payload []byte
				message string
			}{
				{
					name: "instrument",
					payload: makeRawPayload(
						domain.StrategyKindMovingAverageCrossover.String(),
						" ",
						strings.ToUpper(randomWord(t, fake, "symbol")),
						domain.AssetClassCrypto.String(),
						domain.Timeframe1m.String(),
						fake.Bool(),
						fake.IntBetween(1, 10),
						fake.IntBetween(11, 20),
					),
					message: "strategy instrument venue is required",
				},
				{
					name: "timeframe",
					payload: makeRawPayload(
						domain.StrategyKindMovingAverageCrossover.String(),
						randomWord(t, fake, "venue"),
						strings.ToUpper(randomWord(t, fake, "symbol")),
						domain.AssetClassCrypto.String(),
						randomWord(t, fake, "timeframe"),
						fake.Bool(),
						fake.IntBetween(1, 10),
						fake.IntBetween(11, 20),
					),
					message: "strategy timeframe is required",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					canonical, err := parseDSLV0(testCase.payload)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.message)
					require.Equal(t, canonicalStrategyDSLV0{}, canonical)
				})
			}
		})

		t.Run("rejects invalid crossover parameters", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			baseVenue := randomWord(t, fake, "venue")
			baseSymbol := strings.ToUpper(randomWord(t, fake, "symbol"))
			active := fake.Bool()
			testCases := []struct {
				name       string
				fastWindow int
				slowWindow int
				message    string
			}{
				{
					name:       "fast window non positive",
					fastWindow: 0,
					slowWindow: fake.IntBetween(1, 20),
					message:    "moving average crossover fast window must be positive",
				},
				{
					name:       "slow window non positive",
					fastWindow: fake.IntBetween(1, 20),
					slowWindow: 0,
					message:    "moving average crossover slow window must be positive",
				},
				{
					name:       "equal windows",
					fastWindow: fake.IntBetween(1, 20),
					message:    "moving average crossover fast window must be less than slow window",
				},
				{
					name:       "fast window greater than slow window",
					fastWindow: fake.IntBetween(11, 20),
					slowWindow: fake.IntBetween(1, 10),
					message:    "moving average crossover fast window must be less than slow window",
				},
			}

			for index, testCase := range testCases {
				if testCase.name == "equal windows" {
					testCase.slowWindow = testCase.fastWindow
				}

				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					canonical, err := parseDSLV0(makeRawPayload(
						domain.StrategyKindMovingAverageCrossover.String(),
						baseVenue+"-"+strconv.Itoa(index),
						baseSymbol+strconv.Itoa(index),
						domain.AssetClassCrypto.String(),
						domain.Timeframe1m.String(),
						active,
						testCase.fastWindow,
						testCase.slowWindow,
					))

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.message)
					require.Equal(t, canonicalStrategyDSLV0{}, canonical)
				})
			}
		})

		t.Run("canonicalizes equivalent payloads", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := randomWord(t, fake, "venue")
			symbol := strings.ToUpper(randomWord(t, fake, "symbol"))
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			payloadA := fmt.Appendf(nil, `{
				"kind": %q,
				"instrument": {
					"venue": %q,
					"symbol": %q,
					"assetClass": %q,
					"active": %t
				},
				"timeframe": %q,
				"parameters": {
					"fastWindow": %d,
					"slowWindow": %d
				}
			}`,
				domain.StrategyKindMovingAverageCrossover.String(),
				venue,
				symbol,
				domain.AssetClassCrypto.String(),
				active,
				domain.Timeframe1m.String(),
				fastWindow,
				slowWindow,
			)
			payloadB := fmt.Appendf(
				nil,
				`{ "parameters": { "slowWindow": %d, "fastWindow": %d }, "timeframe": %q, "instrument": { "active": %t, "assetClass": %q, "symbol": %q, "venue": %q }, "kind": %q }`,
				slowWindow,
				fastWindow,
				" 1M ",
				active,
				" crypto ",
				"  "+symbol+"  ",
				"  "+venue+"  ",
				"  MOVING-AVERAGE-CROSSOVER  ",
			)

			canonicalA, err := parseDSLV0(payloadA)
			require.NoError(t, err)

			canonicalB, err := parseDSLV0(payloadB)
			require.NoError(t, err)

			require.Equal(t, canonicalA, canonicalB)
		})

		t.Run("rejects unknown and code like fields", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := randomWord(t, fake, "venue")
			symbol := strings.ToUpper(randomWord(t, fake, "symbol"))
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			testCases := []struct {
				name    string
				payload []byte
				message string
			}{
				{
					name: "top level script field",
					payload: fmt.Appendf(nil, `{
						"kind": %q,
						"instrument": {"venue": %q, "symbol": %q, "assetClass": %q, "active": %t},
						"timeframe": %q,
						"parameters": {"fastWindow": %d, "slowWindow": %d},
						"script": %q
					}`,
						domain.StrategyKindMovingAverageCrossover.String(),
						venue,
						symbol,
						domain.AssetClassCrypto.String(),
						active,
						domain.Timeframe1m.String(),
						fastWindow,
						slowWindow,
						randomWord(t, fake, "script"),
					),
					message: "unknown field \"script\"",
				},
				{
					name: "nested module field",
					payload: fmt.Appendf(nil, `{
						"kind": %q,
						"instrument": {"venue": %q, "symbol": %q, "assetClass": %q, "active": %t},
						"timeframe": %q,
						"parameters": {"fastWindow": %d, "slowWindow": %d, "module": %q}
					}`,
						domain.StrategyKindMovingAverageCrossover.String(),
						venue,
						symbol,
						domain.AssetClassCrypto.String(),
						active,
						domain.Timeframe1m.String(),
						fastWindow,
						slowWindow,
						randomWord(t, fake, "module"),
					),
					message: "unknown field \"module\"",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					canonical, err := parseDSLV0(testCase.payload)

					require.ErrorIs(t, err, ErrValidation)
					require.ErrorContains(t, err, testCase.message)
					require.Equal(t, canonicalStrategyDSLV0{}, canonical)
				})
			}
		})
	})

	t.Run("mapDSLV0ToEvaluateRequest", func(t *testing.T) {
		t.Parallel()

		t.Run("maps canonical DSL with explicit range", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := randomWord(t, fake, "venue")
			symbol := strings.ToUpper(randomWord(t, fake, "symbol"))
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			canonical, err := parseDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				venue,
				symbol,
				domain.AssetClassCrypto.String(),
				domain.Timeframe5m.String(),
				active,
				fastWindow,
				slowWindow,
			))
			require.NoError(t, err)

			rangeStart := time.Date(
				fake.IntBetween(2022, 2031),
				time.Month(fake.IntBetween(1, 12)),
				fake.IntBetween(1, 28),
				fake.IntBetween(0, 23),
				fake.IntBetween(0, 59),
				fake.IntBetween(0, 59),
				fake.IntBetween(0, 999999999),
				time.FixedZone(randomWord(t, fake, "zone-start"), fake.IntBetween(-11, 12)*3600),
			)
			rangeEnd := rangeStart.Add(time.Duration(fake.IntBetween(1, 180)) * time.Minute)

			request, err := mapDSLV0ToEvaluateRequest(canonical, domain.TimeRange{
				Start: rangeStart,
				End:   rangeEnd,
			})

			require.NoError(t, err)
			expectedRange, err := domain.NewTimeRange(rangeStart, rangeEnd)
			require.NoError(t, err)
			require.Equal(t, EvaluateRequest{
				Instrument:   canonical.Instrument,
				Timeframe:    canonical.Timeframe,
				TimeRange:    expectedRange,
				StrategyKind: canonical.Kind,
				Parameters:   canonical.Parameters,
			}, request)
		})

		t.Run("rejects invalid evaluation range", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			canonical, err := parseDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				randomWord(t, fake, "venue"),
				strings.ToUpper(randomWord(t, fake, "symbol")),
				domain.AssetClassCrypto.String(),
				domain.Timeframe15m.String(),
				fake.Bool(),
				fake.IntBetween(1, 20),
				fake.IntBetween(21, 40),
			))
			require.NoError(t, err)

			testCases := []struct {
				name      string
				timeRange domain.TimeRange
				message   string
			}{
				{
					name:      "missing start",
					timeRange: domain.TimeRange{},
					message:   "strategy evaluation time range: time range start is required",
				},
				{
					name: "end before start",
					timeRange: domain.TimeRange{
						Start: time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC),
						End:   time.Date(2026, time.January, 2, 9, 0, 0, 0, time.UTC),
					},
					message: "strategy evaluation time range: time range end must be after start",
				},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()

					request, mapErr := mapDSLV0ToEvaluateRequest(canonical, testCase.timeRange)

					require.ErrorIs(t, mapErr, ErrValidation)
					require.ErrorContains(t, mapErr, testCase.message)
					require.Equal(t, EvaluateRequest{}, request)
				})
			}
		})
	})
}
