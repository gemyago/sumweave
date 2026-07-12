package strategyassistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEvaluationWorkspaceService struct {
	createCalls   []app.CreateEvaluationParams
	listCalls     []app.ListEvaluationsParams
	getCalls      []string
	reportCalls   []string
	evidenceCalls []string

	createFunc   func(context.Context, app.CreateEvaluationParams) (*app.EvaluationDetail, error)
	listFunc     func(context.Context, app.ListEvaluationsParams) ([]app.EvaluationListItem, error)
	getFunc      func(context.Context, string) (*app.EvaluationDetail, error)
	reportFunc   func(context.Context, string) (*app.EvaluationReportView, error)
	evidenceFunc func(context.Context, string) (*app.EvaluationEvidenceView, error)
}

func (f *fakeEvaluationWorkspaceService) CreateEvaluation(
	ctx context.Context,
	params app.CreateEvaluationParams,
) (*app.EvaluationDetail, error) {
	f.createCalls = append(f.createCalls, params)
	if f.createFunc == nil {
		return nil, errors.New("unexpected create evaluation call")
	}
	return f.createFunc(ctx, params)
}

func (f *fakeEvaluationWorkspaceService) ListEvaluations(
	ctx context.Context,
	params app.ListEvaluationsParams,
) ([]app.EvaluationListItem, error) {
	f.listCalls = append(f.listCalls, params)
	if f.listFunc == nil {
		return nil, errors.New("unexpected list evaluations call")
	}
	return f.listFunc(ctx, params)
}

func (f *fakeEvaluationWorkspaceService) GetEvaluation(
	ctx context.Context,
	runID string,
) (*app.EvaluationDetail, error) {
	f.getCalls = append(f.getCalls, runID)
	if f.getFunc == nil {
		return nil, errors.New("unexpected get evaluation call")
	}
	return f.getFunc(ctx, runID)
}

func (f *fakeEvaluationWorkspaceService) GetEvaluationReport(
	ctx context.Context,
	runID string,
) (*app.EvaluationReportView, error) {
	f.reportCalls = append(f.reportCalls, runID)
	if f.reportFunc == nil {
		return nil, errors.New("unexpected get evaluation report call")
	}
	return f.reportFunc(ctx, runID)
}

func (f *fakeEvaluationWorkspaceService) GetEvaluationEvidence(
	ctx context.Context,
	runID string,
) (*app.EvaluationEvidenceView, error) {
	f.evidenceCalls = append(f.evidenceCalls, runID)
	if f.evidenceFunc == nil {
		return nil, errors.New("unexpected get evaluation evidence call")
	}
	return f.evidenceFunc(ctx, runID)
}

