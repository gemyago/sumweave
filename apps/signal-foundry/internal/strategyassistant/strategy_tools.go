package strategyassistant

import (
	"fmt"
	"strconv"
	"strings"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/runtime/agent"
	"github.com/gemyago/signal-foundry/runtime/strategy"
)

const (
	defaultStrategyVersionsToolLimit = 20
	maxStrategyVersionsToolLimit     = 100
)

func handleListStrategyVersionsTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input ListStrategyVersionsRequest,
) (ListStrategyVersionsResponse, error) {
	if deps.StrategyWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return ListStrategyVersionsResponse{
			Items:        []StrategyVersionRow{},
			Error:        errResult,
			NextStepHint: nextStepHint,
		}, nil
	}

	limit, err := normalizeLimit(input.Limit, defaultStrategyVersionsToolLimit, maxStrategyVersionsToolLimit)
	if err != nil {
		return ListStrategyVersionsResponse{Items: []StrategyVersionRow{}, Error: toolErrorFrom(err)}, nil
	}

	offset, err := normalizeOffset(input.Offset)
	if err != nil {
		return ListStrategyVersionsResponse{Items: []StrategyVersionRow{}, Error: toolErrorFrom(err)}, nil
	}

	statusFilter, err := canonicalStrategyStatusFilter(input.Status)
	if err != nil {
		return ListStrategyVersionsResponse{Items: []StrategyVersionRow{}, Error: toolErrorFrom(err)}, nil
	}

	items, err := deps.StrategyWorkspace.ListVersions(toolContextContext(ctx))
	if err != nil {
		return ListStrategyVersionsResponse{Items: []StrategyVersionRow{}, Error: toolErrorFrom(err)}, nil
	}

	strategyIDFilter := strings.TrimSpace(input.StrategyID)
	filtered := make([]app.StrategyVersionRecord, 0, len(items))
	for i := range items {
		if strategyIDFilter != "" && items[i].StrategyID != strategyIDFilter {
			continue
		}
		if statusFilter != "" && items[i].Status != statusFilter {
			continue
		}
		filtered = append(filtered, items[i])
	}

	start := min(offset, len(filtered))
	end := min(start+limit, len(filtered))
	hasMore := end < len(filtered)

	rows := make([]StrategyVersionRow, end-start)
	for i := start; i < end; i++ {
		rows[i-start] = mapStrategyVersionRow(filtered[i])
	}

	response := ListStrategyVersionsResponse{Items: rows}
	if hasMore {
		nextOffset := offset + len(rows)
		response.Truncation = NewTruncation(limit, len(rows), nil, strconv.Itoa(nextOffset), nil)
		response.NextStepHint = fmt.Sprintf("Retry with offset=%d to continue browsing strategy versions.", nextOffset)
	}

	return response, nil
}

func handleGetStrategyVersionTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input GetStrategyVersionRequest,
) (GetStrategyVersionResponse, error) {
	if deps.StrategyWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return GetStrategyVersionResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	strategyID, err := requiredString(input.StrategyID, "strategyId")
	if err != nil {
		return GetStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}
	version, err := requiredString(input.Version, "version")
	if err != nil {
		return GetStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}

	record, err := deps.StrategyWorkspace.GetVersion(toolContextContext(ctx), strategyID, version)
	if err != nil {
		return GetStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}

	detail := mapStrategyVersionDetail(*record)
	return GetStrategyVersionResponse{Version: &detail}, nil
}

func handleValidateStrategyDefinitionTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input ValidateStrategyDefinitionRequest,
) (ValidateStrategyDefinitionResponse, error) {
	if deps.StrategyWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return ValidateStrategyDefinitionResponse{Valid: false, Error: errResult, NextStepHint: nextStepHint}, nil
	}

	result, err := deps.StrategyWorkspace.ValidateDefinition(
		toolContextContext(ctx),
		mapStrategyDefinitionInput(input.Definition),
	)
	if err != nil {
		return ValidateStrategyDefinitionResponse{Valid: false, Error: toolErrorFrom(err)}, nil
	}

	response := ValidateStrategyDefinitionResponse{Valid: result.Valid}
	if !result.Valid {
		response.Error = strategyValidationToolError(result.Errors)
		return response, nil
	}
	if result.Preview != nil {
		preview := mapStrategyValidationPreview(*result.Preview)
		response.Preview = &preview
	}

	return response, nil
}

func handleDuplicateStrategyVersionTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input DuplicateStrategyVersionRequest,
) (DuplicateStrategyVersionResponse, error) {
	if deps.StrategyWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return DuplicateStrategyVersionResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	strategyID, err := requiredString(input.StrategyID, "strategyId")
	if err != nil {
		return DuplicateStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}
	version, err := requiredString(input.Version, "version")
	if err != nil {
		return DuplicateStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}

	candidate, err := deps.StrategyWorkspace.DuplicateVersion(toolContextContext(ctx), strategyID, version)
	if err != nil {
		return DuplicateStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}

	mapped := mapStrategyVersionCandidate(*candidate)
	return DuplicateStrategyVersionResponse{Candidate: &mapped}, nil
}

