package flows

import (
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/audit"
	"github.com/gemyago/signal-foundry/runtime/backtest"
	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/execution"
	"github.com/gemyago/signal-foundry/runtime/governor"
	"github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestDurableBacktestFlow(t *testing.T) {
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

	makeInstrument := func(t *testing.T, fake faker.Faker) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(randomWord(t, fake, "venue")),
			Symbol:     domain.Symbol(strings.ToUpper(randomWord(t, fake, "symbol"))),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeReplayCandles := func(
		t *testing.T,
		fake faker.Faker,
		instrument domain.Instrument,
		start time.Time,
		closes []float64,
	) []data.ReplayCandle {
		t.Helper()

		replayed := make([]data.ReplayCandle, len(closes))
		identityBase := uint64(fake.IntBetween(100, 900))
		for idx, closeValue := range closes {
			provenance, err := domain.NewSourceProvenance(
				randomWord(t, fake, "source"),
				randomWord(t, fake, "record"),
			)
			require.NoError(t, err)

			candle, err := domain.NewCandle(domain.CandleParams{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				TimeRange: domain.TimeRange{
					Start: start.Add(time.Duration(idx) * time.Minute),
					End:   start.Add(time.Duration(idx+1) * time.Minute),
				},
				Open:       closeValue - 0.25,
				High:       closeValue + 0.5,
				Low:        closeValue - 0.5,
				Close:      closeValue,
				Volume:     float64(fake.IntBetween(10, 5000)),
				Quality:    domain.DataQualityValidated,
				Provenance: provenance,
			})
			require.NoError(t, err)

			replayed[idx] = data.ReplayCandle{
				Identity: identityBase + uint64(idx) + 1,
				Candle:   candle,
			}
		}

		return replayed
	}

	makeAction := func(
		t *testing.T,
		instrument domain.Instrument,
		decisionTime time.Time,
		kind domain.CandidateActionKind,
	) domain.CandidateAction {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy: domain.StrategyIdentity{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			},
			Kind:         kind,
			DecisionTime: decisionTime,
			InputRange: domain.TimeRange{
				Start: decisionTime.Add(-time.Minute),
				End:   decisionTime,
			},
			Quality: domain.DataQualityValidated,
		})
		require.NoError(t, err)

		return action
	}

	t.Run("links durable records across audit execution snapshots and reports", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestStart := time.Date(2026, 4, 5, 9, 0, 0, 0, time.FixedZone(randomWord(t, fake, "zone"), 0))
		requestRange, err := domain.NewTimeRange(requestStart, requestStart.Add(8*time.Minute))
		require.NoError(t, err)

		replayed := makeReplayCandles(t, fake, instrument, requestStart, []float64{10, 11, 10, 9, 11, 12, 9, 8})
		firstAction := makeAction(t, instrument, requestStart.Add(4*time.Minute), domain.CandidateActionKindLong)
		secondAction := makeAction(t, instrument, requestStart.Add(6*time.Minute), domain.CandidateActionKindShort)

		replayReader := &fakeCandleReplayReader{result: replayed}
		analyticsCalc := &fakeAnalyticsCalculator{}
		strategyEvaluator := &fakeStrategyEvaluator{result: strategy.EvaluateResult{
			Strategy: domain.StrategyIdentity{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			},
			TimeRange: requestRange,
			Parameters: strategy.MovingAverageCrossoverParams{
				FastWindow: 2,
				SlowWindow: 3,
			},
			Actions: []domain.CandidateAction{firstAction, secondAction},
		}}

		auditStore, err := audit.NewDatabaseStore(":memory:", audit.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, auditStore.AutoMigrate())
		auditService, err := audit.NewService(auditStore)
		require.NoError(t, err)

		executionStore, err := execution.NewDatabaseStore(":memory:", execution.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, executionStore.AutoMigrate())
		paperExecutor, err := execution.NewPaperService(executionStore)
		require.NoError(t, err)
		snapshotProjector, err := execution.NewSnapshotService(executionStore)
		require.NoError(t, err)

		backtestStore, err := backtest.NewDatabaseStore(":memory:", backtest.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, backtestStore.AutoMigrate())
		backtestService, err := backtest.NewService(backtestStore)
		require.NoError(t, err)

		flow, err := NewDurableBacktestFlow(DurableBacktestFlowDeps{
			CandleReplayReader:  replayReader,
			AnalyticsCalculator: analyticsCalc,
			StrategyEvaluator:   strategyEvaluator,
			AuditRecorder:       auditService,
			GovernorEvaluator:   governor.NewService(),
			PaperExecutor:       paperExecutor,
			SnapshotProjector:   snapshotProjector,
			BacktestRecorder:    backtestService,
		})
		require.NoError(t, err)

		result, err := flow.Run(t.Context(), PaperBacktestRequest{
			RunID:                "  " + randomWord(t, fake, "run") + "  ",
			Mode:                 domain.DecisionModeBacktest,
			StrategyID:           "  " + randomWord(t, fake, "strategy-id") + "  ",
			StrategyVersion:      "  " + randomWord(t, fake, "strategy-version") + "  ",
			StrategyArtifactHash: "  " + randomWord(t, fake, "strategy-artifact") + "  ",
			Instrument:           instrument,
			Timeframe:            domain.Timeframe1m,
			TimeRange:            requestRange,
			StrategyParameters: strategy.MovingAverageCrossoverParams{
				FastWindow: 2,
				SlowWindow: 3,
			},
			GovernorPolicy: governor.Policy{
				AllowedModes: []domain.DecisionMode{domain.DecisionModeBacktest},
				AllowedActionKinds: []domain.CandidateActionKind{
					domain.CandidateActionKindLong,
					domain.CandidateActionKindShort,
				},
				MinimumQuality:       domain.DataQualityValidated,
				MaximumApprovedCount: 1,
			},
			Quantity: 1,
		})
		require.NoError(t, err)

		mode := domain.DecisionModeBacktest
		require.Len(t, result.PaperExecutions, 1)
		paperExecution := result.PaperExecutions[0]
		require.NotNil(t, paperExecution.Fill)

		intents, err := auditService.ListOrderIntents(t.Context(), audit.OrderIntentQuery{
			StrategyID: result.BacktestRun.StrategyID,
			Instrument: &instrument,
			Mode:       &mode,
			TimeRange:  &requestRange,
		})
		require.NoError(t, err)
		require.Len(t, intents, 2)

		traces, err := auditService.ListTraces(t.Context(), audit.TraceQuery{
			StrategyID: result.BacktestRun.StrategyID,
			Instrument: &instrument,
			Mode:       &mode,
			TimeRange:  &requestRange,
		})
		require.NoError(t, err)
		require.Len(t, traces, 2)
		require.Equal(t, requestStart.Add(4*time.Minute).UTC(), traces[0].DecisionTime.Time())
		require.Equal(t, requestStart.Add(6*time.Minute).UTC(), traces[1].DecisionTime.Time())
		require.Equal(t, result.DatasetReference.DatasetID.String(), traces[0].DatasetReference)
		require.Equal(t, result.BacktestRun.RunID.String(), traces[0].RunReference)
		require.Equal(t, string(intents[0].IntentID), traces[0].Metadata["intent_id"])
		require.NotEmpty(t, traces[0].Metadata["governor_decision_reference"])
		require.Equal(t, domain.GovernorDecisionStatusApproved.String(), traces[0].Metadata["governor_decision_status"])
		require.Equal(t, string(paperExecution.Command.CommandID), traces[0].Metadata["execution_command_id"])
		require.Equal(t, string(paperExecution.Order.OrderID), traces[0].Metadata["execution_order_id"])
		require.Equal(t, string(paperExecution.Fill.FillID), traces[0].Metadata["execution_fill_id"])
		require.Equal(t, result.PositionSnapshots[0].SnapshotID.String(), traces[0].Metadata["position_snapshot_id"])
		require.Equal(t, result.PortfolioSnapshots[0].SnapshotID.String(), traces[0].Metadata["portfolio_snapshot_id"])
		require.Equal(t, result.EvaluationReport.EvaluationID.String(), traces[0].Metadata["evaluation_report_id"])
		require.Equal(t, domain.GovernorDecisionStatusBlocked.String(), traces[1].Metadata["governor_decision_status"])
		require.Equal(t, result.EvaluationReport.EvaluationID.String(), traces[1].Metadata["evaluation_report_id"])

		require.Equal(t, traces[0].TraceID, intents[0].TraceID)
		require.Equal(t, traces[1].TraceID, intents[1].TraceID)
		require.Len(t, result.IntentContexts, 2)
		require.Equal(t, traces[0], result.IntentContexts[0].Trace)
		require.Equal(t, intents[0], result.IntentContexts[0].Intent)
		require.Equal(t, firstAction, result.IntentContexts[0].CandidateAction)
		require.Equal(t, traces[1], result.IntentContexts[1].Trace)
		require.Equal(t, intents[1], result.IntentContexts[1].Intent)
		require.Equal(t, secondAction, result.IntentContexts[1].CandidateAction)
		require.Equal(t, domain.OrderIntentStatusExecutionCreated, intents[0].Status)
		require.Equal(t, domain.OrderIntentStatusBlocked, intents[1].Status)
		require.Equal(
			t,
			traces[0].Metadata["governor_decision_reference"],
			intents[0].Metadata["governor_decision_reference"],
		)
		require.Equal(t, string(paperExecution.Command.CommandID), intents[0].Metadata["execution_command_id"])
		require.Equal(t, string(paperExecution.Order.OrderID), intents[0].Metadata["execution_order_id"])
		require.Equal(t, string(paperExecution.Fill.FillID), intents[0].Metadata["execution_fill_id"])
		require.Equal(t, result.PositionSnapshots[0].SnapshotID.String(), intents[0].Metadata["position_snapshot_id"])
		require.Equal(t, result.PortfolioSnapshots[0].SnapshotID.String(), intents[0].Metadata["portfolio_snapshot_id"])
		require.Equal(t, result.EvaluationReport.EvaluationID.String(), intents[0].Metadata["evaluation_report_id"])
		require.Equal(t, domain.GovernorDecisionStatusBlocked.String(), intents[1].Metadata["governor_decision_status"])
		require.Equal(t, result.EvaluationReport.EvaluationID.String(), intents[1].Metadata["evaluation_report_id"])

		require.Len(t, result.GovernorEvaluation.Decisions, 2)
		require.Equal(t, domain.GovernorDecisionStatusApproved, result.GovernorEvaluation.Decisions[0].Status)
		require.Equal(t, domain.GovernorDecisionStatusBlocked, result.GovernorEvaluation.Decisions[1].Status)

		require.Equal(t, traces[0].TraceID, paperExecution.Command.TraceID)
		require.Equal(t, intents[0].IntentID, paperExecution.Command.IntentID)
		require.Equal(t, paperExecution.Command.CommandID, paperExecution.Order.Command.CommandID)
		require.Equal(t, paperExecution.Order.OrderID, paperExecution.Fill.Order.OrderID)
		require.Equal(t, domain.ExecutionOrderStatusFilled, paperExecution.Reconciliation.Status)

		persistedFills, err := executionStore.ListFillsByOrder(t.Context(), string(paperExecution.Order.OrderID))
		require.NoError(t, err)
		require.Len(t, persistedFills, 1)
		require.Equal(t, paperExecution.Fill.FillID, persistedFills[0].FillID)
		require.Equal(t, paperExecution.Fill.Order.OrderID, persistedFills[0].Order.OrderID)

		require.Len(t, result.PositionSnapshots, 1)
		require.Len(t, result.PortfolioSnapshots, 1)
		require.Equal(t, paperExecution.Fill.FillID, result.PositionSnapshots[0].SourceFillID)
		require.Equal(t, paperExecution.Fill.FillID, result.PortfolioSnapshots[0].SourceFillID)

		positionSnapshots, err := snapshotProjector.QueryPositionSnapshots(t.Context(), execution.PositionSnapshotQuery{
			StrategyID: result.BacktestRun.StrategyID,
			Instrument: &instrument,
			Mode:       &mode,
			TimeRange:  &requestRange,
		})
		require.NoError(t, err)
		require.Equal(t, result.PositionSnapshots, positionSnapshots)

		portfolioSnapshots, err := snapshotProjector.QueryPortfolioSnapshots(
			t.Context(),
			execution.PortfolioSnapshotQuery{
				Mode:      &mode,
				TimeRange: &requestRange,
			},
		)
		require.NoError(t, err)
		require.Equal(t, result.PortfolioSnapshots, portfolioSnapshots)

		runs, err := backtestService.QueryBacktestRuns(t.Context(), backtest.RunQuery{
			StrategyID: result.BacktestRun.StrategyID,
			DatasetID:  result.DatasetReference.DatasetID.String(),
		})
		require.NoError(t, err)
		require.Equal(t, []domain.BacktestRun{result.BacktestRun}, runs)
		require.Equal(t, result.DatasetReference.DatasetID, result.BacktestRun.DatasetID)
		require.Equal(t, domain.BacktestRunStatusCompleted, result.BacktestRun.Status)

		reports, err := backtestService.QueryEvaluationReports(t.Context(), backtest.EvaluationReportQuery{
			StrategyID: result.BacktestRun.StrategyID,
			BacktestID: result.BacktestRun.RunID.String(),
			DatasetID:  result.DatasetReference.DatasetID.String(),
		})
		require.NoError(t, err)
		require.Equal(t, []domain.EvaluationReport{result.EvaluationReport}, reports)
		require.Equal(t, result.BacktestRun.RunID, result.EvaluationReport.BacktestRunID)
		require.Equal(t, result.DatasetReference.DatasetID, result.EvaluationReport.DatasetID)
		require.Equal(t, domain.EvaluationDecisionNeedsReview, result.EvaluationReport.Decision)
		require.Equal(
			t,
			[]string{domain.GovernorDecisionReasonApprovalLimitReached.String()},
			result.EvaluationReport.FailureReasons,
		)
		require.NotNil(t, result.EvaluationReport.Metrics)
		require.NotNil(t, result.EvaluationReport.Metrics.TradeCount)
		require.Equal(t, 1, *result.EvaluationReport.Metrics.TradeCount)
		require.NotNil(t, result.EvaluationReport.Metrics.BlockedGovernorDecisionCount)
		require.Equal(t, 1, *result.EvaluationReport.Metrics.BlockedGovernorDecisionCount)
		require.NotNil(t, result.EvaluationReport.Metrics.RejectedGovernorDecisionCount)
		require.Equal(t, 0, *result.EvaluationReport.Metrics.RejectedGovernorDecisionCount)
		require.Equal(t, result.EvaluationReport.Metrics, result.BacktestRun.Metrics)
	})

	t.Run("buildDatasetReference is input scoped across run ids", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestStart := time.Date(2026, 4, 6, 9, 0, 0, 0, time.FixedZone(randomWord(t, fake, "zone"), 0))
		requestRange, err := domain.NewTimeRange(requestStart, requestStart.Add(4*time.Minute))
		require.NoError(t, err)

		replayed := makeReplayCandles(t, fake, instrument, requestStart, []float64{10, 11, 12, 13})
		firstRequest, err := canonicalizePaperBacktestRequest(PaperBacktestRequest{
			RunID:                randomWord(t, fake, "run-a"),
			Mode:                 domain.DecisionModeBacktest,
			StrategyID:           randomWord(t, fake, "strategy-id"),
			StrategyVersion:      randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash: randomWord(t, fake, "strategy-artifact"),
			Instrument:           instrument,
			Timeframe:            domain.Timeframe1m,
			TimeRange:            requestRange,
			StrategyParameters: strategy.MovingAverageCrossoverParams{
				FastWindow: 2,
				SlowWindow: 3,
			},
			GovernorPolicy: governor.Policy{
				AllowedModes:       []domain.DecisionMode{domain.DecisionModeBacktest},
				AllowedActionKinds: []domain.CandidateActionKind{domain.CandidateActionKindLong},
				MinimumQuality:     domain.DataQualityValidated,
			},
			Quantity: 1,
		})
		require.NoError(t, err)

		secondRequest := firstRequest
		secondRequest.runID = randomWord(t, fake, "run-b")

		firstDataset, err := buildDatasetReference(firstRequest, replayed)
		require.NoError(t, err)
		secondDataset, err := buildDatasetReference(secondRequest, replayed)
		require.NoError(t, err)

		require.Equal(t, firstDataset.DatasetID, secondDataset.DatasetID)
		require.Equal(t, firstDataset.ReplayChecksum, secondDataset.ReplayChecksum)
		require.Equal(t, map[string]string{}, firstDataset.Metadata)
		require.Equal(t, firstDataset.Metadata, secondDataset.Metadata)
	})

	t.Run("reuses dataset references across repeated runs with identical replay inputs", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		requestStart := time.Date(2026, 4, 7, 9, 0, 0, 0, time.FixedZone(randomWord(t, fake, "zone"), 0))
		requestRange, err := domain.NewTimeRange(requestStart, requestStart.Add(8*time.Minute))
		require.NoError(t, err)

		replayed := makeReplayCandles(t, fake, instrument, requestStart, []float64{10, 11, 10, 9, 11, 12, 9, 8})
		firstAction := makeAction(t, instrument, requestStart.Add(4*time.Minute), domain.CandidateActionKindLong)
		secondAction := makeAction(t, instrument, requestStart.Add(6*time.Minute), domain.CandidateActionKindShort)

		replayReader := &fakeCandleReplayReader{result: replayed}
		analyticsCalc := &fakeAnalyticsCalculator{}
		strategyEvaluator := &fakeStrategyEvaluator{result: strategy.EvaluateResult{
			Strategy: domain.StrategyIdentity{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			},
			TimeRange: requestRange,
			Parameters: strategy.MovingAverageCrossoverParams{
				FastWindow: 2,
				SlowWindow: 3,
			},
			Actions: []domain.CandidateAction{firstAction, secondAction},
		}}

		auditStore, err := audit.NewDatabaseStore(":memory:", audit.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, auditStore.AutoMigrate())
		auditService, err := audit.NewService(auditStore)
		require.NoError(t, err)

		executionStore, err := execution.NewDatabaseStore(":memory:", execution.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, executionStore.AutoMigrate())
		paperExecutor, err := execution.NewPaperService(executionStore)
		require.NoError(t, err)
		snapshotProjector, err := execution.NewSnapshotService(executionStore)
		require.NoError(t, err)

		backtestStore, err := backtest.NewDatabaseStore(":memory:", backtest.DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, backtestStore.AutoMigrate())
		backtestService, err := backtest.NewService(backtestStore)
		require.NoError(t, err)

		flow, err := NewDurableBacktestFlow(DurableBacktestFlowDeps{
			CandleReplayReader:  replayReader,
			AnalyticsCalculator: analyticsCalc,
			StrategyEvaluator:   strategyEvaluator,
			AuditRecorder:       auditService,
			GovernorEvaluator:   governor.NewService(),
			PaperExecutor:       paperExecutor,
			SnapshotProjector:   snapshotProjector,
			BacktestRecorder:    backtestService,
		})
		require.NoError(t, err)

		makeRequest := func(runID string) PaperBacktestRequest {
			t.Helper()

			return PaperBacktestRequest{
				RunID:                runID,
				Mode:                 domain.DecisionModeBacktest,
				StrategyID:           randomWord(t, fake, "strategy-id"),
				StrategyVersion:      randomWord(t, fake, "strategy-version"),
				StrategyArtifactHash: randomWord(t, fake, "strategy-artifact"),
				Instrument:           instrument,
				Timeframe:            domain.Timeframe1m,
				TimeRange:            requestRange,
				StrategyParameters: strategy.MovingAverageCrossoverParams{
					FastWindow: 2,
					SlowWindow: 3,
				},
				GovernorPolicy: governor.Policy{
					AllowedModes: []domain.DecisionMode{domain.DecisionModeBacktest},
					AllowedActionKinds: []domain.CandidateActionKind{
						domain.CandidateActionKindLong,
						domain.CandidateActionKindShort,
					},
					MinimumQuality:       domain.DataQualityValidated,
					MaximumApprovedCount: 1,
				},
				Quantity: 1,
			}
		}

		firstResult, err := flow.Run(t.Context(), makeRequest(randomWord(t, fake, "run-a")))
		require.NoError(t, err)

		secondResult, err := flow.Run(t.Context(), makeRequest(randomWord(t, fake, "run-b")))
		require.NoError(t, err)
		require.Equal(t, firstResult.DatasetReference, secondResult.DatasetReference)
		require.NotEqual(t, firstResult.BacktestRun.RunID, secondResult.BacktestRun.RunID)

		runs, err := backtestService.QueryBacktestRuns(t.Context(), backtest.RunQuery{
			DatasetID: firstResult.DatasetReference.DatasetID.String(),
		})
		require.NoError(t, err)
		require.Len(t, runs, 2)
	})
}
