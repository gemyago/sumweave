package strategyassistant

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	app "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStrategyWorkspaceService struct {
	listCalls      int
	getCalls       []struct{ strategyID, version string }
	validateCalls  []app.StrategyDefinitionInput
	duplicateCalls []struct{ strategyID, version string }
	createCalls    []app.CreateStrategyVersionParams

	listFunc      func(context.Context) ([]app.StrategyVersionRecord, error)
	getFunc       func(context.Context, string, string) (*app.StrategyVersionRecord, error)
	validateFunc  func(context.Context, app.StrategyDefinitionInput) (app.StrategyValidationResult, error)
	duplicateFunc func(context.Context, string, string) (*app.StrategyVersionCandidate, error)
	createFunc    func(context.Context, app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error)
}

func (f *fakeStrategyWorkspaceService) ListVersions(ctx context.Context) ([]app.StrategyVersionRecord, error) {
	f.listCalls++
	if f.listFunc == nil {
		return nil, errors.New("unexpected list versions call")
	}
	return f.listFunc(ctx)
}

func (f *fakeStrategyWorkspaceService) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*app.StrategyVersionRecord, error) {
	f.getCalls = append(f.getCalls, struct{ strategyID, version string }{strategyID: strategyID, version: version})
	if f.getFunc == nil {
		return nil, errors.New("unexpected get version call")
	}
	return f.getFunc(ctx, strategyID, version)
}

func (f *fakeStrategyWorkspaceService) ValidateDefinition(
	ctx context.Context,
	definition app.StrategyDefinitionInput,
) (app.StrategyValidationResult, error) {
	f.validateCalls = append(f.validateCalls, definition)
	if f.validateFunc == nil {
		return app.StrategyValidationResult{}, errors.New("unexpected validate definition call")
	}
	return f.validateFunc(ctx, definition)
}

func (f *fakeStrategyWorkspaceService) DuplicateVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*app.StrategyVersionCandidate, error) {
	f.duplicateCalls = append(
		f.duplicateCalls,
		struct{ strategyID, version string }{strategyID: strategyID, version: version},
	)
	if f.duplicateFunc == nil {
		return nil, errors.New("unexpected duplicate version call")
	}
	return f.duplicateFunc(ctx, strategyID, version)
}

func (f *fakeStrategyWorkspaceService) CreateVersion(
	ctx context.Context,
	params app.CreateStrategyVersionParams,
) (*app.StrategyVersionRecord, error) {
	f.createCalls = append(f.createCalls, params)
	if f.createFunc == nil {
		return nil, errors.New("unexpected create version call")
	}
	return f.createFunc(ctx, params)
}

