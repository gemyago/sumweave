package strategyassistant

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeJobsService struct {
	createCalls []jobspkg.CreateHistoricalRawCandleBackfillParams
	listCalls   []jobspkg.ListParams
	getCalls    []string

	createFunc func(context.Context, jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error)
	listFunc   func(context.Context, jobspkg.ListParams) (jobspkg.ListResult, error)
	getFunc    func(context.Context, string) (*jobspkg.Job, error)
}

func (f *fakeJobsService) CreateHistoricalRawCandleBackfill(
	ctx context.Context,
	params jobspkg.CreateHistoricalRawCandleBackfillParams,
) (*jobspkg.Job, error) {
	f.createCalls = append(f.createCalls, params)
	if f.createFunc == nil {
		return nil, errors.New("unexpected create historical raw candle backfill call")
	}
	return f.createFunc(ctx, params)
}

func (f *fakeJobsService) List(ctx context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
	f.listCalls = append(f.listCalls, params)
	if f.listFunc == nil {
		return jobspkg.ListResult{}, errors.New("unexpected list jobs call")
	}
	return f.listFunc(ctx, params)
}

func (f *fakeJobsService) Get(ctx context.Context, jobID string) (*jobspkg.Job, error) {
	f.getCalls = append(f.getCalls, jobID)
	if f.getFunc == nil {
		return nil, errors.New("unexpected get job call")
	}
	return f.getFunc(ctx, jobID)
}

type jobsToolContextStub struct {
	context.Context

	sessionID    string
	invocationID string
	userID       string
}

func (c *jobsToolContextStub) SessionID() string    { return c.sessionID }
func (c *jobsToolContextStub) InvocationID() string { return c.invocationID }
func (c *jobsToolContextStub) UserID() string       { return c.userID }

