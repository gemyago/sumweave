package v1controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type jobsServiceStub struct {
	createHistoricalRawCandleBackfillFunc func(context.Context, jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error)
	listFunc                              func(context.Context, jobspkg.ListParams) (jobspkg.ListResult, error)
	getFunc                               func(context.Context, string) (*jobspkg.Job, error)
}

func (s *jobsServiceStub) CreateHistoricalRawCandleBackfill(
	ctx context.Context,
	params jobspkg.CreateHistoricalRawCandleBackfillParams,
) (*jobspkg.Job, error) {
	if s.createHistoricalRawCandleBackfillFunc == nil {
		return nil, errors.New("unexpected CreateHistoricalRawCandleBackfill call")
	}
	return s.createHistoricalRawCandleBackfillFunc(ctx, params)
}

func (s *jobsServiceStub) List(ctx context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
	if s.listFunc == nil {
		return jobspkg.ListResult{}, errors.New("unexpected List call")
	}
	return s.listFunc(ctx, params)
}

func (s *jobsServiceStub) Get(ctx context.Context, jobID string) (*jobspkg.Job, error) {
	if s.getFunc == nil {
		return nil, errors.New("unexpected Get call")
	}
	return s.getFunc(ctx, jobID)
}

func TestJobsController(t *testing.T) {
	fake := faker.New()

	makeAuthMiddleware := func(userID string) middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(r.Context(), &testCallerIdentity{userID: userID})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}

	newController := func(service jobsService, userID string) *JobsController {
		return NewJobsController(JobsControllerDeps{
			JobsService:    service,
			AuthMiddleware: makeAuthMiddleware(userID),
		})
	}

	newHandler := func(ctrl *JobsController) http.Handler {
		return server.NewTestRootHandler().RegisterJobsRoutes(ctrl)
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

	makeJob := func(now time.Time) jobspkg.Job {
		t.Helper()
		rawPayloadCount := 2
		startedAt := now.Add(2 * time.Minute)
		completedAt := now.Add(3 * time.Minute)
		lastAttemptAt := startedAt
		return jobspkg.Job{
			ID:      fake.UUID().V4(),
			JobType: jobspkg.JobTypeHistoricalRawCandleBackfill,
			Status:  jobspkg.JobStatusSucceeded,
			Requester: jobspkg.Requester{
				UserID:         fake.UUID().V4(),
				Source:         jobspkg.RequesterSourceOperator,
				AgentSessionID: "",
				AgentRunID:     "",
			},
			Input: jobspkg.HistoricalRawCandleBackfillInput{
				IngestionRunID: fake.UUID().V4(),
				Venue:          "hyperliquid-perps",
				Symbol:         "BTC",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          now.UTC(),
				End:            now.UTC().Add(3 * time.Minute),
				PageSize:       200,
			},
			Result: &jobspkg.HistoricalRawCandleBackfillResult{
				IngestionRunID:            fake.UUID().V4(),
				PersistedCount:            2,
				ExpectedCount:             3,
				MissingIntervalCount:      1,
				RawPayloadCount:           &rawPayloadCount,
				MissingIntervalPreviewCap: 5,
			},
			Error: &jobspkg.JobError{
				Code:    "job_execution_failed",
				Summary: "bounded summary",
				Details: "bounded details",
			},
			CreatedAt:     now.UTC(),
			UpdatedAt:     now.UTC().Add(3 * time.Minute),
			QueuedAt:      now.UTC(),
			StartedAt:     &startedAt,
			CompletedAt:   &completedAt,
			WorkerID:      "jobs-worker-test",
			AttemptCount:  1,
			LastAttemptAt: &lastAttemptAt,
		}
	}

	t.Run("all jobs endpoints require auth", func(t *testing.T) {
		ctrl := newController(&jobsServiceStub{}, fake.UUID().V4())
		handler := newHandler(ctrl)

		cases := []struct {
			method string
			url    string
			body   string
		}{
			{
				method: http.MethodPost,
				url:    "/api/v1/jobs/historical-data-backfills",
				body:   `{"venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"2026-06-17T00:00:00Z","end":"2026-06-17T00:03:00Z","pageSize":100}`,
			},
			{method: http.MethodGet, url: "/api/v1/jobs"},
			{method: http.MethodGet, url: "/api/v1/jobs/" + fake.UUID().V4()},
		}

		for _, tc := range cases {
			t.Run(tc.method+" "+tc.url, func(t *testing.T) {
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, newRequest(tc.method, tc.url, tc.body, false))
				require.Equal(t, http.StatusUnauthorized, resp.Code)
			})
		}
	})

	t.Run("create returns immediate queued camelCase payload and derives operator requester", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
		expectedJob := makeJob(now)
		expectedJob.Status = jobspkg.JobStatusQueued
		expectedJob.Result = nil
		expectedJob.StartedAt = nil
		expectedJob.CompletedAt = nil
		expectedJob.LastAttemptAt = nil
		expectedJob.AttemptCount = 0
		expectedJob.Requester = jobspkg.Requester{UserID: userID, Source: jobspkg.RequesterSourceOperator}

		ctrl := newController(&jobsServiceStub{
			createHistoricalRawCandleBackfillFunc: func(
				_ context.Context,
				params jobspkg.CreateHistoricalRawCandleBackfillParams,
			) (*jobspkg.Job, error) {
				require.Equal(t, userID, params.Requester.UserID)
				require.Equal(t, jobspkg.RequesterSourceOperator, params.Requester.Source)
				require.Equal(t, "hyperliquid-perps", params.Venue)
				require.Equal(t, "future", params.AssetClass)
				require.Equal(t, 123, params.PageSize)
				return &expectedJob, nil
			},
		}, userID)
		handler := newHandler(ctrl)

		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, newRequest(
			http.MethodPost,
			"/api/v1/jobs/historical-data-backfills",
			fmt.Sprintf(
				`{"idempotencyKey":"%s","correlationId":"%s","venue":"hyperliquid-perps","symbol":"btc","assetClass":"future","timeframe":"1m","start":"%s","end":"%s","pageSize":123}`,
				"job-key-"+fake.Lorem().Word(),
				"corr-"+fake.Lorem().Word(),
				now.Format(time.RFC3339),
				now.Add(3*time.Minute).Format(time.RFC3339),
			),
			true,
		))
		require.Equal(t, http.StatusOK, resp.Code)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, string(jobspkg.JobStatusQueued), payload["status"])
		require.Equal(t, string(jobspkg.JobTypeHistoricalRawCandleBackfill), payload["jobType"])
		requester := payload["requester"].(map[string]any)
		require.Equal(t, userID, requester["userId"])
		require.Equal(t, string(jobspkg.RequesterSourceOperator), requester["source"])
		input := payload["input"].(map[string]any)
		require.Contains(t, input, "ingestionRunId")
		require.Contains(t, input, "assetClass")
	})

	t.Run("create allows omitted page size and forwards zero to the service", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
		expectedJob := makeJob(now)
		expectedJob.Status = jobspkg.JobStatusQueued
		expectedJob.Result = nil
		expectedJob.StartedAt = nil
		expectedJob.CompletedAt = nil
		expectedJob.LastAttemptAt = nil
		expectedJob.AttemptCount = 0

		ctrl := newController(&jobsServiceStub{
			createHistoricalRawCandleBackfillFunc: func(
				_ context.Context,
				params jobspkg.CreateHistoricalRawCandleBackfillParams,
			) (*jobspkg.Job, error) {
				require.Equal(t, userID, params.Requester.UserID)
				require.Equal(t, 0, params.PageSize)
				return &expectedJob, nil
			},
		}, userID)

		resp := httptest.NewRecorder()
		newHandler(ctrl).ServeHTTP(
			resp,
			newRequest(
				http.MethodPost,
				"/api/v1/jobs/historical-data-backfills",
				fmt.Sprintf(
					`{"venue":"hyperliquid-perps","symbol":"btc","assetClass":"future","timeframe":"1m","start":"%s","end":"%s"}`,
					now.Format(time.RFC3339),
					now.Add(3*time.Minute).Format(time.RFC3339),
				),
				true,
			),
		)
		require.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("list supports filters and returns compact camelCase payload", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 1, 0, 0, 0, time.UTC)
		expectedJob := makeJob(now)
		nextCursor := "cursor-" + fake.Lorem().Word()

		ctrl := newController(&jobsServiceStub{
			listFunc: func(_ context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
				require.Equal(t, []jobspkg.JobStatus{jobspkg.JobStatusRunning}, params.Statuses)
				require.Equal(t, []jobspkg.JobType{jobspkg.JobTypeHistoricalRawCandleBackfill}, params.JobTypes)
				require.Equal(t, []jobspkg.RequesterSource{jobspkg.RequesterSourceOperator}, params.Sources)
				require.Equal(t, 7, params.Limit)
				require.Equal(t, nextCursor, params.Cursor)
				return jobspkg.ListResult{Items: []jobspkg.Job{expectedJob}, NextCursor: nextCursor + "-next"}, nil
			},
		}, fake.UUID().V4())
		handler := newHandler(ctrl)

		resp := httptest.NewRecorder()
		handler.ServeHTTP(
			resp,
			newRequest(
				http.MethodGet,
				"/api/v1/jobs?status=running&jobType=historical_raw_candle_backfill&source=operator&limit=7&cursor="+nextCursor,
				"",
				true,
			),
		)
		require.Equal(t, http.StatusOK, resp.Code)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, nextCursor+"-next", payload["nextCursor"])
		items := payload["items"].([]any)
		require.Len(t, items, 1)
		item := items[0].(map[string]any)
		require.Contains(t, item, "attemptCount")
		require.Contains(t, item, "completedAt")
		require.Contains(t, item, "result")
		require.Contains(t, item, "input")
	})

	t.Run("get returns detail payload", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 2, 0, 0, 0, time.UTC)
		expectedJob := makeJob(now)

		ctrl := newController(&jobsServiceStub{
			getFunc: func(_ context.Context, jobID string) (*jobspkg.Job, error) {
				require.Equal(t, expectedJob.ID, jobID)
				return &expectedJob, nil
			},
		}, fake.UUID().V4())
		handler := newHandler(ctrl)

		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/jobs/"+expectedJob.ID, "", true))
		require.Equal(t, http.StatusOK, resp.Code)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, expectedJob.ID, payload["id"])
		require.Contains(t, payload, "lastAttemptAt")
		require.Contains(t, payload, "workerId")
		require.Contains(t, payload, "error")
	})

	t.Run("create maps validation not-found conflict and internal errors safely", func(t *testing.T) {
		body := `{"venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"2026-06-17T00:00:00Z","end":"2026-06-17T00:03:00Z","pageSize":100}`

		t.Run("validation returns 400", func(t *testing.T) {
			ctrl := newController(&jobsServiceStub{
				createHistoricalRawCandleBackfillFunc: func(context.Context, jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
					return nil, app.NewErrInvalidInput("request", "bad")
				},
			}, fake.UUID().V4())

			resp := httptest.NewRecorder()
			newHandler(ctrl).ServeHTTP(
				resp,
				newRequest(http.MethodPost, "/api/v1/jobs/historical-data-backfills", body, true),
			)
			require.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("not found returns 404", func(t *testing.T) {
			ctrl := newController(&jobsServiceStub{
				getFunc: func(context.Context, string) (*jobspkg.Job, error) {
					return nil, app.NewErrNotFound("job", "missing")
				},
			}, fake.UUID().V4())

			resp := httptest.NewRecorder()
			newHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/jobs/missing", "", true))
			require.Equal(t, http.StatusNotFound, resp.Code)
		})

		t.Run("idempotency conflict returns 409 with stable code", func(t *testing.T) {
			store, err := jobspkg.NewStore(":memory:", jobspkg.StoreOpts{})
			require.NoError(t, err)
			require.NoError(t, store.AutoMigrate())
			svc, err := jobspkg.NewService(jobspkg.ServiceDeps{Store: store, IDGenerator: ident.NewDefaultGenerator()})
			require.NoError(t, err)
			start := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
			end := start.Add(3 * time.Minute)
			idempotencyKey := "key-" + fake.Lorem().Word()
			_, err = svc.CreateHistoricalRawCandleBackfill(t.Context(), jobspkg.CreateHistoricalRawCandleBackfillParams{
				Requester:      jobspkg.Requester{UserID: "operator-a", Source: jobspkg.RequesterSourceOperator},
				IdempotencyKey: idempotencyKey,
				Venue:          "hyperliquid-perps",
				Symbol:         "BTC",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          start,
				End:            end,
				PageSize:       100,
			})
			require.NoError(t, err)

			ctrl := newController(svc, "operator-a")

			resp := httptest.NewRecorder()
			newHandler(ctrl).ServeHTTP(resp, newRequest(
				http.MethodPost,
				"/api/v1/jobs/historical-data-backfills",
				fmt.Sprintf(
					`{"idempotencyKey":"%s","venue":"hyperliquid-perps","symbol":"ETH","assetClass":"future","timeframe":"1m","start":"%s","end":"%s","pageSize":100}`,
					idempotencyKey,
					start.Format(time.RFC3339),
					end.Format(time.RFC3339),
				),
				true,
			))
			require.Equal(t, http.StatusConflict, resp.Code)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
			require.Equal(t, "idempotency_key_conflict", payload["code"])
		})

		t.Run("internal error returns 500", func(t *testing.T) {
			ctrl := newController(&jobsServiceStub{
				listFunc: func(context.Context, jobspkg.ListParams) (jobspkg.ListResult, error) {
					return jobspkg.ListResult{}, errors.New("boom")
				},
			}, fake.UUID().V4())

			resp := httptest.NewRecorder()
			newHandler(ctrl).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/jobs", "", true))
			require.Equal(t, http.StatusInternalServerError, resp.Code)
		})
	})
}