func TestStrategyTools(t *testing.T) {
	makeDefinition := func() StrategyDefinition {
		return StrategyDefinition{
			Kind: "moving-average-crossover",
			Instrument: StrategyInstrument{
				Venue:      "hyperliquid-perps",
				Symbol:     "BTCUSD",
				AssetClass: "crypto",
				Active:     true,
			},
			Timeframe:  "1h",
			Parameters: StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
		}
	}

	makeRecord := func(strategyID, version, status, sourceType, sourceLabel string, createdAt time.Time) app.StrategyVersionRecord {
		definition := mapStrategyDefinitionInput(makeDefinition())
		return app.StrategyVersionRecord{
			StrategyID:       strategyID,
			Version:          version,
			DisplayName:      "Momentum " + version,
			Status:           status,
			SourceType:       sourceType,
			SourceLabel:      sourceLabel,
			ArtifactHash:     "hash-" + version,
			SchemaVersion:    "strategy.dsl/v0",
			Kind:             definition.Kind,
			Instrument:       definition.Instrument,
			Timeframe:        definition.Timeframe,
			ParameterSummary: definition.Parameters,
			Notes:            "notes-" + version,
			ParentStrategyID: "base-strategy",
			ParentVersion:    "v0",
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt.Add(time.Hour),
			Definition:       definition,
		}
	}

	t.Run("list versions returns compact filtered metadata with deterministic mapping", func(t *testing.T) {
		createdAt := time.Date(2026, time.June, 16, 10, 0, 0, 0, time.UTC)
		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.listFunc = func(_ context.Context) ([]app.StrategyVersionRecord, error) {
			return []app.StrategyVersionRecord{
				makeRecord("strat-a", "v3", "ready", "human", "Human", createdAt),
				makeRecord("strat-b", "v2", "archived", "demo", "Demo example", createdAt.Add(-time.Hour)),
				makeRecord("strat-a", "v1", "ready", "ai_draft", "AI draft", createdAt.Add(-2*time.Hour)),
			}, nil
		}

		tool := newListStrategyVersionsTool(RegisterDeps{StrategyWorkspace: strategySvc})
		response, err := tool.Handler(
			nil,
			ListStrategyVersionsRequest{StrategyID: "strat-a", Status: "ready", Limit: 1},
		)
		require.NoError(t, err)
		require.Len(t, response.Items, 1)
		assert.Nil(t, response.Error)
		assert.Equal(t, StrategyVersionRow{
			StrategyID:       "strat-a",
			Version:          "v3",
			DisplayName:      "Momentum v3",
			Status:           "ready",
			SourceType:       "human",
			SourceLabel:      "Human",
			ArtifactHash:     "hash-v3",
			SchemaVersion:    "strategy.dsl/v0",
			Kind:             "moving-average-crossover",
			Instrument:       makeDefinition().Instrument,
			Timeframe:        "1h",
			ParameterSummary: StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
			Notes:            "notes-v3",
			ParentStrategyID: "base-strategy",
			ParentVersion:    "v0",
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt.Add(time.Hour),
		}, response.Items[0])
		require.NotNil(t, response.Truncation)
		assert.Equal(t, "1", response.Truncation.NextCursor)
		assert.Equal(t, "Retry with offset=1 to continue browsing strategy versions.", response.NextStepHint)
		assert.Equal(t, 1, strategySvc.listCalls)

		serialized, err := json.Marshal(response)
		require.NoError(t, err)
		assert.NotContains(t, string(serialized), "definition")
	})

	t.Run("list versions validates supported status filters", func(t *testing.T) {
		tool := newListStrategyVersionsTool(
			RegisterDeps{StrategyWorkspace: &fakeStrategyWorkspaceService{}},
		)

		response, err := tool.Handler(
			nil,
			ListStrategyVersionsRequest{Status: "draft"},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Error)
		assert.Equal(t, toolErrorCodeValidation, response.Error.Code)
		assert.Equal(
			t,
			[]ToolFieldError{{Field: "status", Message: "unsupported strategy status"}},
			response.Error.FieldErrors,
		)
	})

	t.Run("list versions handles placeholder, request validation, and internal errors", func(t *testing.T) {
		placeholderTool := newListStrategyVersionsTool(RegisterDeps{})
		placeholderResponse, err := placeholderTool.Handler(nil, ListStrategyVersionsRequest{})
		require.NoError(t, err)
		require.NotNil(t, placeholderResponse.Error)
		assert.Equal(t, toolErrorCodeNotReady, placeholderResponse.Error.Code)

		tool := newListStrategyVersionsTool(
			RegisterDeps{StrategyWorkspace: &fakeStrategyWorkspaceService{}},
		)

		validationResponse, err := tool.Handler(
			nil,
			ListStrategyVersionsRequest{Limit: -1},
		)
		require.NoError(t, err)
		require.NotNil(t, validationResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, validationResponse.Error.Code)

		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.listFunc = func(_ context.Context) ([]app.StrategyVersionRecord, error) {
			return nil, errors.New("boom")
		}

		internalTool := newListStrategyVersionsTool(RegisterDeps{StrategyWorkspace: strategySvc})
		internalResponse, err := internalTool.Handler(
			nil,
			ListStrategyVersionsRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, internalResponse.Error)
		assert.Equal(t, toolErrorCodeInternal, internalResponse.Error.Code)
	})

	t.Run("get version returns saved definition detail and not found deterministically", func(t *testing.T) {
		createdAt := time.Date(2026, time.June, 16, 11, 0, 0, 0, time.UTC)
		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.getFunc = func(_ context.Context, strategyID, version string) (*app.StrategyVersionRecord, error) {
			if strategyID == "missing" {
				return nil, app.NewErrNotFound("strategy version", strategyID+"/"+version)
			}
			record := makeRecord(strategyID, version, "ready", "demo", "Demo example", createdAt)
			return &record, nil
		}

		tool := newGetStrategyVersionTool(RegisterDeps{StrategyWorkspace: strategySvc})
		response, err := tool.Handler(
			nil,
			GetStrategyVersionRequest{StrategyID: "strat-a", Version: "v2"},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Version)
		assert.Nil(t, response.Error)
		assert.Equal(t, StrategyVersionDetail{
			StrategyVersionRow: StrategyVersionRow{
				StrategyID:       "strat-a",
				Version:          "v2",
				DisplayName:      "Momentum v2",
				Status:           "ready",
				SourceType:       "demo",
				SourceLabel:      "Demo example",
				ArtifactHash:     "hash-v2",
				SchemaVersion:    "strategy.dsl/v0",
				Kind:             "moving-average-crossover",
				Instrument:       makeDefinition().Instrument,
				Timeframe:        "1h",
				ParameterSummary: StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
				Notes:            "notes-v2",
				ParentStrategyID: "base-strategy",
				ParentVersion:    "v0",
				CreatedAt:        createdAt,
				UpdatedAt:        createdAt.Add(time.Hour),
			},
			Definition: makeDefinition(),
		}, *response.Version)

		notFoundResponse, err := tool.Handler(
			nil,
			GetStrategyVersionRequest{StrategyID: "missing", Version: "v9"},
		)
		require.NoError(t, err)
		require.NotNil(t, notFoundResponse.Error)
		assert.Equal(t, toolErrorCodeNotFound, notFoundResponse.Error.Code)
		assert.Nil(t, notFoundResponse.Version)

		validationResponse, err := tool.Handler(
			nil,
			GetStrategyVersionRequest{StrategyID: " ", Version: "v1"},
		)
		require.NoError(t, err)
		require.NotNil(t, validationResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, validationResponse.Error.Code)
	})

	t.Run("validate definition delegates strictly and never persists", func(t *testing.T) {
		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.validateFunc = func(_ context.Context, definition app.StrategyDefinitionInput) (app.StrategyValidationResult, error) {
			assert.Equal(t, mapStrategyDefinitionInput(makeDefinition()), definition)
			return app.StrategyValidationResult{
				Valid: true,
				Preview: &app.StrategyValidationPreview{
					SchemaVersion:    "strategy.dsl/v0",
					Kind:             definition.Kind,
					Instrument:       definition.Instrument,
					Timeframe:        definition.Timeframe,
					ParameterSummary: definition.Parameters,
					CanonicalJSON:    `{"kind":"moving-average-crossover"}`,
					ArtifactHash:     "hash-preview",
					ExistingArtifact: true,
				},
			}, nil
		}

		tool := newValidateStrategyDefinitionTool(RegisterDeps{StrategyWorkspace: strategySvc})
		response, err := tool.Handler(
			nil,
			ValidateStrategyDefinitionRequest{Definition: makeDefinition()},
		)
		require.NoError(t, err)
		assert.True(t, response.Valid)
		require.NotNil(t, response.Preview)
		assert.Equal(t, &StrategyValidationPreview{
			SchemaVersion:    "strategy.dsl/v0",
			Kind:             "moving-average-crossover",
			Instrument:       makeDefinition().Instrument,
			Timeframe:        "1h",
			ParameterSummary: StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
			CanonicalJSON:    `{"kind":"moving-average-crossover"}`,
			ArtifactHash:     "hash-preview",
			ExistingArtifact: true,
		}, response.Preview)
		assert.Len(t, strategySvc.validateCalls, 1)
		assert.Empty(t, strategySvc.createCalls)

		strategySvc.validateFunc = func(_ context.Context, _ app.StrategyDefinitionInput) (app.StrategyValidationResult, error) {
			return app.StrategyValidationResult{
				Valid: false,
				Errors: []app.StrategyFieldError{
					{Path: "definition.kind", Message: "kind is invalid"},
					{Path: "definition.parameters.fastWindow", Message: "must be positive"},
				},
			}, nil
		}

		invalidResponse, err := tool.Handler(
			nil,
			ValidateStrategyDefinitionRequest{Definition: makeDefinition()},
		)
		require.NoError(t, err)
		assert.False(t, invalidResponse.Valid)
		require.NotNil(t, invalidResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, invalidResponse.Error.Code)
		assert.Equal(t, []ToolFieldError{
			{Field: "definition.kind", Message: "kind is invalid"},
			{Field: "definition.parameters.fastWindow", Message: "must be positive"},
		}, invalidResponse.Error.FieldErrors)
		assert.Empty(t, strategySvc.createCalls)

		strategySvc.validateFunc = func(_ context.Context, _ app.StrategyDefinitionInput) (app.StrategyValidationResult, error) {
			return app.StrategyValidationResult{}, errors.New("boom")
		}

		internalResponse, err := tool.Handler(
			nil,
			ValidateStrategyDefinitionRequest{Definition: makeDefinition()},
		)
		require.NoError(t, err)
		require.NotNil(t, internalResponse.Error)
		assert.Equal(t, toolErrorCodeInternal, internalResponse.Error.Code)
	})

	t.Run("duplicate version returns editable candidate and deterministic errors", func(t *testing.T) {
		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.duplicateFunc = func(_ context.Context, strategyID, version string) (*app.StrategyVersionCandidate, error) {
			if strategyID == "missing" {
				return nil, app.NewErrNotFound("strategy version", strategyID+"/"+version)
			}
			candidate := app.StrategyVersionCandidate{
				StrategyID:       strategyID,
				Version:          "",
				DisplayName:      "Momentum clone",
				Status:           "draft",
				SourceType:       "human",
				SourceLabel:      "Human",
				Notes:            "carry notes",
				ParentStrategyID: strategyID,
				ParentVersion:    version,
				Definition:       mapStrategyDefinitionInput(makeDefinition()),
			}
			return &candidate, nil
		}

		tool := newDuplicateStrategyVersionTool(RegisterDeps{StrategyWorkspace: strategySvc})
		response, err := tool.Handler(
			nil,
			DuplicateStrategyVersionRequest{StrategyID: "strat-a", Version: "v2"},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Candidate)
		assert.Equal(t, &StrategyVersionCandidate{
			StrategyID:       "strat-a",
			Version:          "",
			DisplayName:      "Momentum clone",
			Status:           "draft",
			SourceType:       "human",
			SourceLabel:      "Human",
			Notes:            "carry notes",
			ParentStrategyID: "strat-a",
			ParentVersion:    "v2",
			Definition:       makeDefinition(),
		}, response.Candidate)

		notFoundResponse, err := tool.Handler(
			nil,
			DuplicateStrategyVersionRequest{StrategyID: "missing", Version: "v2"},
		)
		require.NoError(t, err)
		require.NotNil(t, notFoundResponse.Error)
		assert.Equal(t, toolErrorCodeNotFound, notFoundResponse.Error.Code)

		validationResponse, err := tool.Handler(
			nil,
			DuplicateStrategyVersionRequest{StrategyID: "strat-a", Version: " "},
		)
		require.NoError(t, err)
		require.NotNil(t, validationResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, validationResponse.Error.Code)
	})

	t.Run("create version uses workspace save path and maps success and failures", func(t *testing.T) {
		createdAt := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)
		strategySvc := &fakeStrategyWorkspaceService{}
		strategySvc.createFunc = func(_ context.Context, params app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error) {
			if params.StrategyID == "conflict" {
				return nil, app.NewErrConflict("strategy version", "already exists")
			}
			if params.StrategyID == "invalid" {
				return nil, app.NewErrInvalidInput(
					"definition",
					"strategy validation failed: definition.kind: unsupported",
				)
			}
			record := makeRecord(params.StrategyID, params.Version, "ready", "human", "Human", createdAt)
			record.DisplayName = params.DisplayName
			record.Notes = params.Notes
			record.ParentStrategyID = params.ParentStrategyID
			record.ParentVersion = params.ParentVersion
			record.Definition = params.Definition
			return &record, nil
		}

		request := CreateStrategyVersionRequest{
			StrategyID:       "strat-a",
			Version:          "v4",
			DisplayName:      "Momentum v4",
			Notes:            "alpha notes",
			ParentStrategyID: "strat-a",
			ParentVersion:    "v3",
			Definition:       makeDefinition(),
		}

		tool := newCreateStrategyVersionTool(RegisterDeps{StrategyWorkspace: strategySvc})
		response, err := tool.Handler(nil, request)
		require.NoError(t, err)
		require.Len(t, strategySvc.createCalls, 1)
		assert.Equal(t, app.CreateStrategyVersionParams{
			StrategyID:       "strat-a",
			Version:          "v4",
			DisplayName:      "Momentum v4",
			Notes:            "alpha notes",
			ParentStrategyID: "strat-a",
			ParentVersion:    "v3",
			Definition:       mapStrategyDefinitionInput(makeDefinition()),
		}, strategySvc.createCalls[0])
		require.NotNil(t, response.Version)
		assert.Equal(t, "alpha notes", response.Version.Notes)
		assert.Equal(t, "strat-a", response.Version.ParentStrategyID)
		assert.Equal(t, "v3", response.Version.ParentVersion)
		assert.Equal(t, makeDefinition(), response.Version.Definition)

		conflictResponse, err := tool.Handler(
			nil,
			CreateStrategyVersionRequest{
				StrategyID:  "conflict",
				Version:     "v1",
				DisplayName: "Name",
				Definition:  makeDefinition(),
			},
		)
		require.NoError(t, err)
		require.NotNil(t, conflictResponse.Error)
		assert.Equal(t, toolErrorCodeConflict, conflictResponse.Error.Code)

		invalidResponse, err := tool.Handler(
			nil,
			CreateStrategyVersionRequest{
				StrategyID:  "invalid",
				Version:     "v1",
				DisplayName: "Name",
				Definition:  makeDefinition(),
			},
		)
		require.NoError(t, err)
		require.NotNil(t, invalidResponse.Error)
		assert.Equal(t, toolErrorCodeValidation, invalidResponse.Error.Code)
		assert.Equal(
			t,
			[]ToolFieldError{{
				Field:   "definition",
				Message: "strategy validation failed: definition.kind: unsupported",
			}},
			invalidResponse.Error.FieldErrors,
		)

		strategySvc.createFunc = func(_ context.Context, _ app.CreateStrategyVersionParams) (*app.StrategyVersionRecord, error) {
			return nil, errors.New("boom")
		}

		internalResponse, err := tool.Handler(
			nil,
			CreateStrategyVersionRequest{
				StrategyID:  "strat-a",
				Version:     "v5",
				DisplayName: "Name",
				Definition:  makeDefinition(),
			},
		)
		require.NoError(t, err)
		require.NotNil(t, internalResponse.Error)
		assert.Equal(t, toolErrorCodeInternal, internalResponse.Error.Code)
	})

	t.Run("strategy tool placeholder behavior remains when workspace service is absent", func(t *testing.T) {
		response, err := newValidateStrategyDefinitionTool(RegisterDeps{}).Handler(
			nil,
			ValidateStrategyDefinitionRequest{},
		)
		require.NoError(t, err)
		require.NotNil(t, response.Error)
		assert.Equal(t, toolErrorCodeNotReady, response.Error.Code)
	})
}
