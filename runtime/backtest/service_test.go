package backtest

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type sqliteBacktestIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteBacktestIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteBacktestTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestService(t *testing.T) {
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

	randomTime := func(t *testing.T, fake faker.Faker) time.Time {
		t.Helper()

		return time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone("", fake.IntBetween(-11, 12)*3600),
		)
	}

	makeStore := func(t *testing.T, dsn string, tablePrefix string) *DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewDatabaseStore(sqlDB, dsn, DatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeService := func(t *testing.T) (*Service, *DatabaseStore) {
		t.Helper()

		store := makeStore(t, ":memory:", "")
		svc, err := NewService(store)
		require.NoError(t, err)

		return svc, store
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

	makeDatasetReference := func(t *testing.T, fake faker.Faker, createdAt time.Time) domain.DatasetReference {
		t.Helper()

		reference, err := domain.NewDatasetReference(domain.DatasetReferenceParams{
			DatasetID:   randomWord(t, fake, "dataset"),
			EntityTypes: []string{"candles", "candles", "analytics"},
			Instruments: []domain.Instrument{makeInstrument(t, fake), makeInstrument(t, fake)},
			Timeframes:  []domain.Timeframe{domain.Timeframe1m, domain.Timeframe5m, domain.Timeframe1m},
			TimeRange: domain.TimeRange{
				Start: createdAt.Add(-24 * time.Hour),
				End:   createdAt.Add(-12 * time.Hour),
			},
			SourceDataHashes: []string{randomWord(t, fake, "hash-b"), randomWord(t, fake, "hash-a")},
			CreatedAt:        createdAt,
			Metadata:         map[string]string{"assumption": randomWord(t, fake, "metadata")},
		})
		require.NoError(t, err)

		return reference
	}

	makeMetrics := func(tradeCount int) *domain.VersionedMetrics {
		metrics, err := domain.NewVersionedMetrics(domain.VersionedMetricsParams{
			SchemaVersion: "backtest-run.v1",
			TradeCount:    &tradeCount,
		})
		require.NoError(t, err)

		return metrics
	}

	makeBacktestRun := func(
		t *testing.T,
		fake faker.Faker,
		datasetID string,
		status domain.BacktestRunStatus,
		createdAt time.Time,
		updatedAt time.Time,
	) domain.BacktestRun {
		t.Helper()

		run, err := domain.NewBacktestRun(domain.BacktestRunParams{
			RunID:                 randomWord(t, fake, "run"),
			StrategyID:            randomWord(t, fake, "strategy"),
			StrategyVersion:       randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash:  randomWord(t, fake, "strategy-hash"),
			DatasetID:             datasetID,
			GovernorPolicyID:      randomWord(t, fake, "policy"),
			GovernorPolicyVersion: randomWord(t, fake, "policy-version"),
			GovernorPolicyHash:    randomWord(t, fake, "policy-hash"),
			Mode:                  domain.DecisionModeBacktest,
			TestedRange: domain.TimeRange{
				Start: createdAt.Add(-6 * time.Hour),
				End:   createdAt.Add(-time.Hour),
			},
			FeeModelID: randomWord(t, fake, "fee-model"),
			SlippageAssumptions: map[string]string{
				"slippage": randomWord(t, fake, "slippage"),
			},
			SlippageModelID:           "",
			FeeAssumptions:            nil,
			ExecutionSimulatorVersion: randomWord(t, fake, "simulator"),
			Status:                    status,
			Metrics:                   nil,
			CreatedAt:                 createdAt,
			UpdatedAt:                 updatedAt,
		})
		require.NoError(t, err)

		return run
	}

	makeAction := func(t *testing.T, instrument domain.Instrument, decisionTime time.Time) domain.CandidateAction {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy: domain.StrategyIdentity{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			},
			Kind:         domain.CandidateActionKindLong,
			DecisionTime: decisionTime,
			InputRange:   domain.TimeRange{Start: decisionTime.Add(-time.Minute), End: decisionTime},
			Quality:      domain.DataQualityValidated,
		})
		require.NoError(t, err)

		return action
	}

	makeDecision := func(
		t *testing.T,
		instrument domain.Instrument,
		status domain.GovernorDecisionStatus,
		decisionTime time.Time,
	) domain.GovernorDecision {
		t.Helper()

		reason := domain.GovernorDecisionReasonOK
		if status == domain.GovernorDecisionStatusBlocked {
			reason = domain.GovernorDecisionReasonKillSwitchActive
		}
		if status == domain.GovernorDecisionStatusRejected {
			reason = domain.GovernorDecisionReasonModeNotAllowed
		}

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: makeAction(t, instrument, decisionTime.Add(-time.Minute)),
			Status:          status,
			Reason:          reason,
			DecisionTime:    decisionTime,
		})
		require.NoError(t, err)

		return decision
	}

	makePortfolioSnapshot := func(t *testing.T, fillID string, eventTime time.Time, realized float64, unrealized *float64) domain.PortfolioSnapshot {
		t.Helper()

		snapshot, err := domain.NewPortfolioSnapshot(domain.PortfolioSnapshotParams{
			SnapshotID:    "portfolio-" + fillID,
			SourceFillID:  fillID,
			Mode:          domain.DecisionModeBacktest,
			GrossExposure: 1,
			NetExposure:   1,
			RealizedPnL:   realized,
			UnrealizedPnL: unrealized,
			EventTime:     eventTime,
			Metadata:      map[string]string{"projection": "fill-ledger-v0"},
		})
		require.NoError(t, err)

		return snapshot
	}

	makeFill := func(t *testing.T, fake faker.Faker, instrument domain.Instrument, eventTime time.Time) domain.ExecutionFill {
		t.Helper()

		limitPrice := 100.0
		decision := makeDecision(t, instrument, domain.GovernorDecisionStatusApproved, eventTime.Add(-2*time.Minute))
		command, err := domain.NewExecutionCommand(domain.ExecutionCommandParams{
			CommandID:                 randomWord(t, fake, "command"),
			Mode:                      domain.DecisionModeBacktest,
			StrategyID:                randomWord(t, fake, "strategy"),
			StrategyVersion:           randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash:      randomWord(t, fake, "strategy-hash"),
			Venue:                     instrument.Venue,
			Instrument:                instrument,
			ActionKind:                domain.CandidateActionKindLong,
			OrderType:                 domain.OrderTypeLimit,
			LimitPrice:                &limitPrice,
			GovernorDecisionReference: randomWord(t, fake, "decision-ref"),
			ApprovedDecision:          decision,
			Status:                    domain.ExecutionCommandStatusCreated,
			Quantity:                  1,
			Notional:                  100,
			EventTime:                 eventTime.Add(-time.Minute),
		})
		require.NoError(t, err)

		order, err := domain.NewExecutionOrder(domain.ExecutionOrderParams{
			OrderID:              randomWord(t, fake, "order"),
			Command:              command,
			Mode:                 domain.DecisionModeBacktest,
			StrategyID:           command.StrategyID,
			StrategyVersion:      command.StrategyVersion,
			StrategyArtifactHash: command.StrategyArtifactHash,
			Venue:                instrument.Venue,
			Instrument:           instrument,
			OrderType:            domain.OrderTypeLimit,
			TimeInForce:          domain.TimeInForceGTC,
			ClientOrderID:        randomWord(t, fake, "client-order"),
			Status:               domain.ExecutionOrderStatusFilled,
			Quantity:             1,
			Notional:             100,
			LimitPrice:           &limitPrice,
			EventTime:            eventTime.Add(-time.Minute),
		})
		require.NoError(t, err)

		fill, err := domain.NewExecutionFill(domain.ExecutionFillParams{
			FillID:                    randomWord(t, fake, "fill"),
			Order:                     order,
			SourceMarketDataReference: randomWord(t, fake, "source"),
			FeeAmount:                 0,
			SlippageAmount:            0,
			Metadata:                  map[string]string{"simulator": "closed-candle-limit-v0"},
			Quantity:                  1,
			Price:                     100,
			EventTime:                 eventTime,
		})
		require.NoError(t, err)

		return fill
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteBacktestIndexListRow
		require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteBacktestIndexInfoRow
			require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error)

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

	t.Run("persists compact dataset references and backtest lifecycle records", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		createdAt := randomTime(t, fake)

		reference := makeDatasetReference(t, fake, createdAt)
		persistedReference, err := svc.CreateDatasetReference(t.Context(), reference)
		require.NoError(t, err)
		require.True(t, createdAt.Equal(persistedReference.CreatedAt.Time()))
		require.NotEmpty(t, persistedReference.SourceDataHashes)
		require.Empty(t, persistedReference.ReplayChecksum)

		run := makeBacktestRun(
			t,
			fake,
			persistedReference.DatasetID.String(),
			domain.BacktestRunStatusPending,
			createdAt.Add(time.Minute),
			createdAt.Add(time.Minute),
		)
		persistedRun, err := svc.CreateBacktestRun(t.Context(), run)
		require.NoError(t, err)
		require.Equal(t, domain.BacktestRunStatusPending, persistedRun.Status)

		started, err := svc.StartBacktestRun(
			t.Context(),
			persistedRun.RunID.String(),
			domain.BacktestRunTime(createdAt.Add(2*time.Minute)),
		)
		require.NoError(t, err)
		require.Equal(t, domain.BacktestRunStatusRunning, started.Status)
		require.Equal(t, persistedRun.DatasetID, started.DatasetID)
		require.Equal(t, persistedRun.StrategyArtifactHash, started.StrategyArtifactHash)

		completed, err := svc.CompleteBacktestRun(t.Context(), CompleteBacktestRunRequest{
			RunID:   started.RunID.String(),
			Metrics: makeMetrics(3),
			EndedAt: domain.BacktestRunTime(createdAt.Add(3 * time.Minute)),
		})
		require.NoError(t, err)
		require.Equal(t, domain.BacktestRunStatusCompleted, completed.Status)
		require.NotNil(t, completed.Metrics)
		require.Equal(t, "backtest-run.v1", completed.Metrics.SchemaVersion)
		require.Equal(t, persistedRun.DatasetID, completed.DatasetID)
		require.Equal(t, persistedRun.GovernorPolicyHash, completed.GovernorPolicyHash)
		require.True(t, completed.UpdatedAt.Time().After(completed.CreatedAt.Time()))

		failedRun := makeBacktestRun(
			t,
			fake,
			persistedReference.DatasetID.String(),
			domain.BacktestRunStatusPending,
			createdAt.Add(4*time.Minute),
			createdAt.Add(4*time.Minute),
		)
		failedPersisted, err := svc.CreateBacktestRun(t.Context(), failedRun)
		require.NoError(t, err)

		failed, err := svc.FailBacktestRun(t.Context(), FailBacktestRunRequest{
			RunID:          failedPersisted.RunID.String(),
			FailureReason:  "REPLAY_DATA_MISSING",
			FailureDetails: randomWord(t, fake, "failure-detail"),
			EndedAt:        domain.BacktestRunTime(createdAt.Add(5 * time.Minute)),
		})
		require.NoError(t, err)
		require.Equal(t, domain.BacktestRunStatusFailed, failed.Status)
		require.Equal(t, "REPLAY_DATA_MISSING", failed.FailureReason)
		require.NotEmpty(t, failed.FailureDetails)

		runs, err := svc.QueryBacktestRuns(t.Context(), RunQuery{
			StrategyID: completed.StrategyID,
			DatasetID:  completed.DatasetID.String(),
			Status:     &completed.Status,
		})
		require.NoError(t, err)
		require.Equal(t, []domain.BacktestRun{completed}, runs)

		require.Equal(t, int64(1), readCount(t, store, "dataset_references"))
		require.Equal(t, int64(2), readCount(t, store, "backtest_runs"))
	})

	t.Run("reuses existing dataset reference when dataset id already exists", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		createdAt := randomTime(t, fake)

		reference := makeDatasetReference(t, fake, createdAt)
		persistedReference, err := svc.CreateDatasetReference(t.Context(), reference)
		require.NoError(t, err)

		duplicateReference, err := svc.CreateDatasetReference(t.Context(), reference)
		require.NoError(t, err)
		require.Equal(t, persistedReference, duplicateReference)
		require.Equal(t, int64(1), readCount(t, store, "dataset_references"))
	})

	t.Run("rejects unsupported backtest lifecycle transitions", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		createdAt := randomTime(t, fake)

		reference, err := svc.CreateDatasetReference(t.Context(), makeDatasetReference(t, fake, createdAt))
		require.NoError(t, err)

		run, err := svc.CreateBacktestRun(t.Context(), makeBacktestRun(
			t,
			fake,
			reference.DatasetID.String(),
			domain.BacktestRunStatusPending,
			createdAt.Add(time.Minute),
			createdAt.Add(time.Minute),
		))
		require.NoError(t, err)

		_, err = svc.CompleteBacktestRun(t.Context(), CompleteBacktestRunRequest{
			RunID:   run.RunID.String(),
			Metrics: makeMetrics(1),
			EndedAt: domain.BacktestRunTime(createdAt.Add(2 * time.Minute)),
		})
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "pending")
		require.ErrorContains(t, err, "completed")
	})

	t.Run("persists compact evaluation reports with derivable metrics only", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		createdAt := randomTime(t, fake)
		instrument := makeInstrument(t, fake)

		reference, err := svc.CreateDatasetReference(t.Context(), makeDatasetReference(t, fake, createdAt))
		require.NoError(t, err)
		run, err := svc.CreateBacktestRun(t.Context(), makeBacktestRun(
			t,
			fake,
			reference.DatasetID.String(),
			domain.BacktestRunStatusPending,
			createdAt.Add(time.Minute),
			createdAt.Add(time.Minute),
		))
		require.NoError(t, err)

		fillA := makeFill(t, fake, instrument, createdAt.Add(2*time.Minute))
		fillB := makeFill(t, fake, instrument, createdAt.Add(3*time.Minute))
		unrealized := -3.0
		report, err := svc.CreateEvaluationReport(t.Context(), CreateEvaluationReportRequest{
			EvaluationID:         randomWord(t, fake, "evaluation"),
			StrategyID:           run.StrategyID,
			StrategyVersion:      run.StrategyVersion,
			StrategyArtifactHash: run.StrategyArtifactHash,
			BacktestRunID:        run.RunID.String(),
			DatasetID:            reference.DatasetID.String(),
			Decision:             domain.EvaluationDecisionNeedsReview,
			FailureReasons:       []string{"DATA_QUALITY_WARNING", "REPLAY_GAP"},
			Notes:                randomWord(t, fake, "note"),
			CreatedAt:            domain.EvaluationReportTime(createdAt.Add(4 * time.Minute)),
			Fills:                []domain.ExecutionFill{fillA, fillB},
			GovernorDecisions: []domain.GovernorDecision{
				makeDecision(t, instrument, domain.GovernorDecisionStatusBlocked, createdAt.Add(2*time.Minute)),
				makeDecision(t, instrument, domain.GovernorDecisionStatusRejected, createdAt.Add(3*time.Minute)),
			},
			PortfolioSnapshots: []domain.PortfolioSnapshot{
				makePortfolioSnapshot(t, string(fillA.FillID), createdAt.Add(2*time.Minute), 5, nil),
				makePortfolioSnapshot(t, string(fillB.FillID), createdAt.Add(3*time.Minute), 1, &unrealized),
			},
		})
		require.NoError(t, err)
		require.Equal(t, run.RunID, report.BacktestRunID)
		require.Equal(t, reference.DatasetID, report.DatasetID)
		require.Equal(t, []string{"DATA_QUALITY_WARNING", "REPLAY_GAP"}, report.FailureReasons)
		require.NotNil(t, report.Metrics)
		require.Equal(t, "evaluation-report.v1", report.Metrics.SchemaVersion)
		require.NotNil(t, report.Metrics.TradeCount)
		require.Equal(t, 2, *report.Metrics.TradeCount)
		require.NotNil(t, report.Metrics.BlockedGovernorDecisionCount)
		require.Equal(t, 1, *report.Metrics.BlockedGovernorDecisionCount)
		require.NotNil(t, report.Metrics.RejectedGovernorDecisionCount)
		require.Equal(t, 1, *report.Metrics.RejectedGovernorDecisionCount)
		require.NotNil(t, report.Metrics.MaxDrawdown)
		require.InDelta(t, 7, *report.Metrics.MaxDrawdown, 1e-9)

		minimalReport, err := svc.CreateEvaluationReport(t.Context(), CreateEvaluationReportRequest{
			EvaluationID:         randomWord(t, fake, "evaluation-minimal"),
			StrategyID:           run.StrategyID,
			StrategyVersion:      run.StrategyVersion,
			StrategyArtifactHash: run.StrategyArtifactHash,
			BacktestRunID:        run.RunID.String(),
			DatasetID:            reference.DatasetID.String(),
			Decision:             domain.EvaluationDecisionReject,
			CreatedAt:            domain.EvaluationReportTime(createdAt.Add(5 * time.Minute)),
		})
		require.NoError(t, err)
		require.NotNil(t, minimalReport.Metrics)
		require.Equal(t, "evaluation-report.v1", minimalReport.Metrics.SchemaVersion)
		require.Nil(t, minimalReport.Metrics.TradeCount)
		require.Nil(t, minimalReport.Metrics.BlockedGovernorDecisionCount)
		require.Nil(t, minimalReport.Metrics.RejectedGovernorDecisionCount)
		require.Nil(t, minimalReport.Metrics.MaxDrawdown)

		reports, err := svc.QueryEvaluationReports(t.Context(), EvaluationReportQuery{
			BacktestID: run.RunID.String(),
			DatasetID:  reference.DatasetID.String(),
		})
		require.NoError(t, err)
		require.Equal(t, []domain.EvaluationReport{report, minimalReport}, reports)

		require.Equal(t, int64(2), readCount(t, store, "evaluation_reports"))
	})

	t.Run("filters and orders canonical runs and reports deterministically", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		_, store := makeService(t)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))
		datasetID := randomWord(t, fake, "dataset")

		earlierRun, err := store.CreateBacktestRun(
			t.Context(),
			makeBacktestRun(t, fake, datasetID, domain.BacktestRunStatusPending, earlier, earlier),
		)
		require.NoError(t, err)
		laterRun, err := store.CreateBacktestRun(
			t.Context(),
			makeBacktestRun(t, fake, datasetID, domain.BacktestRunStatusPending, later, later),
		)
		require.NoError(t, err)

		makeReport := func(run domain.BacktestRun, createdAt time.Time, suffix string) domain.EvaluationReport {
			t.Helper()
			report, reportErr := domain.NewEvaluationReport(domain.EvaluationReportParams{
				EvaluationID:         randomWord(t, fake, "evaluation-"+suffix),
				StrategyID:           run.StrategyID,
				StrategyVersion:      run.StrategyVersion,
				StrategyArtifactHash: run.StrategyArtifactHash,
				BacktestRunID:        run.RunID.String(),
				DatasetID:            run.DatasetID.String(),
				Decision:             domain.EvaluationDecisionNeedsReview,
				Notes:                randomWord(t, fake, "note-"+suffix),
				CreatedAt:            createdAt,
			})
			require.NoError(t, reportErr)
			return report
		}
		earlierReport, err := store.CreateEvaluationReport(t.Context(), makeReport(earlierRun, earlier, "earlier"))
		require.NoError(t, err)
		laterReport, err := store.CreateEvaluationReport(t.Context(), makeReport(laterRun, later, "later"))
		require.NoError(t, err)
		runs, err := store.QueryBacktestRuns(t.Context(), RunQuery{})
		require.NoError(t, err)
		require.Equal(t, []domain.BacktestRunID{earlierRun.RunID, laterRun.RunID}, []domain.BacktestRunID{
			runs[0].RunID,
			runs[1].RunID,
		})
		reports, err := store.QueryEvaluationReports(t.Context(), EvaluationReportQuery{})
		require.NoError(t, err)
		require.Equal(
			t,
			[]domain.EvaluationReportID{earlierReport.EvaluationID, laterReport.EvaluationID},
			[]domain.EvaluationReportID{reports[0].EvaluationID, reports[1].EvaluationID},
		)

		boundaryRange, err := domain.NewTimeRange(earlier.Add(-time.Minute), earlier.Add(time.Minute))
		require.NoError(t, err)
		runs, err = store.QueryBacktestRuns(t.Context(), RunQuery{TimeRange: &boundaryRange})
		require.NoError(t, err)
		require.Len(t, runs, 1)
		require.Equal(t, earlierRun.RunID, runs[0].RunID)
		reports, err = store.QueryEvaluationReports(t.Context(), EvaluationReportQuery{TimeRange: &boundaryRange})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		require.Equal(t, earlierReport.EvaluationID, reports[0].EvaluationID)

		mixedOffsetAt := time.Date(2026, time.January, 1, 0, 0, 0, 789, time.FixedZone("east", 2*60*60))
		mixedRun, err := store.CreateBacktestRun(
			t.Context(),
			makeBacktestRun(t, fake, datasetID, domain.BacktestRunStatusPending, mixedOffsetAt, mixedOffsetAt),
		)
		require.NoError(t, err)
		mixedReport, err := store.CreateEvaluationReport(
			t.Context(),
			makeReport(mixedRun, mixedOffsetAt, "mixed-offset"),
		)
		require.NoError(t, err)
		instantRange, err := domain.NewTimeRange(
			time.Date(2025, time.December, 31, 21, 30, 0, 0, time.UTC),
			time.Date(2025, time.December, 31, 22, 30, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		runs, err = store.QueryBacktestRuns(t.Context(), RunQuery{TimeRange: &instantRange})
		require.NoError(t, err)
		require.Equal(t, []domain.BacktestRunID{mixedRun.RunID}, []domain.BacktestRunID{runs[0].RunID})
		reports, err = store.QueryEvaluationReports(t.Context(), EvaluationReportQuery{TimeRange: &instantRange})
		require.NoError(t, err)
		require.Equal(
			t,
			[]domain.EvaluationReportID{mixedReport.EvaluationID},
			[]domain.EvaluationReportID{reports[0].EvaluationID},
		)
	})

	t.Run("migrates explicit sqlite backtest schema", func(t *testing.T) {
		t.Parallel()

		store := makeStore(t, ":memory:", "")

		for _, tableName := range []string{"dataset_references", "backtest_runs", "evaluation_reports"} {
			var columns []sqliteBacktestTableInfoRow
			require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA table_info('%s')", tableName)).Scan(&columns).Error)
			require.NotEmpty(t, columns)
		}
		require.True(t, store.db.Migrator().HasIndex(&backtestRunModel{}, "idx_backtest_runs_status_created_id"))
		require.True(
			t,
			store.db.Migrator().HasIndex(&evaluationReportModel{}, "idx_evaluation_reports_decision_created_id"),
		)

		require.True(t, hasUniqueIndexWithColumns(t, store, "dataset_references", []string{"dataset_id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, "backtest_runs", []string{"run_id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, "evaluation_reports", []string{"evaluation_id"}))
	})
}
