package strategyassistant

import (
	"strings"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/runtime/agent"
)

func handleStartHistoricalDataBackfillTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input StartHistoricalDataBackfillRequest,
) (StartHistoricalDataBackfillResponse, error) {
	if deps.JobsService == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return StartHistoricalDataBackfillResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	requester, correlationID := requesterFromToolContext(ctx)
	requester.Source = jobspkg.RequesterSourceAgent

	job, err := deps.JobsService.CreateHistoricalRawCandleBackfill(
		toolContextContext(ctx),
		jobspkg.CreateHistoricalRawCandleBackfillParams{
			Requester:      requester,
			IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
			CorrelationID:  correlationID,
			Venue:          input.Venue,
			Symbol:         input.Symbol,
			AssetClass:     input.AssetClass,
			Timeframe:      input.Timeframe,
			Start:          input.Start,
			End:            input.End,
			PageSize:       input.PageSize,
		},
	)
	if err != nil {
		toolErr, nextStepHint := resultMetaFromError(mapJobsToolError(err), listOrStartJobsNextStepHint())
		return StartHistoricalDataBackfillResponse{Error: toolErr, NextStepHint: nextStepHint}, nil
	}

	return StartHistoricalDataBackfillResponse{
		Job:          mapJobDetail(*job),
		NextStepHint: startHistoricalBackfillNextStepHint(),
	}, nil
}

func handleListJobsTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input ListJobsRequest,
) (ListJobsResponse, error) {
	if deps.JobsService == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return ListJobsResponse{Items: []JobSummary{}, Error: errResult, NextStepHint: nextStepHint}, nil
	}

	result, err := deps.JobsService.List(toolContextContext(ctx), jobspkg.ListParams{
		Statuses: mapJobStatuses(input.Statuses),
		JobTypes: []jobspkg.JobType{jobspkg.JobTypeHistoricalRawCandleBackfill},
		Sources:  mapRequesterSources(input.Sources),
		Limit:    input.Limit,
		Cursor:   strings.TrimSpace(input.Cursor),
	})
	if err != nil {
		toolErr, nextStepHint := resultMetaFromError(mapJobsToolError(err), listOrStartJobsNextStepHint())
		return ListJobsResponse{Items: []JobSummary{}, Error: toolErr, NextStepHint: nextStepHint}, nil
	}

	items := make([]JobSummary, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, mapJobSummary(result.Items[i]))
	}
	truncationLimit := input.Limit
	if truncationLimit <= 0 && result.NextCursor != "" {
		truncationLimit = len(items)
	}

	return ListJobsResponse{
		Items:        items,
		Truncation:   NewTruncation(truncationLimit, len(items), nil, result.NextCursor, nil),
		NextStepHint: listOrStartJobsNextStepHint(),
	}, nil
}

func handleGetJobTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetJobRequest,
) (GetJobResponse, error) {
	if deps.JobsService == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetJobResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	job, err := deps.JobsService.Get(toolContextContext(ctx), strings.TrimSpace(input.JobID))
	if err != nil {
		toolErr, nextStepHint := resultMetaFromError(mapJobsToolError(err), listOrStartJobsNextStepHint())
		return GetJobResponse{Error: toolErr, NextStepHint: nextStepHint}, nil
	}

	return GetJobResponse{Job: mapJobDetail(*job), NextStepHint: getJobNextStepHint(job.Status)}, nil
}

func requesterFromToolContext(ctx *agent.ToolContext) (jobspkg.Requester, string) {
	base := toolContextContext(ctx)
	requester := jobspkg.Requester{}
	correlationID := ""

	type userIDProvider interface{ UserID() string }
	type sessionIDProvider interface{ SessionID() string }
	type invocationIDProvider interface{ InvocationID() string }

	if provider, ok := base.(userIDProvider); ok {
		requester.UserID = strings.TrimSpace(provider.UserID())
	}
	if provider, ok := base.(sessionIDProvider); ok {
		requester.AgentSessionID = strings.TrimSpace(provider.SessionID())
	}
	if provider, ok := base.(invocationIDProvider); ok {
		requester.AgentRunID = strings.TrimSpace(provider.InvocationID())
		correlationID = strings.TrimSpace(provider.InvocationID())
	}

	return requester, correlationID
}

func mapJobsToolError(err error) error {
	if err == nil {
		return nil
	}
	if jobspkg.IsIdempotencyConflict(err) {
		return app.NewErrConflict("job", "idempotency key conflicts with an existing job")
	}
	return err
}