func TestEvaluationTools(t *testing.T) {
	fake := faker.New()
	makeInstrument := func() app.StrategyInstrumentInput {
		return app.StrategyInstrumentInput{
			Venue:      strategyAssistantSupportedDataVenue,
			Symbol:     "sym-" + strings.ToUpper(fake.Lorem().Word()),
			AssetClass: "crypto",
			Active:     true,
		}
	}
	makeMetrics := func() *app.EvaluationMetricSummary {
		tradeCount := 2
		maxDrawdown := 0.125
		return &app.EvaluationMetricSummary{TradeCount: &tradeCount, MaxDrawdown: &maxDrawdown}
	}
	makeMetadata := func() app.EvaluationAIRenderMetadata {
		return app.EvaluationAIRenderMetadata{
			RequestSourceType:   "human",
			StrategySourceType:  "demo",
			StrategySourceLabel: "Demo example",
			Note:                "note-" + fake.Lorem().Word(),
			EvidenceCounts: app.EvaluationEvidenceCounts{
				Traces:             2,
				OrderIntents:       2,
				GovernorDecisions:  2,
				ExecutionRecords:   2,
				PositionSnapshots:  0,
				PortfolioSnapshots: 1,
			},
		}
	}

	t.Run(
		"run backtest forwards limited saved-version inputs and preserves default policy behavior",
		func(t *testing.T) {
			start := time.Now().UTC().Truncate(time.Second)
			end := start.Add(6 * time.Hour)
			strategyID := "strategy-" + fake.Lorem().Word()
			service := &fakeEvaluationWorkspaceService{}
			service.createFunc = func(_ context.Context, params app.CreateEvaluationParams) (*app.EvaluationDetail, error) {
				assert.Equal(t, app.CreateEvaluationParams{
					StrategyID:      strategyID,
					StrategyVersion: "v1",
					Start:           start,
					End:             end,
					Quantity:        3.5,
					Note:            "operator note",
				}, params)
				return &app.EvaluationDetail{
					RunID:     "run-" + fake.UUID().V4(),
					Status:    "completed",
					CreatedAt: start,
					UpdatedAt: end,
				}, nil
			}

			response, err := newRunBacktestTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
				nil,
				RunBacktestRequest{
					StrategyID:      strategyID,
					StrategyVersion: "v1",
					Start:           start,
					End:             end,
					Quantity:        3.5,
					Note:            "operator note",
				},
			)
			require.NoError(t, err)
			require.NotNil(t, response.Run)
			assert.Nil(t, response.Error)
			assert.Len(t, service.createCalls, 1)
			assert.Empty(t, service.createCalls[0].GovernorPolicyHash)
			assert.Equal(t, strategyID, service.createCalls[0].StrategyID)
		},
	)

	t.Run("run backtest returns persisted data-unavailable failures honestly", func(t *testing.T) {
		start := time.Now().UTC().Truncate(time.Second)
		service := &fakeEvaluationWorkspaceService{}
		service.createFunc = func(_ context.Context, _ app.CreateEvaluationParams) (*app.EvaluationDetail, error) {
			return &app.EvaluationDetail{
				RunID:          "run-" + fake.UUID().V4(),
				Status:         "failed",
				FailureReason:  evaluationFailureReasonDataUnavailable,
				FailureDetails: "local candles missing",
				CreatedAt:      start,
				UpdatedAt:      start,
			}, nil
		}

		response, err := newRunBacktestTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
			nil,
			RunBacktestRequest{
				StrategyID:      "strategy-" + fake.Lorem().Word(),
				StrategyVersion: "v2",
				Start:           start,
				End:             start.Add(time.Hour),
				Quantity:        1,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Run)
		require.NotNil(t, response.Error)
		assert.Equal(t, toolErrorCodeDataUnavailable, response.Error.Code)
		assert.Equal(t, evaluationFailureReasonDataUnavailable, response.Run.FailureReason)
		assert.Contains(t, response.NextStepHint, "candle availability")
	})

	t.Run(
		"run backtest rejects unsaved candidates, non-ready versions, and missing artifacts deterministically",
		func(t *testing.T) {
			tool := newRunBacktestTool(RegisterDeps{EvaluationWorkspace: &fakeEvaluationWorkspaceService{}})

			unsaved, err := tool.Handler(
				nil,
				RunBacktestRequest{StrategyID: "strategy-" + fake.Lorem().Word()},
			)
			require.NoError(t, err)
			require.NotNil(t, unsaved.Error)
			assert.Equal(t, toolErrorCodeUnsavedVersion, unsaved.Error.Code)

			notReadyService := &fakeEvaluationWorkspaceService{}
			notReadyService.createFunc = func(_ context.Context, _ app.CreateEvaluationParams) (*app.EvaluationDetail, error) {
				return nil, app.NewErrInvalidInput("strategyVersion", "strategy version status must be ready")
			}
			notReady, err := newRunBacktestTool(RegisterDeps{EvaluationWorkspace: notReadyService}).Handler(
				nil,
				RunBacktestRequest{
					StrategyID:      "strategy-" + fake.Lorem().Word(),
					StrategyVersion: "draft",
					Start:           time.Now().UTC(),
					End:             time.Now().UTC().Add(time.Hour),
					Quantity:        1,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, notReady.Error)
			assert.Equal(t, toolErrorCodeNotReady, notReady.Error.Code)

			missingArtifactService := &fakeEvaluationWorkspaceService{}
			missingArtifactService.createFunc = func(_ context.Context, _ app.CreateEvaluationParams) (*app.EvaluationDetail, error) {
				return nil, app.NewErrNotFound("strategy artifact", "hash-"+fake.Lorem().Word())
			}
			missingArtifactTool := newRunBacktestTool(
				RegisterDeps{EvaluationWorkspace: missingArtifactService},
			)
			missingArtifact, err := missingArtifactTool.Handler(
				nil,
				RunBacktestRequest{
					StrategyID:      "strategy-" + fake.Lorem().Word(),
					StrategyVersion: "v3",
					Start:           time.Now().UTC(),
					End:             time.Now().UTC().Add(time.Hour),
					Quantity:        1,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, missingArtifact.Error)
			assert.Equal(t, toolErrorCodeMissingArtifact, missingArtifact.Error.Code)
			assert.Nil(t, missingArtifact.Run)
		},
	)

	t.Run("list detail and report map compact references and omit unavailable metrics", func(t *testing.T) {
		instrument := makeInstrument()
		decision := "needs-review"
		metrics := makeMetrics()
		createdAt := time.Date(2026, time.June, 18, 15, 30, 0, 0, time.FixedZone("UTC-04", -4*60*60))
		service := &fakeEvaluationWorkspaceService{}
		service.listFunc = func(
			_ context.Context,
			params app.ListEvaluationsParams,
		) ([]app.EvaluationListItem, error) {
			assert.Equal(t, app.ListEvaluationsParams{StrategyID: "strategy-a", Status: "completed"}, params)
			return []app.EvaluationListItem{
				{
					RunID:                "run-1",
					StrategyID:           "strategy-a",
					StrategyVersion:      "v1",
					StrategyArtifactHash: "artifact-1",
					SourceType:           "demo",
					SourceLabel:          "Demo example",
					Instrument:           instrument,
					Timeframe:            "1h",
					TestedRangeStart:     createdAt.Add(-2 * time.Hour),
					TestedRangeEnd:       createdAt,
					Status:               "completed",
					Decision:             &decision,
					Metrics:              metrics,
					CreatedAt:            createdAt.Add(-2 * time.Hour),
					UpdatedAt:            createdAt,
				},
				{
					RunID:                "run-2",
					StrategyID:           "strategy-a",
					StrategyVersion:      "v2",
					StrategyArtifactHash: "artifact-2",
					SourceType:           "human",
					SourceLabel:          "Human",
					Instrument:           instrument,
					Timeframe:            "1h",
					TestedRangeStart:     createdAt,
					TestedRangeEnd:       createdAt.Add(time.Hour),
					Status:               "failed",
					FailureReason:        "other",
					CreatedAt:            createdAt,
					UpdatedAt:            createdAt.Add(time.Minute),
				},
			}, nil
		}
		service.getFunc = func(_ context.Context, runID string) (*app.EvaluationDetail, error) {
			assert.Equal(t, "run-1", runID)
			return &app.EvaluationDetail{
				RunID:                runID,
				StrategyID:           "strategy-a",
				StrategyVersion:      "v1",
				StrategyArtifactHash: "artifact-1",
				SourceType:           "demo",
				SourceLabel:          "Demo example",
				Instrument:           instrument,
				Timeframe:            "1h",
				TestedRangeStart:     createdAt.Add(-2 * time.Hour),
				TestedRangeEnd:       createdAt,
				Status:               "completed",
				Decision:             &decision,
				Metrics:              metrics,
				DatasetReference: &app.EvaluationDatasetReference{
					DatasetID:      "dataset-1",
					ReplayChecksum: "checksum-1",
					CreatedAt:      createdAt,
				},
				PolicyReference: app.EvaluationPolicyReference{
					PolicyID:      "policy-1",
					PolicyVersion: "v0",
					PolicyHash:    "hash-policy-1",
				},
				CreatedAt:        createdAt.Add(-2 * time.Hour),
				UpdatedAt:        createdAt,
				AIRenderMetadata: makeMetadata(),
			}, nil
		}
		service.reportFunc = func(_ context.Context, runID string) (*app.EvaluationReportView, error) {
			assert.Equal(t, "run-1", runID)
			meta := makeMetadata()
			meta.Note = strings.Repeat("note ", 100)
			return &app.EvaluationReportView{
				RunID:    runID,
				Status:   "completed",
				Decision: &decision,
				Metrics:  metrics,
				DatasetReference: &app.EvaluationDatasetReference{
					DatasetID:      "dataset-1",
					ReplayChecksum: "checksum-1",
					CreatedAt:      createdAt,
				},
				PolicyReference: app.EvaluationPolicyReference{
					PolicyID:      "policy-1",
					PolicyVersion: "v0",
					PolicyHash:    "hash-policy-1",
				},
				AIRenderMetadata: meta,
			}, nil
		}

		listResponse, err := newListBacktestsTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
			nil,
			ListBacktestsRequest{StrategyID: "strategy-a", Status: "completed", Limit: 1},
		)
		require.NoError(t, err)
		require.Len(t, listResponse.Items, 1)
		require.NotNil(t, listResponse.Truncation)
		assert.Equal(t, "artifact-1", listResponse.Items[0].StrategyArtifactHash)
		assert.Equal(t, "1", listResponse.Truncation.NextCursor)

		detailResponse, err := newGetBacktestDetailTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
			nil,
			GetBacktestDetailRequest{RunID: "run-1"},
		)
		require.NoError(t, err)
		require.NotNil(t, detailResponse.Detail)
		assert.Equal(t, "dataset-1", detailResponse.Detail.DatasetReference.DatasetID)
		assert.Equal(t, "policy-1", detailResponse.Detail.PolicyReference.PolicyID)
		assert.Equal(t, createdAt.Add(-2*time.Hour), detailResponse.Detail.CreatedAt)
		assert.Equal(t, createdAt, detailResponse.Detail.UpdatedAt)
		assert.Equal(t, createdAt, detailResponse.Detail.DatasetReference.CreatedAt)

		reportResponse, err := newGetBacktestReportTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
			nil,
			GetBacktestReportRequest{RunID: "run-1"},
		)
		require.NoError(t, err)
		require.NotNil(t, reportResponse.Report)
		assert.Equal(t, "checksum-1", reportResponse.Report.DatasetReference.ReplayChecksum)
		require.NotNil(t, reportResponse.Truncation)
		assert.True(t, reportResponse.Truncation.IsTruncated)

		serialized, marshalErr := json.Marshal(reportResponse)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(serialized), "blockedGovernorDecisionCount")
		assert.NotContains(t, string(serialized), "rejectedGovernorDecisionCount")
	})

	t.Run(
		"evidence mapping returns bounded sections, truncation metadata, and explicit empty optional sections",
		func(t *testing.T) {
			eventTime := time.Now().UTC().Truncate(time.Second)
			service := &fakeEvaluationWorkspaceService{}
			service.evidenceFunc = func(
				_ context.Context,
				runID string,
			) (*app.EvaluationEvidenceView, error) {
				assert.Equal(t, "run-evidence", runID)
				return &app.EvaluationEvidenceView{
					RunID:            runID,
					Status:           "completed",
					AIRenderMetadata: makeMetadata(),
					Traces: []app.EvaluationTraceRow{
						{
							TraceID:      "trace-1",
							DecisionTime: eventTime,
							Result:       "approved",
							DataQuality:  "validated",
						},
						{
							TraceID:      "trace-2",
							DecisionTime: eventTime.Add(time.Minute),
							Result:       "approved",
							DataQuality:  "validated",
						},
					},
					OrderIntents: []app.EvaluationOrderIntentRow{
						{
							IntentID:          "intent-1",
							Status:            "accepted",
							ActionKind:        "long",
							RequestedQuantity: 1,
							RequestedNotional: 10,
							CreatedTime:       eventTime,
						},
						{
							IntentID:          "intent-2",
							Status:            "accepted",
							ActionKind:        "short",
							RequestedQuantity: 2,
							RequestedNotional: 20,
							CreatedTime:       eventTime.Add(time.Minute),
						},
					},
					GovernorDecisions: []app.EvaluationGovernorDecisionRow{
						{
							DecisionID: "gov-1",
							IntentID:   "intent-1",
							Status:     "approved",
							Reason:     "ok",
							Reference:  "gov-ref-1",
						},
						{},
					},
					ExecutionRecords: []app.EvaluationExecutionRow{
						{CommandID: "cmd-1", Status: "filled", EventTime: &eventTime},
						{Status: "unavailable"},
					},
					PositionSnapshots: nil,
					PortfolioSnapshots: []app.EvaluationPortfolioSnapshotRow{{
						SnapshotID:    "portfolio-1",
						FillID:        "fill-1",
						GrossExposure: 10,
						NetExposure:   7,
						RealizedPnL:   1.5,
						EventTime:     eventTime,
					}},
				}, nil
			}

			response, err := newGetBacktestEvidenceTool(RegisterDeps{EvaluationWorkspace: service}).Handler(
				nil,
				GetBacktestEvidenceRequest{RunID: "run-evidence", Limit: 1},
			)
			require.NoError(t, err)
			require.NotNil(t, response.Evidence)
			require.NotNil(t, response.Evidence.Traces.Truncation)
			assert.Equal(t, 2, *response.Evidence.Traces.Truncation.Total)
			assert.Len(t, response.Evidence.Traces.Rows, 1)
			assert.Equal(t, 1, *response.Evidence.GovernorDecisions.Truncation.Total)
			assert.Len(t, response.Evidence.GovernorDecisions.Rows, 1)
			assert.Equal(t, 1, *response.Evidence.ExecutionRecords.Truncation.Total)
			assert.Empty(t, response.Evidence.PositionSnapshots.Rows)
			require.NotNil(t, response.Evidence.PositionSnapshots.Truncation)
			assert.Equal(t, 0, *response.Evidence.PositionSnapshots.Truncation.Total)

			serialized, marshalErr := json.Marshal(response)
			require.NoError(t, marshalErr)
			assert.NotContains(t, strings.ToLower(string(serialized)), "sql")
			assert.NotContains(t, strings.ToLower(string(serialized)), "gorm")
		},
	)

	t.Run("evaluation tools return placeholder results when service deps are absent", func(t *testing.T) {
		run, err := newRunBacktestTool(RegisterDeps{}).Handler(nil, RunBacktestRequest{})
		require.NoError(t, err)
		assert.Equal(t, toolErrorCodeNotReady, run.Error.Code)

		list, err := newListBacktestsTool(RegisterDeps{}).Handler(nil, ListBacktestsRequest{})
		require.NoError(t, err)
		assert.Equal(t, toolErrorCodeNotReady, list.Error.Code)

		detail, err := newGetBacktestDetailTool(RegisterDeps{}).Handler(nil, GetBacktestDetailRequest{})
		require.NoError(t, err)
		assert.Equal(t, toolErrorCodeNotReady, detail.Error.Code)

		report, err := newGetBacktestReportTool(RegisterDeps{}).Handler(nil, GetBacktestReportRequest{})
		require.NoError(t, err)
		assert.Equal(t, toolErrorCodeNotReady, report.Error.Code)

		evidence, err := newGetBacktestEvidenceTool(RegisterDeps{}).Handler(nil, GetBacktestEvidenceRequest{})
		require.NoError(t, err)
		assert.Equal(t, toolErrorCodeNotReady, evidence.Error.Code)
	})

	t.Run("helper mappings keep errors and bounded summaries deterministic", func(t *testing.T) {
		mappedErr := mapEvaluationCreateError(
			app.NewErrInvalidInput(
				"strategyVersion",
				"strategy version does not resolve to the expected artifact",
			),
		)
		require.NotNil(t, toolErrorFrom(mappedErr))
		assert.Equal(t, toolErrorCodeMissingArtifact, toolErrorFrom(mappedErr).Code)

		statusErr := app.NewErrInvalidInput("status", "bad")
		assert.Equal(t, statusErr, mapEvaluationToolError(statusErr))
		assert.Nil(t, mapEvaluationMetrics(nil))
		assert.Nil(t, mapEvaluationDatasetReference(nil))
		assert.Nil(t, cloneStringPointer(nil))
		assert.Nil(t, cloneTimePointer(nil))

		summary, truncation := summarizeBacktestReport(app.EvaluationReportView{
			RunID:  "run-plain",
			Status: "completed",
		})
		expectedSummary := "status=completed; evidence=traces:0,intents:0,governor:0,execution:0,positions:0,portfolios:0"
		assert.Equal(t, expectedSummary, summary)
		assert.Nil(t, truncation)

		selected, selectedTruncation := paginateRows([]string{"a", "b"}, 1, 1)
		assert.Equal(t, []string{"b"}, selected)
		require.NotNil(t, selectedTruncation)
		assert.True(t, selectedTruncation.IsTruncated)

		assert.False(t, hasGovernorDecisionEvidence(app.EvaluationGovernorDecisionRow{}))
		assert.False(t, hasExecutionEvidence(app.EvaluationExecutionRow{}))
	})
}
