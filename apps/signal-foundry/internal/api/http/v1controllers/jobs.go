package v1controllers

import (
	"context"
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
	List(context.Context, jobspkg.ListParams) (jobspkg.ListResult, error)
	Get(context.Context, string) (*jobspkg.Job, error)
}
type JobsControllerDeps struct {
	dig.In

	JobsService    jobsService
	AuthMiddleware middleware.AuthMiddleware
}
type JobsController struct{ deps JobsControllerDeps }

func NewJobsController(deps JobsControllerDeps) *JobsController { return &JobsController{deps: deps} }

var _ handlers.JobsController = (*JobsController)(nil)

func (c *JobsController) GetJob(
	builder handlers.HandlerBuilder[*models.GetJobParams, *models.JobDetailResponse],
) http.Handler {
	return c.deps.AuthMiddleware(
		builder.HandleWith(func(ctx context.Context, params *models.GetJobParams) (*models.JobDetailResponse, error) {
			if err := requireOperatorRequester(ctx); err != nil {
				return nil, err
			}
			job, err := c.deps.JobsService.Get(ctx, params.JobID)
			if err != nil {
				return nil, err
			}
			response := mapJobDetail(*job)
			return &response, nil
		}),
	)
}

func (c *JobsController) ListJobs(
	builder handlers.HandlerBuilder[*models.ListJobsParams, *models.JobListResponse],
) http.Handler {
	return c.deps.AuthMiddleware(
		builder.HandleWith(func(ctx context.Context, params *models.ListJobsParams) (*models.JobListResponse, error) {
			if err := requireOperatorRequester(ctx); err != nil {
				return nil, err
			}
			result, err := c.deps.JobsService.List(
				ctx,
				jobspkg.ListParams{
					Statuses: mapJobStatuses(params.Status),
					JobTypes: mapJobTypes(params.JobType),
					Sources:  mapRequesterSources(params.Source),
					Limit:    int(params.Limit),
					Cursor:   params.Cursor,
				},
			)
			if err != nil {
				return nil, err
			}
			response := mapJobListResponse(result)
			return &response, nil
		}),
	)
}
func requireOperatorRequester(ctx context.Context) error {
	identity := httpapi.CallerIdentityFromContext(ctx)
	if identity == nil || strings.TrimSpace(identity.UserID()) == "" {
		return app.NewErrUnauthorized("unauthorized")
	}
	return nil
}
func mapJobListResponse(result jobspkg.ListResult) models.JobListResponse {
	items := make([]*models.JobSummary, 0, len(result.Items))
	for i := range result.Items {
		item := mapJobSummary(result.Items[i])
		items = append(items, &item)
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
	if job.StartedAt != nil {
		value := *job.StartedAt
		response.StartedAt = &value
	}
	if job.CompletedAt != nil {
		value := *job.CompletedAt
		response.CompletedAt = &value
	}
	if job.Error != nil {
		value := mapJobError(*job.Error)
		response.Error = &value
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
	if job.StartedAt != nil {
		value := *job.StartedAt
		response.StartedAt = &value
	}
	if job.CompletedAt != nil {
		value := *job.CompletedAt
		response.CompletedAt = &value
	}
	if job.LastAttemptAt != nil {
		value := *job.LastAttemptAt
		response.LastAttemptAt = &value
	}
	if job.Error != nil {
		value := mapJobError(*job.Error)
		response.Error = &value
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
func mapJobError(jobErr jobspkg.JobError) models.JobError {
	return models.JobError{Code: jobErr.Code, Summary: jobErr.Summary, Details: jobErr.Details}
}
func mapJobStatuses(values []string) []jobspkg.JobStatus {
	result := make([]jobspkg.JobStatus, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, jobspkg.JobStatus(value))
		}
	}
	return result
}
func mapJobTypes(values []string) []jobspkg.JobType {
	result := make([]jobspkg.JobType, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, jobspkg.JobType(value))
		}
	}
	return result
}
func mapRequesterSources(values []string) []jobspkg.RequesterSource {
	result := make([]jobspkg.RequesterSource, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, jobspkg.RequesterSource(value))
		}
	}
	return result
}