func mapJobSummary(job jobspkg.Job) JobSummary {
	result := JobSummary{
		ID:           job.ID,
		JobType:      string(job.JobType),
		Status:       string(job.Status),
		Requester:    mapJobRequester(job.Requester),
		Input:        mapHistoricalDataBackfillJobInput(job.Input),
		CreatedAt:    job.CreatedAt.UTC(),
		UpdatedAt:    job.UpdatedAt.UTC(),
		AttemptCount: job.AttemptCount,
	}
	if job.StartedAt != nil {
		startedAt := job.StartedAt.UTC()
		result.StartedAt = &startedAt
	}
	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.UTC()
		result.CompletedAt = &completedAt
	}
	if job.Result != nil {
		mapped := mapHistoricalDataBackfillJobResult(*job.Result)
		result.Result = &mapped
	}
	if job.Error != nil {
		mapped := mapJobExecutionError(*job.Error)
		result.Error = &mapped
	}
	return result
}

func mapJobDetail(job jobspkg.Job) *JobDetail {
	result := &JobDetail{
		JobSummary: mapJobSummary(job),
		WorkerID:   job.WorkerID,
	}
	if job.LastAttemptAt != nil {
		lastAttemptAt := job.LastAttemptAt.UTC()
		result.LastAttemptAt = &lastAttemptAt
	}
	return result
}

func mapHistoricalDataBackfillJobInput(
	input jobspkg.HistoricalRawCandleBackfillInput,
) HistoricalDataBackfillJobInput {
	return HistoricalDataBackfillJobInput{
		IngestionRunID: input.IngestionRunID,
		Venue:          input.Venue,
		Symbol:         input.Symbol,
		AssetClass:     input.AssetClass,
		Timeframe:      input.Timeframe,
		Start:          input.Start.UTC(),
		End:            input.End.UTC(),
		PageSize:       input.PageSize,
	}
}

func mapHistoricalDataBackfillJobResult(
	result jobspkg.HistoricalRawCandleBackfillResult,
) HistoricalDataBackfillJobResult {
	mapped := HistoricalDataBackfillJobResult{
		IngestionRunID:            result.IngestionRunID,
		PersistedCount:            result.PersistedCount,
		ExpectedCount:             result.ExpectedCount,
		MissingIntervalCount:      result.MissingIntervalCount,
		DuplicateNaturalKeyCount:  result.DuplicateNaturalKeyCount,
		MissingIntervalPreviewCap: result.MissingIntervalPreviewCap,
	}
	if result.FirstPersistedStart != nil {
		first := result.FirstPersistedStart.UTC()
		mapped.FirstPersistedStart = &first
	}
	if result.LastPersistedEnd != nil {
		last := result.LastPersistedEnd.UTC()
		mapped.LastPersistedEnd = &last
	}
	if result.RawPayloadCount != nil {
		count := *result.RawPayloadCount
		mapped.RawPayloadCount = &count
	}
	if len(result.MissingIntervalPreview) > 0 {
		mapped.MissingIntervalPreview = make([]JobTimeRange, 0, len(result.MissingIntervalPreview))
		for i := range result.MissingIntervalPreview {
			mapped.MissingIntervalPreview = append(mapped.MissingIntervalPreview, JobTimeRange{
				Start: result.MissingIntervalPreview[i].Start.UTC(),
				End:   result.MissingIntervalPreview[i].End.UTC(),
			})
		}
	}
	return mapped
}

func mapJobExecutionError(jobErr jobspkg.JobError) JobExecutionError {
	return JobExecutionError{Code: jobErr.Code, Summary: jobErr.Summary, Details: jobErr.Details}
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

func startHistoricalBackfillNextStepHint() string {
	return "Poll sf_jobs_get with the returned jobId until the job reaches a terminal status before running evaluation; re-check data first once the job finishes."
}

func listOrStartJobsNextStepHint() string {
	return "Inspect matching queued/running historical backfill jobs before starting another request."
}

func getJobNextStepHint(status jobspkg.JobStatus) string {
	switch status {
	case jobspkg.JobStatusQueued, jobspkg.JobStatusRunning:
		return "Keep polling sf_jobs_get until the job reaches a terminal status; do not run evaluation yet."
	case jobspkg.JobStatusSucceeded:
		return "Re-check local candle availability for the requested scope, then run synchronous evaluation only after the needed data is present."
	case jobspkg.JobStatusFailed:
		return "Summarize the failed bounded job honestly, do not invent data, and only retry with a narrower corrected request if needed."
	default:
		return listOrStartJobsNextStepHint()
	}
}
