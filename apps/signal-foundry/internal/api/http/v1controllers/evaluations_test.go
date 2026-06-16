package v1controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type evaluationWorkspaceServiceStub struct {
	createFunc   func(context.Context, app.CreateEvaluationParams) (*app.EvaluationDetail, error)
	listFunc     func(context.Context, app.ListEvaluationsParams) ([]app.EvaluationListItem, error)
	getFunc      func(context.Context, string) (*app.EvaluationDetail, error)
	reportFunc   func(context.Context, string) (*app.EvaluationReportView, error)
	evidenceFunc func(context.Context, string) (*app.EvaluationEvidenceView, error)
}

func (s *evaluationWorkspaceServiceStub) CreateEvaluation(
	ctx context.Context,
	params app.CreateEvaluationParams,
) (*app.EvaluationDetail, error) {
	if s.createFunc == nil {
		return nil, errors.New("unexpected CreateEvaluation call")
	}
	return s.createFunc(ctx, params)
}

func (s *evaluationWorkspaceServiceStub) ListEvaluations(
	ctx context.Context,
	params app.ListEvaluationsParams,
) ([]app.EvaluationListItem, error) {
	if s.listFunc == nil {
		return nil, errors.New("unexpected ListEvaluations call")
	}
	return s.listFunc(ctx, params)
}

func (s *evaluationWorkspaceServiceStub) GetEvaluation(
	ctx context.Context,
	runID string,
) (*app.EvaluationDetail, error) {
	if s.getFunc == nil {
		return nil, errors.New("unexpected GetEvaluation call")
	}
	return s.getFunc(ctx, runID)
}

func (s *evaluationWorkspaceServiceStub) GetEvaluationReport(
	ctx context.Context,
	runID string,
) (*app.EvaluationReportView, error) {
	if s.reportFunc == nil {
		return nil, errors.New("unexpected GetEvaluationReport call")
	}
	return s.reportFunc(ctx, runID)
}

func (s *evaluationWorkspaceServiceStub) GetEvaluationEvidence(
	ctx context.Context,
	runID string,
) (*app.EvaluationEvidenceView, error) {
	if s.evidenceFunc == nil {
		return nil, errors.New("unexpected GetEvaluationEvidence call")
	}
	return s.evidenceFunc(ctx, runID)
}

