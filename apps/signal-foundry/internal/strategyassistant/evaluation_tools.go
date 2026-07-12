package strategyassistant

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/agent"
)

const (
	defaultBacktestsToolLimit     = 20
	maxBacktestsToolLimit         = 100
	defaultBacktestEvidenceLimit  = 50
	maxBacktestEvidenceLimit      = 200
	maxBacktestReportSummaryChars = 320

	evaluationFailureReasonDataUnavailable = "replay-data-unavailable"
	evaluationRunStatusFailed              = "failed"
	evaluationErrorDetailFieldKey          = "field"
)

func handleRunBacktestTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input RunBacktestRequest,
) (RunBacktestResponse, error) {
	if deps.EvaluationWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return RunBacktestResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	strategyID, err := requiredString(input.StrategyID, "strategyId")
	if err != nil {
		return RunBacktestResponse{Error: toolErrorFrom(err)}, nil
	}
	strategyVersion := strings.TrimSpace(input.StrategyVersion)
	if strategyVersion == "" {
		return RunBacktestResponse{
			Error: toolErrorFrom(NewUnsavedVersionError(map[string]string{"strategyId": strategyID})),
		}, nil
	}

	detail, err := deps.EvaluationWorkspace.CreateEvaluation(
		toolContextContext(ctx),
		app.CreateEvaluationParams{
			StrategyID:         strategyID,
			StrategyVersion:    strategyVersion,
			Start:              input.Start,
			End:                input.End,
			Quantity:           input.Quantity,
			GovernorPolicyHash: strings.TrimSpace(input.GovernorPolicyHash),
			Note:               strings.TrimSpace(input.Note),
		},
	)
	if err != nil {
		return RunBacktestResponse{Error: toolErrorFrom(mapEvaluationCreateError(err))}, nil
	}

	response := RunBacktestResponse{Run: mapEvaluationRunSummary(*detail)}
	if detail.Status == evaluationRunStatusFailed &&
		detail.FailureReason == evaluationFailureReasonDataUnavailable {
		response.Error = toolErrorFrom(NewDataUnavailableError(map[string]string{
			"runId":         detail.RunID,
			"failureReason": detail.FailureReason,
		}))
		response.NextStepHint = "Check local candle availability for the selected range before retrying this backtest."
	}

	return response, nil
}

func handleListBacktestsTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input ListBacktestsRequest,
) (ListBacktestsResponse, error) {
	if deps.EvaluationWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return ListBacktestsResponse{
			Items:        []EvaluationListRow{},
			Error:        errResult,
			NextStepHint: nextStepHint,
		}, nil
	}

	limit, err := normalizeLimit(input.Limit, defaultBacktestsToolLimit, maxBacktestsToolLimit)
	if err != nil {
		return ListBacktestsResponse{Items: []EvaluationListRow{}, Error: toolErrorFrom(err)}, nil
	}
	offset, err := normalizeOffset(input.Offset)
	if err != nil {
		return ListBacktestsResponse{Items: []EvaluationListRow{}, Error: toolErrorFrom(err)}, nil
	}

	items, err := deps.EvaluationWorkspace.ListEvaluations(
		toolContextContext(ctx),
		app.ListEvaluationsParams{
			StrategyID: strings.TrimSpace(input.StrategyID),
			Status:     strings.TrimSpace(input.Status),
		},
	)
	if err != nil {
		return ListBacktestsResponse{
			Items: []EvaluationListRow{},
			Error: toolErrorFrom(mapEvaluationToolError(err)),
		}, nil
	}

	start := min(offset, len(items))
	end := min(start+limit, len(items))
	rows := make([]EvaluationListRow, end-start)
	for i := start; i < end; i++ {
		rows[i-start] = mapEvaluationListRow(items[i])
	}

	total := len(items)
	response := ListBacktestsResponse{Items: rows}
	if end < len(items) {
		nextOffset := offset + len(rows)
		response.Truncation = NewTruncation(limit, len(rows), &total, strconv.Itoa(nextOffset), nil)
		response.NextStepHint = fmt.Sprintf("Retry with offset=%d to continue browsing backtests.", nextOffset)
	}

	return response, nil
}

func handleGetBacktestDetailTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetBacktestDetailRequest,
) (GetBacktestDetailResponse, error) {
	if deps.EvaluationWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetBacktestDetailResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	runID, err := requiredString(input.RunID, "runId")
	if err != nil {
		return GetBacktestDetailResponse{Error: toolErrorFrom(err)}, nil
	}

	detail, err := deps.EvaluationWorkspace.GetEvaluation(toolContextContext(ctx), runID)
	if err != nil {
		return GetBacktestDetailResponse{Error: toolErrorFrom(mapEvaluationToolError(err))}, nil
	}

	mapped := mapEvaluationDetail(*detail)
	return GetBacktestDetailResponse{Detail: &mapped}, nil
}

func handleGetBacktestReportTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetBacktestReportRequest,
) (GetBacktestReportResponse, error) {
	if deps.EvaluationWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetBacktestReportResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	runID, err := requiredString(input.RunID, "runId")
	if err != nil {
		return GetBacktestReportResponse{Error: toolErrorFrom(err)}, nil
	}

	report, err := deps.EvaluationWorkspace.GetEvaluationReport(
		toolContextContext(ctx),
		runID,
	)
	if err != nil {
		return GetBacktestReportResponse{Error: toolErrorFrom(mapEvaluationToolError(err))}, nil
	}

	summary, truncation := summarizeBacktestReport(*report)
	mapped := mapEvaluationReport(*report, summary)
	return GetBacktestReportResponse{Report: &mapped, Truncation: truncation}, nil
}

func handleGetBacktestEvidenceTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetBacktestEvidenceRequest,
) (GetBacktestEvidenceResponse, error) {
	if deps.EvaluationWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetBacktestEvidenceResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	runID, err := requiredString(input.RunID, "runId")
	if err != nil {
		return GetBacktestEvidenceResponse{Error: toolErrorFrom(err)}, nil
	}
	limit, err := normalizeLimit(input.Limit, defaultBacktestEvidenceLimit, maxBacktestEvidenceLimit)
	if err != nil {
		return GetBacktestEvidenceResponse{Error: toolErrorFrom(err)}, nil
	}
	offset, err := normalizeOffset(input.Offset)
	if err != nil {
		return GetBacktestEvidenceResponse{Error: toolErrorFrom(err)}, nil
	}

	evidence, err := deps.EvaluationWorkspace.GetEvaluationEvidence(toolContextContext(ctx), runID)
	if err != nil {
		return GetBacktestEvidenceResponse{Error: toolErrorFrom(mapEvaluationToolError(err))}, nil
	}

	mapped := mapEvaluationEvidence(*evidence, limit, offset)
	return GetBacktestEvidenceResponse{Evidence: &mapped}, nil
}

func mapEvaluationCreateError(err error) error {
	var invalidInputErr *app.InvalidInputError
	if errors.As(err, &invalidInputErr) && invalidInputErr.Field == "strategyVersion" {
		reason := strings.ToLower(invalidInputErr.Reason)
		switch {
		case strings.Contains(reason, "status must be ready"):
			return NewNotReadyError(map[string]string{evaluationErrorDetailFieldKey: invalidInputErr.Field})
		case strings.Contains(reason, "expected artifact"):
			return NewMissingArtifactError(
				map[string]string{evaluationErrorDetailFieldKey: invalidInputErr.Field},
			)
		}
	}

	var notFoundErr *app.NotFoundError
	if errors.As(err, &notFoundErr) && notFoundErr.Resource == "strategy artifact" {
		return NewMissingArtifactError(map[string]string{"artifactHash": notFoundErr.ID})
	}

	return mapEvaluationToolError(err)
}

func mapEvaluationToolError(err error) error {
	var invalidInputErr *app.InvalidInputError
	if errors.As(err, &invalidInputErr) && invalidInputErr.Field == "strategyVersion" {
		if strings.Contains(strings.ToLower(invalidInputErr.Reason), "status must be ready") {
			return NewNotReadyError(map[string]string{evaluationErrorDetailFieldKey: invalidInputErr.Field})
		}
	}

	return err
}

