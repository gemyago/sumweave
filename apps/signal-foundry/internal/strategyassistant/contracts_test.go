package strategyassistant

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContracts(t *testing.T) {
	t.Run("request and response DTOs use camelCase JSON fields", func(t *testing.T) {
		types := []reflect.Type{
			reflect.TypeFor[ToolFieldError](),
			reflect.TypeFor[ToolError](),
			reflect.TypeFor[ToolTruncation](),
			reflect.TypeFor[ListCandleAvailabilityRequest](),
			reflect.TypeFor[CandleAvailabilityTimeframeSummary](),
			reflect.TypeFor[CandleAvailabilityDefaultSelection](),
			reflect.TypeFor[CandleAvailabilityRow](),
			reflect.TypeFor[ListCandleAvailabilityResponse](),
			reflect.TypeFor[GetCandlesRequest](),
			reflect.TypeFor[CandleRow](),
			reflect.TypeFor[GetCandlesResponse](),
			reflect.TypeFor[GetCandleEvidenceRequest](),
			reflect.TypeFor[CandleEvidenceRow](),
			reflect.TypeFor[GetCandleEvidenceResponse](),
			reflect.TypeFor[ListStrategyVersionsRequest](),
			reflect.TypeFor[StrategyInstrument](),
			reflect.TypeFor[StrategyParameterSummary](),
			reflect.TypeFor[StrategyDefinition](),
			reflect.TypeFor[StrategyVersionRow](),
			reflect.TypeFor[ListStrategyVersionsResponse](),
			reflect.TypeFor[GetStrategyVersionRequest](),
			reflect.TypeFor[StrategyVersionDetail](),
			reflect.TypeFor[GetStrategyVersionResponse](),
			reflect.TypeFor[ValidateStrategyDefinitionRequest](),
			reflect.TypeFor[StrategyValidationPreview](),
			reflect.TypeFor[ValidateStrategyDefinitionResponse](),
			reflect.TypeFor[DuplicateStrategyVersionRequest](),
			reflect.TypeFor[StrategyVersionCandidate](),
			reflect.TypeFor[DuplicateStrategyVersionResponse](),
			reflect.TypeFor[CreateStrategyVersionRequest](),
			reflect.TypeFor[CreateStrategyVersionResponse](),
			reflect.TypeFor[RunBacktestRequest](),
			reflect.TypeFor[EvaluationRunSummary](),
			reflect.TypeFor[EvaluationMetricSummary](),
			reflect.TypeFor[EvaluationEvidenceCounts](),
			reflect.TypeFor[EvaluationAIRenderMetadata](),
			reflect.TypeFor[EvaluationDatasetReference](),
			reflect.TypeFor[EvaluationPolicyReference](),
			reflect.TypeFor[RunBacktestResponse](),
			reflect.TypeFor[ListBacktestsRequest](),
			reflect.TypeFor[EvaluationListRow](),
			reflect.TypeFor[ListBacktestsResponse](),
			reflect.TypeFor[GetBacktestDetailRequest](),
			reflect.TypeFor[EvaluationDetail](),
			reflect.TypeFor[GetBacktestDetailResponse](),
			reflect.TypeFor[GetBacktestReportRequest](),
			reflect.TypeFor[EvaluationReport](),
			reflect.TypeFor[GetBacktestReportResponse](),
			reflect.TypeFor[GetBacktestEvidenceRequest](),
			reflect.TypeFor[EvaluationTraceEvidenceRow](),
			reflect.TypeFor[EvaluationOrderIntentEvidenceRow](),
			reflect.TypeFor[EvaluationGovernorDecisionEvidenceRow](),
			reflect.TypeFor[EvaluationExecutionEvidenceRow](),
			reflect.TypeFor[EvaluationPositionSnapshotEvidenceRow](),
			reflect.TypeFor[EvaluationPortfolioSnapshotEvidenceRow](),
			reflect.TypeFor[EvaluationTraceEvidenceSection](),
			reflect.TypeFor[EvaluationOrderIntentEvidenceSection](),
			reflect.TypeFor[EvaluationGovernorDecisionEvidenceSection](),
			reflect.TypeFor[EvaluationExecutionEvidenceSection](),
			reflect.TypeFor[EvaluationPositionSnapshotEvidenceSection](),
			reflect.TypeFor[EvaluationPortfolioSnapshotEvidenceSection](),
			reflect.TypeFor[EvaluationEvidence](),
			reflect.TypeFor[GetBacktestEvidenceResponse](),
		}

		for _, dtoType := range types {
			assertCamelCaseJSONTags(t, dtoType)
		}
	})

	t.Run("recoverable errors map to deterministic safe results", func(t *testing.T) {
		errorResult, nextStepHint := resultMetaFromError(
			app.NewErrInvalidInput("definition.parameters.fastWindow", "must be positive"),
			"Fix the invalid request and retry.",
		)
		require.NotNil(t, errorResult)
		assert.Equal(t, toolErrorCodeValidation, errorResult.Code)
		assert.Equal(t, toolErrorMessageValidation, errorResult.Message)
		assert.Equal(t, []ToolFieldError{{
			Field:   "definition.parameters.fastWindow",
			Message: "must be positive",
		}}, errorResult.FieldErrors)
		assert.Equal(t, "Fix the invalid request and retry.", nextStepHint)

		notFoundResult, _ := resultMetaFromError(app.NewErrNotFound("strategy version", "missing"), "")
		require.NotNil(t, notFoundResult)
		assert.Equal(t, toolErrorCodeNotFound, notFoundResult.Code)
		assert.Equal(t, toolErrorMessageNotFound, notFoundResult.Message)

		conflictResult, _ := resultMetaFromError(app.NewErrConflict("strategy version", "already exists"), "")
		require.NotNil(t, conflictResult)
		assert.Equal(t, toolErrorCodeConflict, conflictResult.Code)
		assert.Equal(t, toolErrorMessageConflict, conflictResult.Message)

		dataUnavailableResult, _ := resultMetaFromError(
			NewDataUnavailableError(map[string]string{"stage": "replay"}),
			"Check available datasets first.",
		)
		require.NotNil(t, dataUnavailableResult)
		assert.Equal(t, toolErrorCodeDataUnavailable, dataUnavailableResult.Code)
		assert.Equal(t, toolErrorMessageDataUnavailable, dataUnavailableResult.Message)
		assert.Equal(t, map[string]string{"stage": "replay"}, dataUnavailableResult.Details)

		notReadyResult, _ := resultMetaFromError(NewNotReadyError(map[string]string{"status": "draft"}), "")
		require.NotNil(t, notReadyResult)
		assert.Equal(t, toolErrorCodeNotReady, notReadyResult.Code)
		assert.Equal(t, toolErrorMessageNotReady, notReadyResult.Message)

		unsavedVersionResult, _ := resultMetaFromError(
			NewUnsavedVersionError(map[string]string{"strategyId": "strat-1"}),
			"",
		)
		require.NotNil(t, unsavedVersionResult)
		assert.Equal(t, toolErrorCodeUnsavedVersion, unsavedVersionResult.Code)
		assert.Equal(t, toolErrorMessageUnsavedVersion, unsavedVersionResult.Message)

		missingArtifactResult, _ := resultMetaFromError(
			NewMissingArtifactError(map[string]string{"artifactHash": "abc123"}),
			"",
		)
		require.NotNil(t, missingArtifactResult)
		assert.Equal(t, toolErrorCodeMissingArtifact, missingArtifactResult.Code)
		assert.Equal(t, toolErrorMessageMissingArtifact, missingArtifactResult.Message)

		unsafeResult, _ := resultMetaFromError(
			errors.New("gorm: SQLSTATE 42P01: relation strategy_versions does not exist"),
			"",
		)
		require.NotNil(t, unsafeResult)
		assert.Equal(t, toolErrorCodeInternal, unsafeResult.Code)
		assert.Equal(t, toolErrorMessageInternal, unsafeResult.Message)
		assert.NotContains(t, strings.ToLower(unsafeResult.Message), "gorm")
		assert.NotContains(t, strings.ToLower(unsafeResult.Message), "sql")
		assert.NotContains(t, strings.ToLower(unsafeResult.Message), "strategy_versions")

		nilResult, nextStepHint := resultMetaFromError(nil, "Retry later.")
		assert.Nil(t, nilResult)
		assert.Equal(t, "Retry later.", nextStepHint)
	})

	t.Run("truncation metadata is deterministic", func(t *testing.T) {
		nextRangeStart := time.Now().UTC().Truncate(time.Second)
		truncatedTotal := 12
		exactLimitTotal := 5

		truncation := NewTruncation(5, 5, &truncatedTotal, "cursor-2", &nextRangeStart)
		require.NotNil(t, truncation)
		assert.Equal(t, &ToolTruncation{
			IsTruncated:    true,
			Limit:          5,
			Returned:       5,
			Total:          &truncatedTotal,
			NextCursor:     "cursor-2",
			NextRangeStart: &nextRangeStart,
		}, truncation)

		notTruncated := NewTruncation(5, 5, &exactLimitTotal, "", nil)
		require.NotNil(t, notTruncated)
		assert.Equal(t, &ToolTruncation{
			IsTruncated: false,
			Limit:       5,
			Returned:    5,
			Total:       &exactLimitTotal,
		}, notTruncated)

		assert.Nil(t, NewTruncation(0, 0, nil, "", nil))
	})

	t.Run("placeholder results and map cloning are stable", func(t *testing.T) {
		errorResult, nextStepHint := placeholderToolErrorResult()
		require.NotNil(t, errorResult)
		assert.Equal(t, toolErrorCodeNotReady, errorResult.Code)
		assert.Equal(t, toolErrorMessageNotReady, errorResult.Message)
		assert.Equal(t, defaultPlaceholderNextStepHint, nextStepHint)
		assert.Equal(t, map[string]string{"state": "chunk-pending"}, errorResult.Details)

		original := map[string]string{"stage": "replay"}
		cloned := cloneStringMap(original)
		require.NotNil(t, cloned)
		original["stage"] = "changed"
		assert.Equal(t, map[string]string{"stage": "replay"}, cloned)
		assert.Nil(t, cloneStringMap(nil))
	})
}

func assertCamelCaseJSONTags(t *testing.T, dtoType reflect.Type) {
	t.Helper()
	require.Equal(t, reflect.Struct, dtoType.Kind())

	for fieldIndex := range dtoType.NumField() {
		field := dtoType.Field(fieldIndex)
		if !field.IsExported() {
			continue
		}
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			assertCamelCaseJSONTags(t, field.Type)
			continue
		}

		jsonTag, hasJSONTag := field.Tag.Lookup("json")
		require.Truef(t, hasJSONTag, "%s.%s missing json tag", dtoType.Name(), field.Name)

		name := strings.Split(jsonTag, ",")[0]
		require.NotEmptyf(t, name, "%s.%s has empty json field name", dtoType.Name(), field.Name)
		assert.Equalf(
			t,
			strings.ToLower(name[:1]),
			name[:1],
			"%s.%s must use camelCase json name",
			dtoType.Name(),
			field.Name,
		)
		assert.NotContainsf(t, name, "_", "%s.%s must not use snake_case", dtoType.Name(), field.Name)
	}
}
