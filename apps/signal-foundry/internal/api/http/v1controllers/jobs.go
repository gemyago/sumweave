package v1controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/runtime/httpapi"
	"go.uber.org/dig"
)

type jobsService interface {
	CreateHistoricalRawCandleBackfill(
		ctx context.Context,
		params jobspkg.CreateHistoricalRawCandleBackfillParams,
	) (*jobspkg.Job, error)
	List(ctx context.Context, params jobspkg.ListParams) (jobspkg.ListResult, error)
	Get(ctx context.Context, jobID string) (*jobspkg.Job, error)
}

type JobsControllerDeps struct {
	dig.In

	JobsService    jobsService
	AuthMiddleware middleware.AuthMiddleware
}

type JobsController struct {
	deps JobsControllerDeps
}

func NewJobsController(deps JobsControllerDeps) *JobsController {
	return &JobsController{deps: deps}
}

var _ handlers.JobsController = (*JobsController)(nil)

func (c *JobsController) CreateHistoricalDataBackfillJob(
	builder handlers.HandlerBuilder[
		*models.CreateHistoricalDataBackfillJobParams,
		*models.JobDetailResponse,
	],
) http.Handler {
	inner := builder.HandleWithHTTP(func(
		w http.ResponseWriter,
		req *http.Request,
		params *models.CreateHistoricalDataBackfillJobParams,
	) (*models.JobDetailResponse, error) {
		requester, err := operatorRequesterFromContext(req.Context())
		if err != nil {
			return nil, err
		}

		created, err := c.deps.JobsService.CreateHistoricalRawCandleBackfill(
			req.Context(),
			jobspkg.CreateHistoricalRawCandleBackfillParams{
				Requester:      requester,
				IdempotencyKey: params.Payload.IDempotencyKey,
				CorrelationID:  params.Payload.CorrelationID,
				Venue:          params.Payload.Venue,
				Symbol:         params.Payload.Symbol,
				AssetClass:     params.Payload.AssetClass,
				Timeframe:      params.Payload.Timeframe,
				Start:          params.Payload.Start,
				End:            params.Payload.End,
				PageSize:       int(params.Payload.PageSize),
			},
		)
		if err != nil {
			if jobspkg.IsIdempotencyConflict(err) {
				writeJobConflict(w, "idempotency_key_conflict", "idempotency key conflicts with an existing job")
				return &models.JobDetailResponse{}, nil
			}
			return nil, err
		}

		response := mapJobDetail(*created)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *JobsController) GetJob(
	builder handlers.HandlerBuilder[*models.GetJobParams, *models.JobDetailResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetJobParams,
	) (*models.JobDetailResponse, error) {
		if _, err := operatorRequesterFromContext(ctx); err != nil {
			return nil, err
		}

		job, err := c.deps.JobsService.Get(ctx, params.JobID)
		if err != nil {
			return nil, err
		}

		response := mapJobDetail(*job)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *JobsController) ListJobs(
	builder handlers.HandlerBuilder[*models.ListJobsParams, *models.JobListResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ListJobsParams,
	) (*models.JobListResponse, error) {
		if _, err := operatorRequesterFromContext(ctx); err != nil {
			return nil, err
		}

		result, err := c.deps.JobsService.List(ctx, jobspkg.ListParams{
			Statuses: mapJobStatuses(params.Status),
			JobTypes: mapJobTypes(params.JobType),
			Sources:  mapRequesterSources(params.Source),
			Limit:    int(params.Limit),
			Cursor:   params.Cursor,
		})
		if err != nil {
			return nil, err
		}

		response := mapJobListResponse(result)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func operatorRequesterFromContext(ctx context.Context) (jobspkg.Requester, error) {
	identity := httpapi.CallerIdentityFromContext(ctx)
	if identity == nil || strings.TrimSpace(identity.UserID()) == "" {
		return jobspkg.Requester{}, app.NewErrUnauthorized("unauthorized")
	}

	return jobspkg.Requester{
		UserID: identity.UserID(),
		Source: jobspkg.RequesterSourceOperator,
	}, nil
}

func writeJobConflict(w http.ResponseWriter, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}

func mapJobListResponse(result jobspkg.ListResult) models.JobListResponse {
	items := make([]*models.JobSummary, 0, len(result.Items))
	for i := range result.Items {
		summary := mapJobSummary(result.Items[i])
		items = append(items, &summary)
	}
	return models.JobListResponse{Items: items, NextCursor: result.NextCursor}
}

func mapJobSummary(job jobspkg.Job) models.JobSummary {
	response := models.JobSummary{
		ID:           job.ID,
		JobType:      string(job.JobType),
		Status:       string(job.Status),
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
		AttemptCount: int64(job.AttemptCount),
	}
	requester := mapJobRequester(job.Requester)
	response.Requester = &requester
	if job.JobType == jobspkg.JobTypeHistoricalRawCandleBackfill {
		input := mapHistoricalDataBackfillInput(job.Input)
		response.Input = &input
	}
	if job.StartedAt != nil {
		startedAt := *job.StartedAt
		response.StartedAt = &startedAt
	}
	if job.CompletedAt != nil {
		completedAt := *job.CompletedAt
		response.CompletedAt = &completedAt
	}
	if job.Result != nil {
		result := mapHistoricalDataBackfillResult(*job.Result)
		response.Result = &result
	}
	if job.Error != nil {
		jobErr := mapJobError(*job.Error)
		response.Error = &jobErr
	}
	return response
}

func mapJobDetail(job jobspkg.Job) models.JobDetailResponse {
	response := models.JobDetailResponse{
		ID:           job.ID,
		JobType:      string(job.JobType),
		Status:       string(job.Status),
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
		WorkerID:     job.WorkerID,
		AttemptCount: int64(job.AttemptCount),
	}
	requester := mapJobRequester(job.Requester)
	response.Requester = &requester
	if job.JobType == jobspkg.JobTypeHistoricalRawCandleBackfill {
		input := mapHistoricalDataBackfillInput(job.Input)
		response.Input = &input
	}
	if job.StartedAt != nil {
		startedAt := *job.StartedAt
		response.StartedAt = &startedAt
	}
	if job.CompletedAt != nil {
		completedAt := *job.CompletedAt
		response.CompletedAt = &completedAt
	}
	if job.LastAttemptAt != nil {
		lastAttemptAt := *job.LastAttemptAt
		response.LastAttemptAt = &lastAttemptAt
	}
	if job.Result != nil {
		result := mapHistoricalDataBackfillResult(*job.Result)
		response.Result = &result
	}
	if job.Error != nil {
		jobErr := mapJobError(*job.Error)
		response.Error = &jobErr
	}
	return response
}

func mapJobRequester(requester jobspkg.Requester) models.JobRequester {
	return models.JobRequester{
		UserID:         requester.UserID,
		Source:         string(requester.Source),
		AgentSessionID: requester.AgentSessionID,
		AgentRunID:     requester.AgentRunID,
	}
}

func mapHistoricalDataBackfillInput(
	input jobspkg.HistoricalRawCandleBackfillInput,
) models.HistoricalDataBackfillJobInput {
	return models.HistoricalDataBackfillJobInput{
		IngestionRunID: input.IngestionRunID,
		Venue:          input.Venue,
		Symbol:         input.Symbol,
		AssetClass:     input.AssetClass,
		Timeframe:      input.Timeframe,
		Start:          input.Start,
		End:            input.End,
		PageSize:       int64(input.PageSize),
	}
}

func mapHistoricalDataBackfillResult(
	result jobspkg.HistoricalRawCandleBackfillResult,
) models.HistoricalDataBackfillJobResult {
	response := models.HistoricalDataBackfillJobResult{
		IngestionRunID:            result.IngestionRunID,
		PersistedCount:            int64(result.PersistedCount),
		ExpectedCount:             int64(result.ExpectedCount),
		MissingIntervalCount:      int64(result.MissingIntervalCount),
		DuplicateNaturalKeyCount:  int64(result.DuplicateNaturalKeyCount),
		MissingIntervalPreviewCap: int64(result.MissingIntervalPreviewCap),
	}
	if result.FirstPersistedStart != nil {
		firstPersistedStart := *result.FirstPersistedStart
		response.FirstPersistedStart = &firstPersistedStart
	}
	if result.LastPersistedEnd != nil {
		lastPersistedEnd := *result.LastPersistedEnd
		response.LastPersistedEnd = &lastPersistedEnd
	}
	if result.RawPayloadCount != nil {
		rawPayloadCount := int64(*result.RawPayloadCount)
		response.RawPayloadCount = &rawPayloadCount
	}
	if len(result.MissingIntervalPreview) > 0 {
		response.MissingIntervalPreview = make([]*models.JobTimeRange, 0, len(result.MissingIntervalPreview))
		for i := range result.MissingIntervalPreview {
			interval := models.JobTimeRange{
				Start: result.MissingIntervalPreview[i].Start,
				End:   result.MissingIntervalPreview[i].End,
			}
			response.MissingIntervalPreview = append(response.MissingIntervalPreview, &interval)
		}
	}
	return response
}

func mapJobError(jobErr jobspkg.JobError) models.JobError {
	return models.JobError{
		Code:    jobErr.Code,
		Summary: jobErr.Summary,
		Details: jobErr.Details,
	}
}

func mapJobStatuses(values []string) []jobspkg.JobStatus {
	result := make([]jobspkg.JobStatus, 0, len(values))
	for i := range values {
		trimmed := strings.TrimSpace(values[i])
		if trimmed == "" {
			continue
		}
		result = append(result, jobspkg.JobStatus(trimmed))
	}
	return result
}

func mapJobTypes(values []string) []jobspkg.JobType {
	result := make([]jobspkg.JobType, 0, len(values))
	for i := range values {
		trimmed := strings.TrimSpace(values[i])
		if trimmed == "" {
			continue
		}
		result = append(result, jobspkg.JobType(trimmed))
	}
	return result
}

func mapRequesterSources(values []string) []jobspkg.RequesterSource {
	result := make([]jobspkg.RequesterSource, 0, len(values))
	for i := range values {
		trimmed := strings.TrimSpace(values[i])
		if trimmed == "" {
			continue
		}
		result = append(result, jobspkg.RequesterSource(trimmed))
	}
	return result
}