func mapEvaluationRunSummary(detail app.EvaluationDetail) *EvaluationRunSummary {
	return &EvaluationRunSummary{
		RunID:          detail.RunID,
		Status:         detail.Status,
		FailureReason:  detail.FailureReason,
		FailureDetails: detail.FailureDetails,
		CreatedAt:      detail.CreatedAt,
		UpdatedAt:      detail.UpdatedAt,
	}
}

func mapEvaluationListRow(item app.EvaluationListItem) EvaluationListRow {
	return EvaluationListRow{
		RunID:                item.RunID,
		StrategyID:           item.StrategyID,
		StrategyVersion:      item.StrategyVersion,
		StrategyArtifactHash: item.StrategyArtifactHash,
		SourceType:           item.SourceType,
		SourceLabel:          item.SourceLabel,
		Instrument:           mapStrategyInstrument(item.Instrument),
		Timeframe:            item.Timeframe,
		TestedRangeStart:     item.TestedRangeStart,
		TestedRangeEnd:       item.TestedRangeEnd,
		Status:               item.Status,
		Decision:             cloneStringPointer(item.Decision),
		Metrics:              mapEvaluationMetrics(item.Metrics),
		FailureReason:        item.FailureReason,
		FailureDetails:       item.FailureDetails,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func mapEvaluationDetail(detail app.EvaluationDetail) EvaluationDetail {
	return EvaluationDetail{
		RunID:                detail.RunID,
		StrategyID:           detail.StrategyID,
		StrategyVersion:      detail.StrategyVersion,
		StrategyArtifactHash: detail.StrategyArtifactHash,
		SourceType:           detail.SourceType,
		SourceLabel:          detail.SourceLabel,
		Instrument:           mapStrategyInstrument(detail.Instrument),
		Timeframe:            detail.Timeframe,
		TestedRangeStart:     detail.TestedRangeStart,
		TestedRangeEnd:       detail.TestedRangeEnd,
		Status:               detail.Status,
		Decision:             cloneStringPointer(detail.Decision),
		FailureReason:        detail.FailureReason,
		FailureDetails:       detail.FailureDetails,
		Metrics:              mapEvaluationMetrics(detail.Metrics),
		DatasetReference:     mapEvaluationDatasetReference(detail.DatasetReference),
		PolicyReference:      mapEvaluationPolicyReference(detail.PolicyReference),
		CreatedAt:            detail.CreatedAt,
		UpdatedAt:            detail.UpdatedAt,
		AIRenderMetadata:     mapEvaluationAIRenderMetadata(detail.AIRenderMetadata),
	}
}

func mapEvaluationReport(report app.EvaluationReportView, summary string) EvaluationReport {
	mapped := EvaluationReport{
		RunID:            report.RunID,
		Status:           report.Status,
		Summary:          summary,
		FailureReason:    report.FailureReason,
		FailureDetails:   report.FailureDetails,
		Metrics:          mapEvaluationMetrics(report.Metrics),
		DatasetReference: mapEvaluationDatasetReference(report.DatasetReference),
		PolicyReference:  mapEvaluationPolicyReference(report.PolicyReference),
		AIRenderMetadata: mapEvaluationAIRenderMetadata(report.AIRenderMetadata),
	}
	if report.Decision != nil {
		mapped.Decision = *report.Decision
	}
	return mapped
}

func mapEvaluationEvidence(view app.EvaluationEvidenceView, limit int, offset int) EvaluationEvidence {
	traces := buildTraceEvidenceSection(view.Traces, limit, offset)
	orderIntents := buildOrderIntentEvidenceSection(view.OrderIntents, limit, offset)
	governorDecisions := buildGovernorDecisionEvidenceSection(view.GovernorDecisions, limit, offset)
	executionRecords := buildExecutionEvidenceSection(view.ExecutionRecords, limit, offset)
	positionSnapshots := buildPositionSnapshotEvidenceSection(view.PositionSnapshots, limit, offset)
	portfolioSnapshots := buildPortfolioSnapshotEvidenceSection(view.PortfolioSnapshots, limit, offset)

	metadata := mapEvaluationAIRenderMetadata(view.AIRenderMetadata)
	if metadata != nil {
		metadata.EvidenceCounts = &EvaluationEvidenceCounts{
			Traces:             len(view.Traces),
			OrderIntents:       len(view.OrderIntents),
			GovernorDecisions:  countActualGovernorDecisions(view.GovernorDecisions),
			ExecutionRecords:   countActualExecutionRows(view.ExecutionRecords),
			PositionSnapshots:  len(view.PositionSnapshots),
			PortfolioSnapshots: len(view.PortfolioSnapshots),
		}
	}

	return EvaluationEvidence{
		RunID:              view.RunID,
		Status:             view.Status,
		AIRenderMetadata:   metadata,
		Traces:             traces,
		OrderIntents:       orderIntents,
		GovernorDecisions:  governorDecisions,
		ExecutionRecords:   executionRecords,
		PositionSnapshots:  positionSnapshots,
		PortfolioSnapshots: portfolioSnapshots,
	}
}

func summarizeBacktestReport(report app.EvaluationReportView) (string, *ToolTruncation) {
	parts := []string{fmt.Sprintf("status=%s", report.Status)}
	if report.Decision != nil && strings.TrimSpace(*report.Decision) != "" {
		parts = append(parts, fmt.Sprintf("decision=%s", strings.TrimSpace(*report.Decision)))
	}
	if report.FailureReason != "" {
		parts = append(parts, fmt.Sprintf("failureReason=%s", report.FailureReason))
	}
	if metricsSummary := formatMetricsSummary(report.Metrics); metricsSummary != "" {
		parts = append(parts, metricsSummary)
	}
	if note := strings.TrimSpace(report.AIRenderMetadata.Note); note != "" {
		parts = append(parts, fmt.Sprintf("note=%s", note))
	}
	if countsSummary := formatEvidenceCountsSummary(report.AIRenderMetadata.EvidenceCounts); countsSummary != "" {
		parts = append(parts, countsSummary)
	}

	summary := strings.Join(parts, "; ")
	if len(summary) <= maxBacktestReportSummaryChars {
		return summary, nil
	}

	truncated := summary[:maxBacktestReportSummaryChars]
	total := len(summary)
	return truncated, NewTruncation(maxBacktestReportSummaryChars, len(truncated), &total, "", nil)
}

func formatMetricsSummary(metrics *app.EvaluationMetricSummary) string {
	if metrics == nil {
		return ""
	}

	parts := make([]string, 0, 4)
	if metrics.TradeCount != nil {
		parts = append(parts, fmt.Sprintf("tradeCount=%d", *metrics.TradeCount))
	}
	if metrics.BlockedGovernorDecisionCount != nil {
		parts = append(parts, fmt.Sprintf("blockedGovernorDecisions=%d", *metrics.BlockedGovernorDecisionCount))
	}
	if metrics.RejectedGovernorDecisionCount != nil {
		parts = append(parts, fmt.Sprintf("rejectedGovernorDecisions=%d", *metrics.RejectedGovernorDecisionCount))
	}
	if metrics.MaxDrawdown != nil {
		parts = append(parts, fmt.Sprintf("maxDrawdown=%.6f", *metrics.MaxDrawdown))
	}

	return strings.Join(parts, ",")
}

func formatEvidenceCountsSummary(counts app.EvaluationEvidenceCounts) string {
	return fmt.Sprintf(
		"evidence=traces:%d,intents:%d,governor:%d,execution:%d,positions:%d,portfolios:%d",
		counts.Traces,
		counts.OrderIntents,
		counts.GovernorDecisions,
		counts.ExecutionRecords,
		counts.PositionSnapshots,
		counts.PortfolioSnapshots,
	)
}

func mapEvaluationMetrics(metrics *app.EvaluationMetricSummary) *EvaluationMetricSummary {
	if metrics == nil {
		return nil
	}

	return &EvaluationMetricSummary{
		TradeCount:                    metrics.TradeCount,
		BlockedGovernorDecisionCount:  metrics.BlockedGovernorDecisionCount,
		RejectedGovernorDecisionCount: metrics.RejectedGovernorDecisionCount,
		MaxDrawdown:                   metrics.MaxDrawdown,
	}
}

func mapEvaluationDatasetReference(reference *app.EvaluationDatasetReference) *EvaluationDatasetReference {
	if reference == nil {
		return nil
	}

	return &EvaluationDatasetReference{
		DatasetID:      reference.DatasetID,
		ReplayChecksum: reference.ReplayChecksum,
		CreatedAt:      reference.CreatedAt,
	}
}

func mapEvaluationPolicyReference(reference app.EvaluationPolicyReference) EvaluationPolicyReference {
	return EvaluationPolicyReference{
		PolicyID:      reference.PolicyID,
		PolicyVersion: reference.PolicyVersion,
		PolicyHash:    reference.PolicyHash,
	}
}

func mapEvaluationAIRenderMetadata(metadata app.EvaluationAIRenderMetadata) *EvaluationAIRenderMetadata {
	return &EvaluationAIRenderMetadata{
		RequestSourceType:   metadata.RequestSourceType,
		StrategySourceType:  metadata.StrategySourceType,
		StrategySourceLabel: metadata.StrategySourceLabel,
		Note:                metadata.Note,
		EvidenceCounts: &EvaluationEvidenceCounts{
			Traces:             metadata.EvidenceCounts.Traces,
			OrderIntents:       metadata.EvidenceCounts.OrderIntents,
			GovernorDecisions:  metadata.EvidenceCounts.GovernorDecisions,
			ExecutionRecords:   metadata.EvidenceCounts.ExecutionRecords,
			PositionSnapshots:  metadata.EvidenceCounts.PositionSnapshots,
			PortfolioSnapshots: metadata.EvidenceCounts.PortfolioSnapshots,
		},
	}
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildTraceEvidenceSection(
	rows []app.EvaluationTraceRow,
	limit int,
	offset int,
) EvaluationTraceEvidenceSection {
	selected, truncation := paginateRows(rows, limit, offset)
	mapped := make([]EvaluationTraceEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationTraceEvidenceRow{
			TraceID:      selected[i].TraceID,
			DecisionTime: selected[i].DecisionTime,
			Result:       selected[i].Result,
			ReasonCodes:  append([]string(nil), selected[i].ReasonCodes...),
			DataQuality:  selected[i].DataQuality,
			RunReference: selected[i].RunReference,
		}
	}
	return EvaluationTraceEvidenceSection{Rows: mapped, Truncation: truncation}
}

func buildOrderIntentEvidenceSection(
	rows []app.EvaluationOrderIntentRow,
	limit int,
	offset int,
) EvaluationOrderIntentEvidenceSection {
	selected, truncation := paginateRows(rows, limit, offset)
	mapped := make([]EvaluationOrderIntentEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationOrderIntentEvidenceRow{
			IntentID:          selected[i].IntentID,
			TraceID:           selected[i].TraceID,
			Status:            selected[i].Status,
			ActionKind:        selected[i].ActionKind,
			RequestedQuantity: selected[i].RequestedQuantity,
			RequestedNotional: selected[i].RequestedNotional,
			CreatedTime:       selected[i].CreatedTime,
		}
	}
	return EvaluationOrderIntentEvidenceSection{Rows: mapped, Truncation: truncation}
}

func buildGovernorDecisionEvidenceSection(
	rows []app.EvaluationGovernorDecisionRow,
	limit int,
	offset int,
) EvaluationGovernorDecisionEvidenceSection {
	actual := make([]app.EvaluationGovernorDecisionRow, 0, len(rows))
	for i := range rows {
		if !hasGovernorDecisionEvidence(rows[i]) {
			continue
		}
		actual = append(actual, rows[i])
	}
	selected, truncation := paginateRows(actual, limit, offset)
	mapped := make([]EvaluationGovernorDecisionEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationGovernorDecisionEvidenceRow{
			DecisionID: selected[i].DecisionID,
			IntentID:   selected[i].IntentID,
			Status:     selected[i].Status,
			Reason:     selected[i].Reason,
			Reference:  selected[i].Reference,
		}
	}
	return EvaluationGovernorDecisionEvidenceSection{Rows: mapped, Truncation: truncation}
}

func buildExecutionEvidenceSection(
	rows []app.EvaluationExecutionRow,
	limit int,
	offset int,
) EvaluationExecutionEvidenceSection {
	actual := make([]app.EvaluationExecutionRow, 0, len(rows))
	for i := range rows {
		if !hasExecutionEvidence(rows[i]) {
			continue
		}
		actual = append(actual, rows[i])
	}
	selected, truncation := paginateRows(actual, limit, offset)
	mapped := make([]EvaluationExecutionEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationExecutionEvidenceRow{
			CommandID: selected[i].CommandID,
			OrderID:   selected[i].OrderID,
			FillID:    selected[i].FillID,
			Status:    selected[i].Status,
			EventTime: cloneTimePointer(selected[i].EventTime),
		}
	}
	return EvaluationExecutionEvidenceSection{Rows: mapped, Truncation: truncation}
}

func buildPositionSnapshotEvidenceSection(
	rows []app.EvaluationPositionSnapshotRow,
	limit int,
	offset int,
) EvaluationPositionSnapshotEvidenceSection {
	selected, truncation := paginateRows(rows, limit, offset)
	mapped := make([]EvaluationPositionSnapshotEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationPositionSnapshotEvidenceRow{
			SnapshotID:  selected[i].SnapshotID,
			FillID:      selected[i].FillID,
			Quantity:    selected[i].Quantity,
			RealizedPnL: selected[i].RealizedPnL,
			EventTime:   selected[i].EventTime,
		}
	}
	return EvaluationPositionSnapshotEvidenceSection{Rows: mapped, Truncation: truncation}
}

func buildPortfolioSnapshotEvidenceSection(
	rows []app.EvaluationPortfolioSnapshotRow,
	limit int,
	offset int,
) EvaluationPortfolioSnapshotEvidenceSection {
	selected, truncation := paginateRows(rows, limit, offset)
	mapped := make([]EvaluationPortfolioSnapshotEvidenceRow, len(selected))
	for i := range selected {
		mapped[i] = EvaluationPortfolioSnapshotEvidenceRow{
			SnapshotID:    selected[i].SnapshotID,
			FillID:        selected[i].FillID,
			GrossExposure: selected[i].GrossExposure,
			NetExposure:   selected[i].NetExposure,
			RealizedPnL:   selected[i].RealizedPnL,
			EventTime:     selected[i].EventTime,
		}
	}
	return EvaluationPortfolioSnapshotEvidenceSection{Rows: mapped, Truncation: truncation}
}

func hasGovernorDecisionEvidence(row app.EvaluationGovernorDecisionRow) bool {
	return strings.TrimSpace(row.DecisionID) != "" ||
		strings.TrimSpace(row.IntentID) != "" ||
		strings.TrimSpace(row.Status) != "" ||
		strings.TrimSpace(row.Reason) != "" ||
		strings.TrimSpace(row.Reference) != ""
}

func hasExecutionEvidence(row app.EvaluationExecutionRow) bool {
	return strings.TrimSpace(row.CommandID) != "" ||
		strings.TrimSpace(row.OrderID) != "" ||
		strings.TrimSpace(row.FillID) != ""
}

func countActualGovernorDecisions(rows []app.EvaluationGovernorDecisionRow) int {
	count := 0
	for i := range rows {
		if hasGovernorDecisionEvidence(rows[i]) {
			count++
		}
	}
	return count
}

func countActualExecutionRows(rows []app.EvaluationExecutionRow) int {
	count := 0
	for i := range rows {
		if hasExecutionEvidence(rows[i]) {
			count++
		}
	}
	return count
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func paginateRows[T any](rows []T, limit int, offset int) ([]T, *ToolTruncation) {
	start := min(offset, len(rows))
	end := min(start+limit, len(rows))
	selected := append([]T(nil), rows[start:end]...)
	total := len(rows)
	nextCursor := ""
	if end < len(rows) {
		nextCursor = strconv.Itoa(offset + len(selected))
	}
	return selected, NewTruncation(limit, len(selected), &total, nextCursor, nil)
}
