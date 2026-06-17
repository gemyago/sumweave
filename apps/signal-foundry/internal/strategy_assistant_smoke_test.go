//go:build !release

package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appinternal "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestStrategyAssistantSmoke(t *testing.T) {
	fake := faker.New()
	container, bundledSkillsRoot := makeWiredRuntimeContainer(t)

	type smokeDeps struct {
		dig.In

		DataStore            *data.DatabaseStore
		DataIngestionService *data.IngestionService
		DataReadService      *data.ReadService
		StrategyWorkspace    *appinternal.StrategyWorkspaceService
		EvaluationWorkspace  *appinternal.EvaluationWorkspaceService
	}

	var deps smokeDeps
	invokeErr := container.Invoke(func(resolved smokeDeps) {
		deps = resolved
	})
	require.NoError(t, invokeErr)
	require.NotNil(t, deps.DataStore)
	require.NotNil(t, deps.DataIngestionService)
	require.NotNil(t, deps.DataReadService)
	require.NotNil(t, deps.StrategyWorkspace)
	require.NotNil(t, deps.EvaluationWorkspace)
	require.NoError(t, deps.DataStore.AutoMigrate())

	makeInstrument := func(t *testing.T, symbol string) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue("hyperliquid-perps"),
			Symbol:     domain.Symbol(symbol),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeDefinition := func(instrument domain.Instrument) appinternal.StrategyDefinitionInput {
		return appinternal.StrategyDefinitionInput{
			Kind: "moving-average-crossover",
			Instrument: appinternal.StrategyInstrumentInput{
				Venue:      instrument.Venue.String(),
				Symbol:     instrument.Symbol.String(),
				AssetClass: instrument.AssetClass.String(),
				Active:     instrument.Active,
			},
			Timeframe:  "1h",
			Parameters: appinternal.StrategyParameterSummary{FastWindow: 2, SlowWindow: 3},
		}
	}

	seedCandles := func(t *testing.T, instrument domain.Instrument, start time.Time, closes []float64) time.Time {
		t.Helper()

		for i, closeValue := range closes {
			openValue := closeValue
			if i > 0 {
				openValue = closes[i-1]
			}
			provenance, err := domain.NewSourceProvenance(
				"smoke-fixture",
				fmt.Sprintf("%s-%02d", instrument.Symbol, i+1),
			)
			require.NoError(t, err)

			candleStart := start.Add(time.Duration(i) * time.Hour)
			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1h,
				TimeRange:  domain.TimeRange{Start: candleStart, End: candleStart.Add(time.Hour)},
				Open:       openValue,
				High:       max(openValue, closeValue) + 0.5,
				Low:        min(openValue, closeValue) - 0.5,
				Close:      closeValue,
				Volume:     100 + float64(i),
				Quality:    domain.DataQualityRaw,
				Provenance: provenance,
			})
			require.NoError(t, err)

			_, err = deps.DataIngestionService.IngestCandle(t.Context(), candle)
			require.NoError(t, err)
		}

		return start.Add(time.Duration(len(closes)) * time.Hour)
	}

	t.Run("completed loop stays executable with seeded local data", func(t *testing.T) {
		symbol := "BTC-" + fake.Lorem().Word()
		instrument := makeInstrument(t, symbol)
		rangeStart := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
		rangeEnd := seedCandles(t, instrument, rangeStart, []float64{10, 11, 10, 9, 11, 12, 9, 8})

		availability, err := deps.DataReadService.ListCandleAvailability(
			t.Context(),
			data.CandleAvailabilityListQuery{
				Venue:      instrument.Venue,
				Symbol:     instrument.Symbol,
				AssetClass: instrument.AssetClass,
				Limit:      10,
			},
		)
		require.NoError(t, err)
		require.Len(t, availability.Items, 1)
		assert.Equal(t, instrument.Symbol, availability.Items[0].Symbol)
		assert.Equal(t, domain.Timeframe1h, availability.Items[0].DefaultSlice.Timeframe)
		assert.Equal(t, rangeStart, availability.Items[0].DefaultSlice.StartAt)
		assert.Equal(t, rangeEnd, availability.Items[0].DefaultSlice.EndAt)

		replayed, err := deps.DataReadService.ReplayCandles(
			t.Context(),
			instrument,
			domain.Timeframe1h,
			domain.TimeRange{Start: rangeStart, End: rangeEnd},
		)
		require.NoError(t, err)
		require.Len(t, replayed, 8)
		assert.Equal(t, uint64(1), replayed[0].Identity)
		assert.InDelta(t, 8.0, replayed[len(replayed)-1].Candle.Close, 0.000001)

		definition := makeDefinition(instrument)
		validation, err := deps.StrategyWorkspace.ValidateDefinition(t.Context(), definition)
		require.NoError(t, err)
		require.True(t, validation.Valid)
		require.NotNil(t, validation.Preview)
		assert.Equal(t, "strategy-artifact.v0", validation.Preview.SchemaVersion)

		strategyID := "smoke-strategy-" + fake.Lorem().Word()
		createdVersion, err := deps.StrategyWorkspace.CreateVersion(
			t.Context(),
			appinternal.CreateStrategyVersionParams{
				StrategyID:  strategyID,
				Version:     "v1",
				DisplayName: "Smoke strategy",
				Notes:       "seeded smoke coverage",
				Definition:  definition,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "ready", createdVersion.Status)

		evaluation, err := deps.EvaluationWorkspace.CreateEvaluation(
			t.Context(),
			appinternal.CreateEvaluationParams{
				StrategyID:      strategyID,
				StrategyVersion: createdVersion.Version,
				Start:           rangeStart,
				End:             rangeEnd,
				Quantity:        1,
				Note:            "smoke evaluation",
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "completed", evaluation.Status)
		assert.Empty(t, evaluation.FailureReason)
		assert.NotEmpty(t, evaluation.RunID)

		report, err := deps.EvaluationWorkspace.GetEvaluationReport(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, evaluation.RunID, report.RunID)
		assert.Equal(t, "completed", report.Status)
		assert.NotNil(t, report.Decision)

		evidence, err := deps.EvaluationWorkspace.GetEvaluationEvidence(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, evaluation.RunID, evidence.RunID)
		assert.Equal(t, "completed", evidence.Status)
		assert.NotEmpty(t, evidence.Traces)

		skillBody, err := os.ReadFile(filepath.Join(bundledSkillsRoot, "strategy-research-loop", "SKILL.md"))
		require.NoError(t, err)
		assert.Contains(t, string(skillBody), "sf_data_list_candle_availability")
		assert.Contains(t, string(skillBody), "Safety boundaries")
	})

	t.Run("missing local data persists explicit data-unavailable failure", func(t *testing.T) {
		instrument := makeInstrument(t, "ETH-"+fake.Lorem().Word())
		definition := makeDefinition(instrument)
		strategyID := "smoke-missing-data-" + fake.Lorem().Word()

		createdVersion, err := deps.StrategyWorkspace.CreateVersion(
			t.Context(),
			appinternal.CreateStrategyVersionParams{
				StrategyID:  strategyID,
				Version:     "v1",
				DisplayName: "Smoke missing data",
				Notes:       "assert honest failed-run behavior",
				Definition:  definition,
			},
		)
		require.NoError(t, err)

		rangeStart := time.Date(2026, time.June, 16, 0, 0, 0, 0, time.UTC)
		evaluation, err := deps.EvaluationWorkspace.CreateEvaluation(
			t.Context(),
			appinternal.CreateEvaluationParams{
				StrategyID:      strategyID,
				StrategyVersion: createdVersion.Version,
				Start:           rangeStart,
				End:             rangeStart.Add(8 * time.Hour),
				Quantity:        1,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "failed", evaluation.Status)
		assert.Equal(t, "replay-data-unavailable", evaluation.FailureReason)
		assert.NotEmpty(t, evaluation.FailureDetails)

		report, err := deps.EvaluationWorkspace.GetEvaluationReport(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, "failed", report.Status)
		assert.Equal(t, "replay-data-unavailable", report.FailureReason)

		evidence, err := deps.EvaluationWorkspace.GetEvaluationEvidence(t.Context(), evaluation.RunID)
		require.NoError(t, err)
		assert.Equal(t, "failed", evidence.Status)
		assert.Empty(t, evidence.Traces)
		assert.Empty(t, evidence.OrderIntents)
	})
}