func TestEvaluationsController(t *testing.T) {
	fake := faker.New()

	makeAuthMiddleware := func() middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}

	newController := func(service evaluationWorkspaceService) *EvaluationsController {
		return NewEvaluationsController(
			EvaluationsControllerDeps{
				EvaluationWorkspaceService: service,
				AuthMiddleware:             makeAuthMiddleware(),
			},
		)
	}

	newHandler := func(ctrl *EvaluationsController) http.Handler {
		return server.NewTestRootHandler().RegisterEvaluationsRoutes(ctrl)
	}

	newRequest := func(method, target, body string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.Lorem().Word())
		}
		return req
	}

	makeDetail := func(status string) *app.EvaluationDetail {
		createdAt := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
		decision := "needs_review"
		tradeCount := 2
		return &app.EvaluationDetail{
			RunID:                "run-1",
			StrategyID:           "strategy-a",
			StrategyVersion:      "v1",
			StrategyArtifactHash: "artifact-a",
			SourceType:           "demo",
			SourceLabel:          "Demo example",
			StrategySourceType:   "demo",
			StrategySourceLabel:  "Demo example",
			Instrument: app.StrategyInstrumentInput{
				Venue:      "binance",
				Symbol:     "BTCUSDT",
				AssetClass: "crypto",
				Active:     true,
			},
			Timeframe:        "1h",
			TestedRangeStart: createdAt.Add(-time.Hour),
			TestedRangeEnd:   createdAt,
			Status:           status,
			Decision:         &decision,
			Metrics:          &app.EvaluationMetricSummary{TradeCount: &tradeCount},
			FailureReason:    "",
			FailureDetails:   "",
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt,
			DatasetReference: &app.EvaluationDatasetReference{
				DatasetID:      "dataset-1",
				ReplayChecksum: "checksum-1",
				CreatedAt:      createdAt,
			},
			PolicyReference: app.EvaluationPolicyReference{
				PolicyID:      "default-paper-governor-policy",
				PolicyVersion: "v0",
				PolicyHash:    "policy-hash",
			},
			AIRenderMetadata: app.EvaluationAIRenderMetadata{
				RequestSourceType:   "human",
				StrategySourceType:  "demo",
				StrategySourceLabel: "Demo example",
				Note:                "operator note",
				EvidenceCounts: app.EvaluationEvidenceCounts{
					Traces:             1,
					OrderIntents:       1,
					GovernorDecisions:  1,
					ExecutionRecords:   1,
					PositionSnapshots:  1,
					PortfolioSnapshots: 1,
				},
			},
			Traces: []app.EvaluationTraceRow{
				{
					TraceID:      "trace-1",
					DecisionTime: createdAt,
					Result:       "intent_created",
					ReasonCodes:  []string{"CROSSOVER"},
					DataQuality:  "raw",
					RunReference: "run-1",
				},
			},
			OrderIntents: []app.EvaluationOrderIntentRow{
				{
					IntentID:          "intent-1",
					TraceID:           "trace-1",
					Status:            "approved",
					ActionKind:        "long",
					RequestedQuantity: 1,
					RequestedNotional: 100,
					CreatedTime:       createdAt,
				},
			},
			GovernorDecisions: []app.EvaluationGovernorDecisionRow{
				{
					DecisionID: "decision-1",
					IntentID:   "intent-1",
					Status:     "approved",
					Reason:     "ok",
					Reference:  "decision-1",
				},
			},
			ExecutionRecords: []app.EvaluationExecutionRow{
				{
					CommandID: "cmd-1",
					OrderID:   "ord-1",
					FillID:    "fill-1",
					Status:    "filled",
					EventTime: &createdAt,
				},
			},
			PositionSnapshots: []app.EvaluationPositionSnapshotRow{
				{
					SnapshotID:  "pos-1",
					FillID:      "fill-1",
					Quantity:    1,
					RealizedPnL: 0,
					EventTime:   createdAt,
				},
			},
			PortfolioSnapshots: []app.EvaluationPortfolioSnapshotRow{
				{
					SnapshotID:    "port-1",
					FillID:        "fill-1",
					GrossExposure: 100,
					NetExposure:   100,
					RealizedPnL:   0,
					EventTime:     createdAt,
				},
			},
		}
	}

	t.Run("all evaluation endpoints require auth", func(t *testing.T) {
		handler := newHandler(newController(&evaluationWorkspaceServiceStub{}))
		cases := []struct{ method, url, body string }{
			{http.MethodGet, "/api/v1/evaluations/backtests", ""},
			{
				http.MethodPost,
				"/api/v1/evaluations/backtests",
				`{"strategyId":"strategy-a","strategyVersion":"v1","start":"2026-06-15T11:00:00Z","end":"2026-06-15T12:00:00Z","quantity":1}`,
			},
			{http.MethodGet, "/api/v1/evaluations/backtests/run-1", ""},
			{http.MethodGet, "/api/v1/evaluations/backtests/run-1/report", ""},
			{http.MethodGet, "/api/v1/evaluations/backtests/run-1/evidence", ""},
		}
		for _, tc := range cases {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(tc.method, tc.url, tc.body, false))
			require.Equal(t, http.StatusUnauthorized, resp.Code)
		}
	})

	t.Run("completed and failed create responses stay stable", func(t *testing.T) {
		service := &evaluationWorkspaceServiceStub{
			createFunc: func(_ context.Context, params app.CreateEvaluationParams) (*app.EvaluationDetail, error) {
				require.Equal(t, "strategy-a", params.StrategyID)
				if params.Note == "failed" {
					detail := makeDetail("failed")
					detail.Decision = nil
					detail.FailureReason = "replay-data-unavailable"
					detail.FailureDetails = "no local replay candles"
					return detail, nil
				}
				return makeDetail("completed"), nil
			},
		}
		handler := newHandler(newController(service))

		for _, body := range []string{`{"strategyId":"strategy-a","strategyVersion":"v1","start":"2026-06-15T11:00:00Z","end":"2026-06-15T12:00:00Z","quantity":1}`, `{"strategyId":"strategy-a","strategyVersion":"v1","start":"2026-06-15T11:00:00Z","end":"2026-06-15T12:00:00Z","quantity":1,"note":"failed"}`} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(
				resp,
				newRequest(http.MethodPost, "/api/v1/evaluations/backtests", body, true),
			)
			require.Equal(t, http.StatusOK, resp.Code)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
			require.Contains(t, payload, "aiReadyMetadata")
			require.Contains(t, payload, "policyReference")
		}
	})

	t.Run("evidence execution rows omit event time when unset", func(t *testing.T) {
		service := &evaluationWorkspaceServiceStub{
			evidenceFunc: func(context.Context, string) (*app.EvaluationEvidenceView, error) {
				detail := makeDetail("completed")
				rows := []app.EvaluationExecutionRow{
					{
						CommandID: "cmd-without-event-time",
						OrderID:   "ord-without-event-time",
						FillID:    "fill-without-event-time",
						Status:    "rejected",
					},
				}
				return &app.EvaluationEvidenceView{
					RunID:              detail.RunID,
					Status:             detail.Status,
					AIRenderMetadata:   detail.AIRenderMetadata,
					ExecutionRecords:   rows,
					Traces:             detail.Traces,
					OrderIntents:       detail.OrderIntents,
					GovernorDecisions:  detail.GovernorDecisions,
					PositionSnapshots:  detail.PositionSnapshots,
					PortfolioSnapshots: detail.PortfolioSnapshots,
				}, nil
			},
		}
		handler := newHandler(newController(service))

		resp := httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(http.MethodGet, "/api/v1/evaluations/backtests/run-1/evidence", "", true),
		)

		require.Equal(t, http.StatusOK, resp.Code)
		var payload struct {
			ExecutionRecords []map[string]any `json:"executionRecords"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Len(t, payload.ExecutionRecords, 1)
		_, ok := payload.ExecutionRecords[0]["eventTime"]
		require.False(t, ok)
	})

	t.Run("list detail report and evidence routes map responses", func(t *testing.T) {
		service := &evaluationWorkspaceServiceStub{
			listFunc: func(_ context.Context, params app.ListEvaluationsParams) ([]app.EvaluationListItem, error) {
				require.Equal(t, "strategy-a", params.StrategyID)
				require.Equal(t, "completed", params.Status)
				detail := makeDetail("completed")
				return []app.EvaluationListItem{
					{
						RunID:                detail.RunID,
						StrategyID:           detail.StrategyID,
						StrategyVersion:      detail.StrategyVersion,
						StrategyArtifactHash: detail.StrategyArtifactHash,
						SourceType:           detail.SourceType,
						SourceLabel:          detail.SourceLabel,
						Instrument:           detail.Instrument,
						Timeframe:            detail.Timeframe,
						TestedRangeStart:     detail.TestedRangeStart,
						TestedRangeEnd:       detail.TestedRangeEnd,
						Status:               detail.Status,
						Decision:             detail.Decision,
						Metrics:              detail.Metrics,
						FailureReason:        detail.FailureReason,
						FailureDetails:       detail.FailureDetails,
						CreatedAt:            detail.CreatedAt,
						UpdatedAt:            detail.UpdatedAt,
						AIRenderMetadata:     detail.AIRenderMetadata,
					},
				}, nil
			},
			getFunc: func(context.Context, string) (*app.EvaluationDetail, error) { return makeDetail("completed"), nil },
			reportFunc: func(context.Context, string) (*app.EvaluationReportView, error) {
				detail := makeDetail("completed")
				return &app.EvaluationReportView{
					RunID:            detail.RunID,
					Status:           detail.Status,
					Decision:         detail.Decision,
					FailureReason:    detail.FailureReason,
					FailureDetails:   detail.FailureDetails,
					Metrics:          detail.Metrics,
					DatasetReference: detail.DatasetReference,
					PolicyReference:  detail.PolicyReference,
					AIRenderMetadata: detail.AIRenderMetadata,
				}, nil
			},
			evidenceFunc: func(context.Context, string) (*app.EvaluationEvidenceView, error) {
				detail := makeDetail("completed")
				return &app.EvaluationEvidenceView{
					RunID:              detail.RunID,
					Status:             detail.Status,
					AIRenderMetadata:   detail.AIRenderMetadata,
					Traces:             detail.Traces,
					OrderIntents:       detail.OrderIntents,
					GovernorDecisions:  detail.GovernorDecisions,
					ExecutionRecords:   detail.ExecutionRecords,
					PositionSnapshots:  detail.PositionSnapshots,
					PortfolioSnapshots: detail.PortfolioSnapshots,
				}, nil
			},
		}
		handler := newHandler(newController(service))

		resp := httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(
				http.MethodGet,
				"/api/v1/evaluations/backtests?strategyId=strategy-a&status=completed",
				"",
				true,
			),
		)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "tradeCount")

		resp = httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(http.MethodGet, "/api/v1/evaluations/backtests/run-1", "", true),
		)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "governorDecisions")

		resp = httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(http.MethodGet, "/api/v1/evaluations/backtests/run-1/report", "", true),
		)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "replayChecksum")

		resp = httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(http.MethodGet, "/api/v1/evaluations/backtests/run-1/evidence", "", true),
		)
		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "executionRecords")
	})
}
