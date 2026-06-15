package domain

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestExecutionSnapshotTypes(t *testing.T) {
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

	makeInstrument := func(t *testing.T) Instrument {
		t.Helper()

		instrument, err := NewInstrument(InstrumentParams{
			Venue:      Venue(randomWord("venue")),
			Symbol:     Symbol(strings.ToUpper(randomWord("symbol"))),
			AssetClass: AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	t.Run("position snapshots validate quantity and entry rules", func(t *testing.T) {
		t.Parallel()

		averageEntryPrice := fake.Float64(2, 10, 1000)
		snapshot, err := NewPositionSnapshot(PositionSnapshotParams{
			SnapshotID:           randomWord("position-snapshot"),
			SourceFillID:         randomWord("fill"),
			Mode:                 DecisionModePaper,
			StrategyID:           randomWord("strategy-id"),
			StrategyVersion:      randomWord("strategy-version"),
			StrategyArtifactHash: randomWord("strategy-hash"),
			Instrument:           makeInstrument(t),
			Quantity:             fake.Float64(4, 1, 25),
			AverageEntryPrice:    &averageEntryPrice,
			RealizedPnL:          fake.Float64(4, -100, 100),
			ExposureNotional:     fake.Float64(4, 10, 10000),
			EventTime:            randomLocationTime(),
			Metadata:             map[string]string{"projection": "fill-ledger-v0"},
		})
		require.NoError(t, err)
		require.Equal(t, time.UTC, snapshot.EventTime.Time().Location())
		require.NotNil(t, snapshot.AverageEntryPrice)

		_, err = NewPositionSnapshot(PositionSnapshotParams{
			SnapshotID:           randomWord("position-snapshot"),
			SourceFillID:         randomWord("fill"),
			Mode:                 DecisionModePaper,
			StrategyID:           randomWord("strategy-id"),
			StrategyVersion:      randomWord("strategy-version"),
			StrategyArtifactHash: randomWord("strategy-hash"),
			Instrument:           makeInstrument(t),
			Quantity:             fake.Float64(4, 1, 25),
			RealizedPnL:          0,
			ExposureNotional:     fake.Float64(4, 10, 10000),
			EventTime:            randomLocationTime(),
		})
		require.ErrorContains(t, err, "position snapshot average entry price is required")

		_, err = NewPositionSnapshot(PositionSnapshotParams{
			SnapshotID:           randomWord("position-snapshot"),
			SourceFillID:         randomWord("fill"),
			Mode:                 DecisionModeBacktest,
			StrategyID:           randomWord("strategy-id"),
			StrategyVersion:      randomWord("strategy-version"),
			StrategyArtifactHash: randomWord("strategy-hash"),
			Instrument:           makeInstrument(t),
			Quantity:             0,
			AverageEntryPrice:    &averageEntryPrice,
			RealizedPnL:          0,
			ExposureNotional:     0,
			EventTime:            randomLocationTime(),
		})
		require.ErrorContains(t, err, "must be empty when quantity is zero")

		_, err = NewPositionSnapshot(PositionSnapshotParams{
			SnapshotID:           randomWord("position-snapshot"),
			SourceFillID:         randomWord("fill"),
			Mode:                 DecisionModeBacktest,
			StrategyID:           randomWord("strategy-id"),
			StrategyVersion:      randomWord("strategy-version"),
			StrategyArtifactHash: randomWord("strategy-hash"),
			Instrument:           makeInstrument(t),
			Quantity:             0,
			RealizedPnL:          0,
			ExposureNotional:     math.NaN(),
			EventTime:            randomLocationTime(),
		})
		require.ErrorContains(t, err, "position snapshot exposure notional must be finite")
	})

	t.Run("portfolio snapshots keep optional unrealized pnl finite", func(t *testing.T) {
		t.Parallel()

		unrealizedPnL := fake.Float64(4, -100, 100)
		snapshot, err := NewPortfolioSnapshot(PortfolioSnapshotParams{
			SnapshotID:    randomWord("portfolio-snapshot"),
			SourceFillID:  randomWord("fill"),
			Mode:          DecisionModePaper,
			GrossExposure: fake.Float64(4, 10, 10000),
			NetExposure:   fake.Float64(4, -5000, 5000),
			RealizedPnL:   fake.Float64(4, -100, 100),
			UnrealizedPnL: &unrealizedPnL,
			EventTime:     randomLocationTime(),
			Metadata:      map[string]string{"projection": "fill-ledger-v0"},
		})
		require.NoError(t, err)
		require.NotNil(t, snapshot.UnrealizedPnL)
		require.Equal(t, time.UTC, snapshot.EventTime.Time().Location())

		nan := math.NaN()
		_, err = NewPortfolioSnapshot(PortfolioSnapshotParams{
			SnapshotID:    randomWord("portfolio-snapshot"),
			SourceFillID:  randomWord("fill"),
			Mode:          DecisionModeBacktest,
			GrossExposure: fake.Float64(4, 10, 10000),
			NetExposure:   fake.Float64(4, -5000, 5000),
			RealizedPnL:   fake.Float64(4, -100, 100),
			UnrealizedPnL: &nan,
			EventTime:     randomLocationTime(),
		})
		require.ErrorContains(t, err, "portfolio snapshot unrealized pnl must be finite")
	})
}