func TestJobsTools(t *testing.T) {
	fake := faker.New()
	makeJob := func(now time.Time, status jobspkg.JobStatus) jobspkg.Job {
		rawPayloadCount := 2
		job := jobspkg.Job{
			ID:      fake.UUID().V4(),
			JobType: jobspkg.JobTypeHistoricalRawCandleBackfill,
			Status:  status,
			Requester: jobspkg.Requester{
				UserID:         fake.UUID().V4(),
				Source:         jobspkg.RequesterSourceAgent,
				AgentSessionID: "session-" + fake.Lorem().Word(),
				AgentRunID:     "run-" + fake.Lorem().Word(),
			},
			Input: jobspkg.HistoricalRawCandleBackfillInput{
				IngestionRunID: fake.UUID().V4(),
				Venue:          "hyperliquid-perps",
				Symbol:         "BTC",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          now,
				End:            now.Add(3 * time.Minute),
				PageSize:       200,
			},
			CreatedAt:    now,
			UpdatedAt:    now.Add(time.Minute),
			QueuedAt:     now,
			AttemptCount: 0,
		}
		if status == jobspkg.JobStatusRunning ||
			status == jobspkg.JobStatusSucceeded ||
			status == jobspkg.JobStatusFailed {
			startedAt := now.Add(30 * time.Second)
			lastAttemptAt := startedAt
			job.StartedAt = &startedAt
			job.LastAttemptAt = &lastAttemptAt
			job.AttemptCount = 1
			job.WorkerID = "jobs-worker-test"
		}
		if status == jobspkg.JobStatusSucceeded {
			completedAt := now.Add(time.Minute)
			firstPersistedStart := job.Input.Start
			lastPersistedEnd := job.Input.End
			job.CompletedAt = &completedAt
			job.Result = &jobspkg.HistoricalRawCandleBackfillResult{
				IngestionRunID:            job.Input.IngestionRunID,
				PersistedCount:            2,
				ExpectedCount:             3,
				MissingIntervalCount:      1,
				FirstPersistedStart:       &firstPersistedStart,
				LastPersistedEnd:          &lastPersistedEnd,
				RawPayloadCount:           &rawPayloadCount,
				MissingIntervalPreviewCap: 5,
			}
		}
		if status == jobspkg.JobStatusFailed {
			completedAt := now.Add(time.Minute)
			job.CompletedAt = &completedAt
			job.Error = &jobspkg.JobError{
				Code:    "job_execution_failed",
				Summary: "bounded failure",
				Details: "bounded failure details",
			}
		}
		return job
	}

	t.Run("start historical backfill calls jobs service directly with agent metadata", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.FixedZone("UTC+05:30", 5*60*60+30*60))
		service := &fakeJobsService{}
		idempotencyKey := "key-" + fake.Lorem().Word()
		ctx := &jobsToolContextStub{
			Context:      t.Context(),
			sessionID:    "session-" + fake.Lorem().Word(),
			invocationID: "invocation-" + fake.Lorem().Word(),
			userID:       "user-" + fake.Lorem().Word(),
		}
		expectedJob := makeJob(now, jobspkg.JobStatusQueued)
		service.createFunc = func(_ context.Context, params jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
			assert.Equal(t, jobspkg.RequesterSourceAgent, params.Requester.Source)
			assert.Equal(t, ctx.userID, params.Requester.UserID)
			assert.Equal(t, ctx.sessionID, params.Requester.AgentSessionID)
			assert.Equal(t, ctx.invocationID, params.Requester.AgentRunID)
			assert.Equal(t, ctx.invocationID, params.CorrelationID)
			assert.Equal(t, "hyperliquid-perps", params.Venue)
			assert.Equal(t, "btc", params.Symbol)
			assert.Equal(t, "future", params.AssetClass)
			assert.Equal(t, "1m", params.Timeframe)
			assert.Equal(t, now, params.Start)
			assert.Equal(t, now.Add(3*time.Minute), params.End)
			assert.Equal(t, 123, params.PageSize)
			assert.Equal(t, idempotencyKey, params.IdempotencyKey)
			return &expectedJob, nil
		}

		response, err := newStartHistoricalDataBackfillTool(RegisterDeps{JobsService: service}).Handler(
			&agent.ToolContext{Context: ctx},
			StartHistoricalDataBackfillRequest{
				IdempotencyKey: idempotencyKey,
				Venue:          "hyperliquid-perps",
				Symbol:         "btc",
				AssetClass:     "future",
				Timeframe:      "1m",
				Start:          now,
				End:            now.Add(3 * time.Minute),
				PageSize:       123,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Job)
		assert.Nil(t, response.Error)
		assert.Equal(t, expectedJob.ID, response.Job.ID)
		assert.Equal(t, string(jobspkg.JobStatusQueued), response.Job.Status)
		assert.Contains(t, response.NextStepHint, "sf_jobs_get")
		assert.Contains(t, strings.ToLower(response.NextStepHint), "before running evaluation")
		assert.Len(t, service.createCalls, 1)
	})

	t.Run("start maps safe validation conflict and internal errors", func(t *testing.T) {
		validationService := &fakeJobsService{}
		validationService.createFunc = func(_ context.Context, _ jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
			return nil, app.NewErrInvalidInput("request", "bad range")
		}
		validationTool := newStartHistoricalDataBackfillTool(
			RegisterDeps{JobsService: validationService},
		)
		validationResponse, err := validationTool.Handler(
			nil,
			StartHistoricalDataBackfillRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, validationResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, validationResponse.Error.Code)

		conflictService := &fakeJobsService{}
		conflictService.createFunc = func(_ context.Context, _ jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
			return nil, app.NewErrConflict("job", "idempotency key conflicts with an existing job")
		}
		conflictTool := newStartHistoricalDataBackfillTool(
			RegisterDeps{JobsService: conflictService},
		)
		conflictResponse, err := conflictTool.Handler(
			nil,
			StartHistoricalDataBackfillRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, conflictResponse.Error)
		assert.Equal(t, toolErrorCodeConflict, conflictResponse.Error.Code)

		internalService := &fakeJobsService{}
		internalService.createFunc = func(_ context.Context, _ jobspkg.CreateHistoricalRawCandleBackfillParams) (*jobspkg.Job, error) {
			return nil, errors.New("backend query failed")
		}
		internalTool := newStartHistoricalDataBackfillTool(
			RegisterDeps{JobsService: internalService},
		)
		internalResponse, err := internalTool.Handler(
			nil,
			StartHistoricalDataBackfillRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, internalResponse.Error)
		assert.Equal(t, toolErrorCodeInternal, internalResponse.Error.Code)
		assert.Equal(t, toolErrorMessageInternal, internalResponse.Error.Message)
	})

	t.Run("list returns bounded items and truncation for duplicate inspection", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 1, 0, 0, 0, time.UTC)
		service := &fakeJobsService{}
		jobOne := makeJob(now, jobspkg.JobStatusQueued)
		jobTwo := makeJob(now.Add(time.Minute), jobspkg.JobStatusRunning)
		service.listFunc = func(_ context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
			assert.Equal(t, []jobspkg.JobStatus{jobspkg.JobStatusQueued, jobspkg.JobStatusRunning}, params.Statuses)
			assert.Equal(t, []jobspkg.RequesterSource{jobspkg.RequesterSourceAgent}, params.Sources)
			assert.Equal(t, 2, params.Limit)
			assert.Equal(t, "cursor-a", params.Cursor)
			return jobspkg.ListResult{Items: []jobspkg.Job{jobOne, jobTwo}, NextCursor: "cursor-b"}, nil
		}

		response, err := newListJobsTool(RegisterDeps{JobsService: service}).Handler(
			nil,
			ListJobsRequest{
				Statuses: []string{string(jobspkg.JobStatusQueued), string(jobspkg.JobStatusRunning)},
				Sources:  []string{string(jobspkg.RequesterSourceAgent)},
				Limit:    2,
				Cursor:   "cursor-a",
			},
		)
		require.NoError(t, err)
		assert.Nil(t, response.Error)
		require.Len(t, response.Items, 2)
		assert.Equal(t, jobOne.ID, response.Items[0].ID)
		assert.Equal(t, jobTwo.Input.Symbol, response.Items[1].Input.Symbol)
		require.NotNil(t, response.Truncation)
		assert.True(t, response.Truncation.IsTruncated)
		assert.Equal(t, "cursor-b", response.Truncation.NextCursor)
		assert.Contains(t, strings.ToLower(response.NextStepHint), "queued")
		assert.Contains(t, strings.ToLower(response.NextStepHint), "running")
	})

	t.Run("list preserves cursor metadata when the default limit is used", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 1, 30, 0, 0, time.UTC)
		service := &fakeJobsService{}
		jobsPage := make([]jobspkg.Job, 0, 25)
		for i := range 25 {
			jobsPage = append(jobsPage, makeJob(now.Add(time.Duration(i)*time.Minute), jobspkg.JobStatusQueued))
		}
		service.listFunc = func(_ context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error) {
			assert.Equal(t, 0, params.Limit)
			return jobspkg.ListResult{Items: jobsPage, NextCursor: "cursor-default-next"}, nil
		}

		response, err := newListJobsTool(RegisterDeps{JobsService: service}).Handler(nil, ListJobsRequest{})
		require.NoError(t, err)
		require.NotNil(t, response.Truncation)
		assert.True(t, response.Truncation.IsTruncated)
		assert.Equal(t, 25, response.Truncation.Limit)
		assert.Equal(t, "cursor-default-next", response.Truncation.NextCursor)
	})

	t.Run("list and get map service errors safely", func(t *testing.T) {
		listService := &fakeJobsService{}
		listService.listFunc = func(_ context.Context, _ jobspkg.ListParams) (jobspkg.ListResult, error) {
			return jobspkg.ListResult{}, app.NewErrInvalidInput("limit", "must be positive")
		}
		listResponse, err := newListJobsTool(RegisterDeps{JobsService: listService}).Handler(
			nil,
			ListJobsRequest{Limit: -1},
		)
		require.NoError(t, err)
		require.NotNil(t, listResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, listResponse.Error.Code)

		getService := &fakeJobsService{}
		getService.getFunc = func(_ context.Context, _ string) (*jobspkg.Job, error) {
			return nil, app.NewErrNotFound("job", "missing-job")
		}
		getResponse, err := newGetJobTool(RegisterDeps{JobsService: getService}).Handler(
			nil,
			GetJobRequest{JobID: "missing-job"},
		)
		require.NoError(t, err)
		require.NotNil(t, getResponse.Error)
		assert.Equal(t, toolErrorCodeNotFound, getResponse.Error.Code)
	})

	t.Run("get returns status-specific next steps", func(t *testing.T) {
		now := time.Date(2026, time.June, 17, 2, 0, 0, 0, time.UTC)
		queuedJob := makeJob(now, jobspkg.JobStatusQueued)
		succeededJob := makeJob(now.Add(time.Minute), jobspkg.JobStatusSucceeded)
		failedJob := makeJob(now.Add(2*time.Minute), jobspkg.JobStatusFailed)

		cases := []struct {
			name     string
			job      jobspkg.Job
			contains []string
		}{
			{name: "queued", job: queuedJob, contains: []string{"sf_jobs_get", "terminal"}},
			{name: "succeeded", job: succeededJob, contains: []string{"re-check", "synchronous evaluation"}},
			{name: "failed", job: failedJob, contains: []string{"failed", "do not invent data"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				service := &fakeJobsService{}
				service.getFunc = func(_ context.Context, jobID string) (*jobspkg.Job, error) {
					assert.Equal(t, tc.job.ID, jobID)
					return &tc.job, nil
				}

				response, err := newGetJobTool(RegisterDeps{JobsService: service}).Handler(
					nil,
					GetJobRequest{JobID: tc.job.ID},
				)
				require.NoError(t, err)
				require.NotNil(t, response.Job)
				assert.Nil(t, response.Error)
				if tc.job.Status == jobspkg.JobStatusSucceeded {
					require.NotNil(t, response.Job.Result)
					assert.NotNil(t, response.Job.Result.FirstPersistedStart)
					assert.NotNil(t, response.Job.Result.LastPersistedEnd)
				}
				for _, fragment := range tc.contains {
					assert.Contains(t, strings.ToLower(response.NextStepHint), strings.ToLower(fragment))
				}
			})
		}
	})

	t.Run("missing jobs service returns deterministic placeholder responses", func(t *testing.T) {
		startTool := newStartHistoricalDataBackfillTool(RegisterDeps{})
		start, err := startTool.Handler(nil, StartHistoricalDataBackfillRequest{})
		require.NoError(t, err)
		require.NotNil(t, start.Error)
		assert.Equal(t, toolErrorCodeNotReady, start.Error.Code)

		listTool := newListJobsTool(RegisterDeps{})
		list, err := listTool.Handler(nil, ListJobsRequest{})
		require.NoError(t, err)
		require.NotNil(t, list.Error)
		assert.Equal(t, toolErrorCodeNotReady, list.Error.Code)

		getTool := newGetJobTool(RegisterDeps{})
		get, err := getTool.Handler(nil, GetJobRequest{})
		require.NoError(t, err)
		require.NotNil(t, get.Error)
		assert.Equal(t, toolErrorCodeNotReady, get.Error.Code)
	})
}
