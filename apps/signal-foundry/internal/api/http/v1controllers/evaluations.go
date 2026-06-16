package v1controllers

import (
	"context"
	"net/http"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"go.uber.org/dig"
)

type evaluationWorkspaceService interface {
	CreateEvaluation(
		ctx context.Context,
		params app.CreateEvaluationParams,
	) (*app.EvaluationDetail, error)
	ListEvaluations(
		ctx context.Context,
		params app.ListEvaluationsParams,
	) ([]app.EvaluationListItem, error)
	GetEvaluation(ctx context.Context, runID string) (*app.EvaluationDetail, error)
	GetEvaluationReport(ctx context.Context, runID string) (*app.EvaluationReportView, error)
	GetEvaluationEvidence(ctx context.Context, runID string) (*app.EvaluationEvidenceView, error)
}

type EvaluationsControllerDeps struct {
	dig.In

	EvaluationWorkspaceService evaluationWorkspaceService
	AuthMiddleware             middleware.AuthMiddleware
}

type EvaluationsController struct{ deps EvaluationsControllerDeps }

func NewEvaluationsController(deps EvaluationsControllerDeps) *EvaluationsController {
	return &EvaluationsController{deps: deps}
}

var _ handlers.EvaluationsController = (*EvaluationsController)(nil)