func handleCreateStrategyVersionTool(
	ctx *agent.ToolContext,
	deps RegisterDeps,
	input CreateStrategyVersionRequest,
) (CreateStrategyVersionResponse, error) {
	if deps.StrategyWorkspace == nil {
		errResult, nextStepHint := placeholderToolErrorResult()
		return CreateStrategyVersionResponse{Error: errResult, NextStepHint: nextStepHint}, nil
	}

	created, err := deps.StrategyWorkspace.CreateVersion(
		toolContextContext(ctx),
		app.CreateStrategyVersionParams{
			StrategyID:       input.StrategyID,
			Version:          input.Version,
			DisplayName:      input.DisplayName,
			Notes:            input.Notes,
			ParentStrategyID: input.ParentStrategyID,
			ParentVersion:    input.ParentVersion,
			Definition:       mapStrategyDefinitionInput(input.Definition),
		},
	)
	if err != nil {
		return CreateStrategyVersionResponse{Error: toolErrorFrom(err)}, nil
	}

	detail := mapStrategyVersionDetail(*created)
	return CreateStrategyVersionResponse{Version: &detail}, nil
}

func requiredString(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", app.NewErrInvalidInput(field, "is required")
	}
	return trimmed, nil
}

func canonicalStrategyStatusFilter(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	status := strategy.VersionStatus(trimmed)
	switch status {
	case strategy.VersionStatusReady, strategy.VersionStatusArchived:
		return trimmed, nil
	default:
		return "", app.NewErrInvalidInput("status", "unsupported strategy status")
	}
}

func strategyValidationToolError(fieldErrors []app.StrategyFieldError) *ToolError {
	mapped := make([]ToolFieldError, 0, len(fieldErrors))
	for i := range fieldErrors {
		mapped = append(mapped, ToolFieldError{
			Field:   fieldErrors[i].Path,
			Message: fieldErrors[i].Message,
		})
	}

	return &ToolError{
		Code:        toolErrorCodeValidation,
		Message:     toolErrorMessageValidation,
		FieldErrors: mapped,
	}
}

func mapStrategyDefinitionInput(definition StrategyDefinition) app.StrategyDefinitionInput {
	return app.StrategyDefinitionInput{
		Kind:      definition.Kind,
		Timeframe: definition.Timeframe,
		Instrument: app.StrategyInstrumentInput{
			Venue:      definition.Instrument.Venue,
			Symbol:     definition.Instrument.Symbol,
			AssetClass: definition.Instrument.AssetClass,
			Active:     definition.Instrument.Active,
		},
		Parameters: app.StrategyParameterSummary{
			FastWindow: definition.Parameters.FastWindow,
			SlowWindow: definition.Parameters.SlowWindow,
		},
	}
}

func mapStrategyVersionRow(record app.StrategyVersionRecord) StrategyVersionRow {
	return StrategyVersionRow{
		StrategyID:       record.StrategyID,
		Version:          record.Version,
		DisplayName:      record.DisplayName,
		Status:           record.Status,
		SourceType:       record.SourceType,
		SourceLabel:      record.SourceLabel,
		ArtifactHash:     record.ArtifactHash,
		SchemaVersion:    record.SchemaVersion,
		Kind:             record.Kind,
		Instrument:       mapStrategyInstrument(record.Instrument),
		Timeframe:        record.Timeframe,
		ParameterSummary: mapStrategyParameterSummary(record.ParameterSummary),
		Notes:            record.Notes,
		ParentStrategyID: record.ParentStrategyID,
		ParentVersion:    record.ParentVersion,
		CreatedAt:        record.CreatedAt.UTC(),
		UpdatedAt:        record.UpdatedAt.UTC(),
	}
}

func mapStrategyVersionDetail(record app.StrategyVersionRecord) StrategyVersionDetail {
	return StrategyVersionDetail{
		StrategyVersionRow: mapStrategyVersionRow(record),
		Definition:         mapStrategyDefinition(record.Definition),
	}
}

func mapStrategyVersionCandidate(candidate app.StrategyVersionCandidate) StrategyVersionCandidate {
	return StrategyVersionCandidate{
		StrategyID:       candidate.StrategyID,
		Version:          candidate.Version,
		DisplayName:      candidate.DisplayName,
		Status:           candidate.Status,
		SourceType:       candidate.SourceType,
		SourceLabel:      candidate.SourceLabel,
		Notes:            candidate.Notes,
		ParentStrategyID: candidate.ParentStrategyID,
		ParentVersion:    candidate.ParentVersion,
		Definition:       mapStrategyDefinition(candidate.Definition),
	}
}

func mapStrategyDefinition(definition app.StrategyDefinitionInput) StrategyDefinition {
	return StrategyDefinition{
		Kind:       definition.Kind,
		Instrument: mapStrategyInstrument(definition.Instrument),
		Timeframe:  definition.Timeframe,
		Parameters: mapStrategyParameterSummary(definition.Parameters),
	}
}

func mapStrategyInstrument(instrument app.StrategyInstrumentInput) StrategyInstrument {
	return StrategyInstrument{
		Venue:      instrument.Venue,
		Symbol:     instrument.Symbol,
		AssetClass: instrument.AssetClass,
		Active:     instrument.Active,
	}
}

func mapStrategyParameterSummary(summary app.StrategyParameterSummary) StrategyParameterSummary {
	return StrategyParameterSummary{
		FastWindow: summary.FastWindow,
		SlowWindow: summary.SlowWindow,
	}
}

func mapStrategyValidationPreview(preview app.StrategyValidationPreview) StrategyValidationPreview {
	return StrategyValidationPreview{
		SchemaVersion:    preview.SchemaVersion,
		Kind:             preview.Kind,
		Instrument:       mapStrategyInstrument(preview.Instrument),
		Timeframe:        preview.Timeframe,
		ParameterSummary: mapStrategyParameterSummary(preview.ParameterSummary),
		CanonicalJSON:    preview.CanonicalJSON,
		ArtifactHash:     preview.ArtifactHash,
		ExistingArtifact: preview.ExistingArtifact,
	}
}
