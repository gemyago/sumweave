package v1controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJobsController(t *testing.T) {
	fake := faker.New()

	makeAuthMiddleware := func(userID string) middleware.AuthMiddleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				ctx := httpapi.ContextWithCallerIdentity(
					r.Context(),
					&testCallerIdentity{userID: userID},
				)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}

	newHandler := func(service jobsService, userID string) http.Handler {
		ctrl := NewJobsController(
			JobsControllerDeps{JobsService: service, AuthMiddleware: makeAuthMiddleware(userID)},
		)
		return server.NewTestRootHandler().RegisterJobsRoutes(ctrl)
	}

	newRequest := func(method, target, body string, authenticated bool) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req = req.WithContext(t.Context())
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+fake.UUID().V4())
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
				UserID: fake.UUID().V4(),
				Source: jobspkg.RequesterSourceOperator,
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
		handler := newHandler(newMockjobsService(t), fake.UUID().V4())
		for _, tc := range []struct {
			method string
			url    string
			body   string
		}{
			{method: http.MethodPost, url: "/api/v1/jobs/historical-data-backfills", body: `{"venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"2026-06-17T00:00:00Z","end":"2026-06-17T00:03:00Z","pageSize":100}`},
			{method: http.MethodGet, url: "/api/v1/jobs"},
			{method: http.MethodGet, url: "/api/v1/jobs/" + fake.UUID().V4()},
		} {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, newRequest(tc.method, tc.url, tc.body, false))
			require.Equal(t, http.StatusUnauthorized, resp.Code)
		}
	})

	t.Run("create list and get delegate and return camelCase payloads", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
		createdJob := makeJob(now)
		createdJob.Status = jobspkg.JobStatusQueued
		createdJob.Result = nil
		createdJob.StartedAt = nil
		createdJob.CompletedAt = nil
		createdJob.LastAttemptAt = nil
		createdJob.AttemptCount = 0
		createdJob.Requester = jobspkg.Requester{
			UserID: userID,
			Source: jobspkg.RequesterSourceOperator,
		}
		listedJob := makeJob(now.Add(time.Hour))
		service := newMockjobsService(t)
		service.EXPECT().
			CreateHistoricalRawCandleBackfill(mock.Anything, mock.Anything).
			RunAndReturn(
				func(_ context.Context, params jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
					require.Equal(t, userID, params.Requester.UserID)
					require.Equal(t, 123, params.PageSize)
					return &createdJob, nil
				},
			)
		nextCursor := "cursor-" + fake.Lorem().Word()
		service.EXPECT().List(mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
				require.Equal(t, []jobspkg.JobStatus{jobspkg.JobStatusRunning}, params.Statuses)
				require.Equal(
					t,
					[]jobspkg.JobType{jobspkg.JobTypeHistoricalRawCandleBackfill},
					params.JobTypes,
				)
				require.Equal(
					t,
					[]jobspkg.RequesterSource{jobspkg.RequesterSourceOperator},
					params.Sources,
				)
				require.Equal(t, 7, params.Limit)
				require.Equal(t, nextCursor, params.Cursor)
				return jobspkg.ListResult{
					Items:      []jobspkg.Job{listedJob},
					NextCursor: nextCursor + "-next",
				}, nil
			},
		)
		service.EXPECT().Get(mock.Anything, listedJob.ID).Return(&listedJob, nil)
		handler := newHandler(service, userID)

		createResp := httptest.NewRecorder()
		handler.ServeHTTP(
			createResp,
			newRequest(
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
			),
		)
		require.Equal(t, http.StatusOK, createResp.Code)
		var createPayload map[string]any
		require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &createPayload))
		require.Equal(t, string(jobspkg.JobStatusQueued), createPayload["status"])
		require.Equal(t, userID, createPayload["requester"].(map[string]any)["userId"])

		listResp := httptest.NewRecorder()
		handler.ServeHTTP(
			listResp,
			newRequest(
				http.MethodGet,
				"/api/v1/jobs?status=running&jobType=data.historical_raw_candle_backfill&source=operator&limit=7&cursor="+nextCursor,
				"",
				true,
			),
		)
		require.Equal(t, http.StatusOK, listResp.Code)
		var listPayload map[string]any
		require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listPayload))
		require.Equal(t, nextCursor+"-next", listPayload["nextCursor"])

		getResp := httptest.NewRecorder()
		handler.ServeHTTP(
			getResp,
			newRequest(http.MethodGet, "/api/v1/jobs/"+listedJob.ID, "", true),
		)
		require.Equal(t, http.StatusOK, getResp.Code)
		var getPayload map[string]any
		require.NoError(t, json.Unmarshal(getResp.Body.Bytes(), &getPayload))
		require.Equal(t, listedJob.ID, getPayload["id"])
		require.Contains(t, getPayload, "lastAttemptAt")
	})

	t.Run("create allows omitted page size and forwards zero", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
		expectedJob := makeJob(now)
		expectedJob.Status = jobspkg.JobStatusQueued
		expectedJob.Result = nil
		expectedJob.StartedAt = nil
		expectedJob.CompletedAt = nil
		expectedJob.LastAttemptAt = nil
		expectedJob.AttemptCount = 0
		service := newMockjobsService(t)
		service.EXPECT().
			CreateHistoricalRawCandleBackfill(mock.Anything, mock.Anything).
			RunAndReturn(
				func(_ context.Context, params jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
					require.Equal(t, userID, params.Requester.UserID)
					require.Equal(t, 0, params.PageSize)
					return &expectedJob, nil
				},
			)
		resp := httptest.NewRecorder()
		newHandler(service, userID).ServeHTTP(
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

	t.Run("registered list and detail routes omit historical input for finance jobs", func(t *testing.T) {
		userID := fake.UUID().V4()
		now := time.Date(2026, time.July, 10, 22, 30, 0, 0, time.FixedZone("fixture", 2*60*60))
		financeJob := jobspkg.Job{
			ID: "finance-job-" + fake.UUID().V4(), JobType: jobspkg.JobType("finance.bank_connection_sync"),
			Status:    jobspkg.JobStatusSucceeded,
			Requester: jobspkg.Requester{UserID: userID, Source: jobspkg.RequesterSourceOperator},
			InputJSON: json.RawMessage(`{"connectionId":"connection-1","reason":"scheduled"}`),
			CreatedAt: now, UpdatedAt: now, QueuedAt: now, AttemptCount: 1,
		}
		service := newMockjobsService(t)
		service.EXPECT().List(mock.Anything, mock.Anything).Return(
			jobspkg.ListResult{Items: []jobspkg.Job{financeJob}},
			nil,
		)
		service.EXPECT().Get(mock.Anything, financeJob.ID).Return(&financeJob, nil)
		handler := newHandler(service, userID)
		for _, target := range []string{"/api/v1/jobs", "/api/v1/jobs/" + financeJob.ID} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newRequest(http.MethodGet, target, "", true))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var payload map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			if items, ok := payload["items"].([]any); ok {
				payload = items[0].(map[string]any)
			}
			require.Equal(t, string(financeJob.JobType), payload["jobType"])
			require.NotContains(t, payload, "input")
		}
	})

	t.Run(
		"controller maps validation not found conflict and internal errors safely",
		func(t *testing.T) {
			body := `{"venue":"hyperliquid-perps","symbol":"BTC","assetClass":"future","timeframe":"1m","start":"2026-06-17T00:00:00Z","end":"2026-06-17T00:03:00Z","pageSize":100}`

			t.Run("validation returns 400", func(t *testing.T) {
				service := newMockjobsService(t)
				service.EXPECT().CreateHistoricalRawCandleBackfill(
					mock.Anything,
					mock.Anything,
				).Return((*jobspkg.Job)(nil), app.NewErrInvalidInput("request", "bad"))
				resp := httptest.NewRecorder()
				newHandler(service, fake.UUID().V4()).ServeHTTP(
					resp,
					newRequest(
						http.MethodPost,
						"/api/v1/jobs/historical-data-backfills",
						body,
						true,
					),
				)
				require.Equal(t, http.StatusBadRequest, resp.Code)
			})

			t.Run("not found returns 404", func(t *testing.T) {
				service := newMockjobsService(t)
				service.EXPECT().Get(
					mock.Anything,
					"missing",
				).Return((*jobspkg.Job)(nil), app.NewErrNotFound("job", "missing"))
				resp := httptest.NewRecorder()
				newHandler(
					service,
					fake.UUID().V4(),
				).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/jobs/missing", "", true))
				require.Equal(t, http.StatusNotFound, resp.Code)
			})

			t.Run("idempotency conflict returns 409 with stable code", func(t *testing.T) {
				dsn := filepath.Join(t.TempDir(), "jobs-controller.sqlite")
				sqlDB, err := sqlconn.Open(dsn)
				require.NoError(t, err)
				defer func() { require.NoError(t, sqlDB.Close()) }()
				store, err := jobspkg.NewStore(sqlDB, dsn, jobspkg.StoreOpts{})
				require.NoError(t, err)
				require.NoError(t, store.AutoMigrate())
				require.NoError(t, appdispatch.AutoMigrate(t.Context(), appdispatch.Config{DatabaseDSN: dsn}, sqlDB))
				publisher, err := appdispatch.NewPublisher(appdispatch.Config{DatabaseDSN: dsn}, sqlDB, slog.Default())
				require.NoError(t, err)
				svc, err := jobspkg.NewService(
					jobspkg.ServiceDeps{Store: store, Publisher: publisher, IDGenerator: ident.NewDefaultGenerator()},
				)
				require.NoError(t, err)
				start := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
				end := start.Add(3 * time.Minute)
				idempotencyKey := "key-" + fake.Lorem().Word()
				_, err = svc.CreateHistoricalRawCandleBackfill(
					t.Context(),
					jobspkg.CreateHistoricalRawCandleBackfillParams{
						Requester: jobspkg.Requester{
							UserID: "operator-a",
							Source: jobspkg.RequesterSourceOperator,
						},
						IdempotencyKey: idempotencyKey,
						Venue:          "hyperliquid-perps",
						Symbol:         "BTC",
						AssetClass:     "future",
						Timeframe:      "1m",
						Start:          start,
						End:            end,
						PageSize:       100,
					},
				)
				require.NoError(t, err)
				resp := httptest.NewRecorder()
				newHandler(
					svc,
					"operator-a",
				).ServeHTTP(resp, newRequest(http.MethodPost, "/api/v1/jobs/historical-data-backfills", fmt.Sprintf(`{"idempotencyKey":"%s","venue":"hyperliquid-perps","symbol":"ETH","assetClass":"future","timeframe":"1m","start":"%s","end":"%s","pageSize":100}`, idempotencyKey, start.Format(time.RFC3339), end.Format(time.RFC3339)), true))
				require.Equal(t, http.StatusConflict, resp.Code)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
				require.Equal(t, "idempotency_key_conflict", payload["code"])
			})

			t.Run("internal error returns 500", func(t *testing.T) {
				service := newMockjobsService(t)
				service.EXPECT().
					List(mock.Anything, mock.Anything).
					Return(jobspkg.ListResult{}, errors.New("boom"))
				resp := httptest.NewRecorder()
				newHandler(
					service,
					fake.UUID().V4(),
				).ServeHTTP(resp, newRequest(http.MethodGet, "/api/v1/jobs", "", true))
				require.Equal(t, http.StatusInternalServerError, resp.Code)
			})
		},
	)
}

func TestOperatorRequesterFromContext(t *testing.T) {
	fake := faker.New()

	t.Run("returns unauthorized without caller identity", func(t *testing.T) {
		_, err := operatorRequesterFromContext(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("returns operator requester for authenticated caller", func(t *testing.T) {
		userID := fake.UUID().V4()
		requester, err := operatorRequesterFromContext(
			httpapi.ContextWithCallerIdentity(t.Context(), &testCallerIdentity{userID: userID}),
		)
		require.NoError(t, err)
		require.Equal(
			t,
			jobspkg.Requester{UserID: userID, Source: jobspkg.RequesterSourceOperator},
			requester,
		)
	})
}