func (c *EvaluationsController) CreateEvaluationBacktest(
	builder handlers.HandlerBuilder[*models.CreateEvaluationBacktestParams, *models.EvaluationDetail],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context, params *models.CreateEvaluationBacktestParams) (*models.EvaluationDetail, error) {
			created, err := c.deps.EvaluationWorkspaceService.CreateEvaluation(
				ctx,
				app.CreateEvaluationParams{
					StrategyID:         params.Payload.StrategyID,
					StrategyVersion:    params.Payload.StrategyVersion,
					Start:              params.Payload.Start,
					End:                params.Payload.End,
					Quantity:           params.Payload.Quantity,
					GovernorPolicyHash: params.Payload.GovernorPolicyHash,
					Note:               params.Payload.Note,
				},
			)
			if err != nil {
				return nil, err
			}
			response := mapEvaluationDetail(created)
			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *EvaluationsController) GetEvaluationBacktest(
	builder handlers.HandlerBuilder[*models.GetEvaluationBacktestParams, *models.EvaluationDetail],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context, params *models.GetEvaluationBacktestParams) (*models.EvaluationDetail, error) {
			detail, err := c.deps.EvaluationWorkspaceService.GetEvaluation(ctx, params.RunID)
			if err != nil {
				return nil, err
			}
			response := mapEvaluationDetail(detail)
			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *EvaluationsController) GetEvaluationBacktestEvidence(
	builder handlers.HandlerBuilder[*models.GetEvaluationBacktestEvidenceParams, *models.EvaluationEvidence],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context, params *models.GetEvaluationBacktestEvidenceParams) (*models.EvaluationEvidence, error) {
			evidence, err := c.deps.EvaluationWorkspaceService.GetEvaluationEvidence(
				ctx,
				params.RunID,
			)
			if err != nil {
				return nil, err
			}
			response := mapEvaluationEvidence(evidence)
			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *EvaluationsController) GetEvaluationBacktestReport(
	builder handlers.HandlerBuilder[*models.GetEvaluationBacktestReportParams, *models.EvaluationReport],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context, params *models.GetEvaluationBacktestReportParams) (*models.EvaluationReport, error) {
			report, err := c.deps.EvaluationWorkspaceService.GetEvaluationReport(ctx, params.RunID)
			if err != nil {
				return nil, err
			}
			response := mapEvaluationReport(report)
			return &response, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *EvaluationsController) ListEvaluationBacktests(
	builder handlers.HandlerBuilder[*models.ListEvaluationBacktestsParams, *models.EvaluationListResponse],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context, params *models.ListEvaluationBacktestsParams) (*models.EvaluationListResponse, error) {
			items, err := c.deps.EvaluationWorkspaceService.ListEvaluations(
				ctx,
				app.ListEvaluationsParams{StrategyID: params.StrategyID, Status: params.Status},
			)
			if err != nil {
				return nil, err
			}
			responseItems := make([]*models.EvaluationRow, 0, len(items))
			for i := range items {
				mapped := mapEvaluationRow(items[i])
				responseItems = append(responseItems, &mapped)
			}
			return &models.EvaluationListResponse{Items: responseItems}, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func mapEvaluationRow(item app.EvaluationListItem) models.EvaluationRow {
	row := models.EvaluationRow{
		RunID:                item.RunID,
		StrategyID:           item.StrategyID,
		StrategyVersion:      item.StrategyVersion,
		StrategyArtifactHash: item.StrategyArtifactHash,
		SourceType:           item.SourceType,
		SourceLabel:          item.SourceLabel,
		Instrument:           mapEvaluationInstrument(item.Instrument),
		Timeframe:            item.Timeframe,
		TestedRangeStart:     item.TestedRangeStart,
		TestedRangeEnd:       item.TestedRangeEnd,
		Status:               item.Status,
		FailureReason:        item.FailureReason,
		FailureDetails:       item.FailureDetails,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
		AiReadyMetadata:      mapEvaluationAIRenderMetadata(item.AIRenderMetadata),
	}
	if item.Decision != nil {
		row.Decision = *item.Decision
	}
	if item.Metrics != nil {
		metrics := mapEvaluationMetrics(item.Metrics)
		row.Metrics = &metrics
	}

	return row
}

func mapEvaluationDetail(detail *app.EvaluationDetail) models.EvaluationDetail {
	row := mapEvaluationRow(
		app.EvaluationListItem{
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
	)
	response := models.EvaluationDetail{
		RunID:                row.RunID,
		StrategyID:           row.StrategyID,
		StrategyVersion:      row.StrategyVersion,
		StrategyArtifactHash: row.StrategyArtifactHash,
		SourceType:           row.SourceType,
		SourceLabel:          row.SourceLabel,
		Instrument:           row.Instrument,
		Timeframe:            row.Timeframe,
		TestedRangeStart:     row.TestedRangeStart,
		TestedRangeEnd:       row.TestedRangeEnd,
		Status:               row.Status,
		FailureReason:        row.FailureReason,
		FailureDetails:       row.FailureDetails,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		AiReadyMetadata:      row.AiReadyMetadata,
		StrategySourceType:   detail.StrategySourceType,
		StrategySourceLabel:  detail.StrategySourceLabel,
		PolicyReference:      mapEvaluationPolicyReference(detail.PolicyReference),
		Traces:               mapEvaluationTraces(detail.Traces),
		OrderIntents:         mapEvaluationOrderIntents(detail.OrderIntents),
		GovernorDecisions:    mapEvaluationGovernorDecisions(detail.GovernorDecisions),
		ExecutionRecords:     mapEvaluationExecutions(detail.ExecutionRecords),
		PositionSnapshots:    mapEvaluationPositionSnapshots(detail.PositionSnapshots),
		PortfolioSnapshots:   mapEvaluationPortfolioSnapshots(detail.PortfolioSnapshots),
	}
	if detail.Decision != nil {
		response.Decision = *detail.Decision
	}
	if detail.Metrics != nil {
		metrics := mapEvaluationMetrics(detail.Metrics)
		response.Metrics = &metrics
	}
	if detail.DatasetReference != nil {
		dataset := mapEvaluationDatasetReference(*detail.DatasetReference)
		response.DatasetReference = &dataset
	}

	return response
}

func mapEvaluationReport(report *app.EvaluationReportView) models.EvaluationReport {
	response := models.EvaluationReport{
		RunID:           report.RunID,
		Status:          report.Status,
		FailureReason:   report.FailureReason,
		FailureDetails:  report.FailureDetails,
		PolicyReference: mapEvaluationPolicyReference(report.PolicyReference),
		AiReadyMetadata: mapEvaluationAIRenderMetadata(report.AIRenderMetadata),
	}
	if report.Decision != nil {
		response.Decision = *report.Decision
	}
	if report.Metrics != nil {
		metrics := mapEvaluationMetrics(report.Metrics)
		response.Metrics = &metrics
	}
	if report.DatasetReference != nil {
		dataset := mapEvaluationDatasetReference(*report.DatasetReference)
		response.DatasetReference = &dataset
	}

	return response
}

func mapEvaluationEvidence(evidence *app.EvaluationEvidenceView) models.EvaluationEvidence {
	return models.EvaluationEvidence{
		RunID:              evidence.RunID,
		Status:             evidence.Status,
		AiReadyMetadata:    mapEvaluationAIRenderMetadata(evidence.AIRenderMetadata),
		Traces:             mapEvaluationTraces(evidence.Traces),
		OrderIntents:       mapEvaluationOrderIntents(evidence.OrderIntents),
		GovernorDecisions:  mapEvaluationGovernorDecisions(evidence.GovernorDecisions),
		ExecutionRecords:   mapEvaluationExecutions(evidence.ExecutionRecords),
		PositionSnapshots:  mapEvaluationPositionSnapshots(evidence.PositionSnapshots),
		PortfolioSnapshots: mapEvaluationPortfolioSnapshots(evidence.PortfolioSnapshots),
	}
}

func mapEvaluationInstrument(
	value app.StrategyInstrumentInput,
) *models.StrategyDefinitionInstrument {
	instrument := models.StrategyDefinitionInstrument{
		Venue:      value.Venue,
		Symbol:     value.Symbol,
		AssetClass: value.AssetClass,
		Active:     value.Active,
	}
	return &instrument
}

func mapEvaluationMetrics(value *app.EvaluationMetricSummary) models.EvaluationMetricSummary {
	metrics := models.EvaluationMetricSummary{}
	if value.TradeCount != nil {
		metrics.TradeCount = int64(*value.TradeCount)
	}
	if value.BlockedGovernorDecisionCount != nil {
		metrics.BlockedGovernorDecisionCount = int64(*value.BlockedGovernorDecisionCount)
	}
	if value.RejectedGovernorDecisionCount != nil {
		metrics.RejectedGovernorDecisionCount = int64(*value.RejectedGovernorDecisionCount)
	}
	if value.MaxDrawdown != nil {
		metrics.MaxDrawdown = *value.MaxDrawdown
	}
	return metrics
}

func mapEvaluationAIRenderMetadata(
	value app.EvaluationAIRenderMetadata,
) *models.EvaluationAiReadyMetadata {
	counts := models.EvaluationEvidenceCounts{
		Traces:             int64(value.EvidenceCounts.Traces),
		OrderIntents:       int64(value.EvidenceCounts.OrderIntents),
		GovernorDecisions:  int64(value.EvidenceCounts.GovernorDecisions),
		ExecutionRecords:   int64(value.EvidenceCounts.ExecutionRecords),
		PositionSnapshots:  int64(value.EvidenceCounts.PositionSnapshots),
		PortfolioSnapshots: int64(value.EvidenceCounts.PortfolioSnapshots),
	}
	metadata := models.EvaluationAiReadyMetadata{
		RequestSourceType:   value.RequestSourceType,
		StrategySourceType:  value.StrategySourceType,
		StrategySourceLabel: value.StrategySourceLabel,
		Note:                value.Note,
		EvidenceCounts:      &counts,
	}
	return &metadata
}

func mapEvaluationDatasetReference(
	value app.EvaluationDatasetReference,
) models.EvaluationDatasetReference {
	return models.EvaluationDatasetReference{
		DatasetID:      value.DatasetID,
		ReplayChecksum: value.ReplayChecksum,
		CreatedAt:      value.CreatedAt,
	}
}

func mapEvaluationPolicyReference(
	value app.EvaluationPolicyReference,
) *models.EvaluationPolicyReference {
	policy := models.EvaluationPolicyReference{
		PolicyID:      value.PolicyID,
		PolicyVersion: value.PolicyVersion,
		PolicyHash:    value.PolicyHash,
	}
	return &policy
}

func mapEvaluationTraces(values []app.EvaluationTraceRow) []*models.EvaluationTraceRow {
	result := make([]*models.EvaluationTraceRow, 0, len(values))
	for _, value := range values {
		trace := models.EvaluationTraceRow{
			TraceID:      value.TraceID,
			DecisionTime: value.DecisionTime,
			Result:       value.Result,
			ReasonCodes:  value.ReasonCodes,
			DataQuality:  value.DataQuality,
			RunReference: value.RunReference,
		}
		result = append(result, &trace)
	}
	return result
}

func mapEvaluationOrderIntents(
	values []app.EvaluationOrderIntentRow,
) []*models.EvaluationOrderIntentRow {
	result := make([]*models.EvaluationOrderIntentRow, 0, len(values))
	for _, value := range values {
		item := models.EvaluationOrderIntentRow{
			IntentID:          value.IntentID,
			TraceID:           value.TraceID,
			Status:            value.Status,
			ActionKind:        value.ActionKind,
			RequestedQuantity: value.RequestedQuantity,
			RequestedNotional: value.RequestedNotional,
			CreatedTime:       value.CreatedTime,
		}
		result = append(result, &item)
	}
	return result
}

func mapEvaluationGovernorDecisions(
	values []app.EvaluationGovernorDecisionRow,
) []*models.EvaluationGovernorDecisionRow {
	result := make([]*models.EvaluationGovernorDecisionRow, 0, len(values))
	for _, value := range values {
		item := models.EvaluationGovernorDecisionRow{
			DecisionID: value.DecisionID,
			IntentID:   value.IntentID,
			Status:     value.Status,
			Reason:     value.Reason,
			Reference:  value.Reference,
		}
		result = append(result, &item)
	}
	return result
}

func mapEvaluationExecutions(values []app.EvaluationExecutionRow) []*models.EvaluationExecutionRow {
	result := make([]*models.EvaluationExecutionRow, 0, len(values))
	for _, value := range values {
		item := models.EvaluationExecutionRow{
			CommandID: value.CommandID,
			OrderID:   value.OrderID,
			FillID:    value.FillID,
			Status:    value.Status,
		}
		if value.EventTime != nil {
			item.EventTime = value.EventTime
		}
		result = append(result, &item)
	}
	return result
}

func mapEvaluationPositionSnapshots(
	values []app.EvaluationPositionSnapshotRow,
) []*models.EvaluationPositionSnapshotRow {
	result := make([]*models.EvaluationPositionSnapshotRow, 0, len(values))
	for _, value := range values {
		item := models.EvaluationPositionSnapshotRow{
			SnapshotID:  value.SnapshotID,
			FillID:      value.FillID,
			Quantity:    value.Quantity,
			RealizedPnl: value.RealizedPnL,
			EventTime:   value.EventTime,
		}
		result = append(result, &item)
	}
	return result
}

func mapEvaluationPortfolioSnapshots(
	values []app.EvaluationPortfolioSnapshotRow,
) []*models.EvaluationPortfolioSnapshotRow {
	result := make([]*models.EvaluationPortfolioSnapshotRow, 0, len(values))
	for _, value := range values {
		item := models.EvaluationPortfolioSnapshotRow{
			SnapshotID:    value.SnapshotID,
			FillID:        value.FillID,
			GrossExposure: value.GrossExposure,
			NetExposure:   value.NetExposure,
			RealizedPnl:   value.RealizedPnL,
			EventTime:     value.EventTime,
		}
		result = append(result, &item)
	}
	return result
}
