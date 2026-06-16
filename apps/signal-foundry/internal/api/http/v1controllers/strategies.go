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

type strategyWorkspaceService interface {
	ValidateDefinition(
		ctx context.Context,
		definition app.StrategyDefinitionInput,
	) (app.StrategyValidationResult, error)
	CreateVersion(
		ctx context.Context,
		params app.CreateStrategyVersionParams,
	) (*app.StrategyVersionRecord, error)
	ListVersions(ctx context.Context) ([]app.StrategyVersionRecord, error)
	GetVersion(
		ctx context.Context,
		strategyID string,
		version string,
	) (*app.StrategyVersionRecord, error)
	DuplicateVersion(
		ctx context.Context,
		strategyID string,
		version string,
	) (*app.StrategyVersionCandidate, error)
}

type StrategiesControllerDeps struct {
	dig.In

	StrategyWorkspaceService strategyWorkspaceService
	AuthMiddleware           middleware.AuthMiddleware
}

type StrategiesController struct {
	deps StrategiesControllerDeps
}

func NewStrategiesController(deps StrategiesControllerDeps) *StrategiesController {
	return &StrategiesController{deps: deps}
}

var _ handlers.StrategiesController = (*StrategiesController)(nil)

func (c *StrategiesController) CreateStrategyVersion(
	builder handlers.HandlerBuilder[*models.CreateStrategyVersionParams, *models.StrategyVersionDetail],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateStrategyVersionParams,
	) (*models.StrategyVersionDetail, error) {
		created, err := c.deps.StrategyWorkspaceService.CreateVersion(
			ctx,
			app.CreateStrategyVersionParams{
				StrategyID:       params.Payload.StrategyID,
				Version:          params.Payload.Version,
				DisplayName:      params.Payload.DisplayName,
				Notes:            params.Payload.Notes,
				ParentStrategyID: params.Payload.ParentStrategyID,
				ParentVersion:    params.Payload.ParentVersion,
				Definition:       mapStrategyDefinitionInput(params.Payload.Definition),
			},
		)
		if err != nil {
			return nil, err
		}

		response := mapStrategyVersionDetail(created)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *StrategiesController) DuplicateStrategyVersion(
	builder handlers.HandlerBuilder[*models.DuplicateStrategyVersionParams, *models.StrategyVersionCandidate],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.DuplicateStrategyVersionParams,
	) (*models.StrategyVersionCandidate, error) {
		candidate, err := c.deps.StrategyWorkspaceService.DuplicateVersion(
			ctx,
			params.StrategyID,
			params.Version,
		)
		if err != nil {
			return nil, err
		}

		response := mapStrategyVersionCandidate(candidate)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *StrategiesController) GetStrategyVersion(
	builder handlers.HandlerBuilder[*models.GetStrategyVersionParams, *models.StrategyVersionDetail],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.GetStrategyVersionParams,
	) (*models.StrategyVersionDetail, error) {
		version, err := c.deps.StrategyWorkspaceService.GetVersion(
			ctx,
			params.StrategyID,
			params.Version,
		)
		if err != nil {
			return nil, err
		}

		response := mapStrategyVersionDetail(version)
		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func (c *StrategiesController) ListStrategies(
	builder handlers.NoParamsHandlerBuilder[*models.StrategyVersionListResponse],
) http.Handler {
	inner := builder.HandleWith(
		func(ctx context.Context) (*models.StrategyVersionListResponse, error) {
			items, err := c.deps.StrategyWorkspaceService.ListVersions(ctx)
			if err != nil {
				return nil, err
			}

			responseItems := make([]*models.StrategyVersionRow, 0, len(items))
			for i := range items {
				row := mapStrategyVersionRow(items[i])
				responseItems = append(responseItems, &row)
			}

			return &models.StrategyVersionListResponse{Items: responseItems}, nil
		},
	)

	return c.deps.AuthMiddleware(inner)
}

func (c *StrategiesController) ValidateStrategy(
	builder handlers.HandlerBuilder[*models.ValidateStrategyParams, *models.StrategyValidationResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.ValidateStrategyParams,
	) (*models.StrategyValidationResponse, error) {
		result, err := c.deps.StrategyWorkspaceService.ValidateDefinition(
			ctx,
			mapStrategyDefinitionInput(params.Payload.Definition),
		)
		if err != nil {
			return nil, err
		}

		response := models.StrategyValidationResponse{Valid: result.Valid}
		response.Errors = make([]*models.StrategyFieldError, 0, len(result.Errors))
		for i := range result.Errors {
			fieldError := models.StrategyFieldError{
				Path:    result.Errors[i].Path,
				Message: result.Errors[i].Message,
			}
			response.Errors = append(response.Errors, &fieldError)
		}
		if result.Preview != nil {
			preview := mapStrategyValidationPreview(*result.Preview)
			response.Preview = &preview
		}

		return &response, nil
	})

	return c.deps.AuthMiddleware(inner)
}

func mapStrategyDefinitionInput(definition *models.StrategyDefinition) app.StrategyDefinitionInput {
	if definition == nil {
		return app.StrategyDefinitionInput{}
	}

	result := app.StrategyDefinitionInput{
		Kind:      definition.Kind,
		Timeframe: definition.Timeframe,
	}
	if definition.Instrument != nil {
		result.Instrument = app.StrategyInstrumentInput{
			Venue:      definition.Instrument.Venue,
			Symbol:     definition.Instrument.Symbol,
			AssetClass: definition.Instrument.AssetClass,
			Active:     definition.Instrument.Active,
		}
	}
	if definition.Parameters != nil {
		result.Parameters = app.StrategyParameterSummary{
			FastWindow: int(definition.Parameters.FastWindow),
			SlowWindow: int(definition.Parameters.SlowWindow),
		}
	}

	return result
}

func mapStrategyDefinition(definition app.StrategyDefinitionInput) models.StrategyDefinition {
	instrument := models.StrategyDefinitionInstrument{
		Venue:      definition.Instrument.Venue,
		Symbol:     definition.Instrument.Symbol,
		AssetClass: definition.Instrument.AssetClass,
		Active:     definition.Instrument.Active,
	}
	parameters := models.StrategyDefinitionParameters{
		FastWindow: int64(definition.Parameters.FastWindow),
		SlowWindow: int64(definition.Parameters.SlowWindow),
	}

	return models.StrategyDefinition{
		Kind:       definition.Kind,
		Instrument: &instrument,
		Timeframe:  definition.Timeframe,
		Parameters: &parameters,
	}
}

func mapStrategyInstrument(
	instrument app.StrategyInstrumentInput,
) models.StrategyDefinitionInstrument {
	return models.StrategyDefinitionInstrument{
		Venue:      instrument.Venue,
		Symbol:     instrument.Symbol,
		AssetClass: instrument.AssetClass,
		Active:     instrument.Active,
	}
}

func mapStrategyVersionRow(record app.StrategyVersionRecord) models.StrategyVersionRow {
	instrument := mapStrategyInstrument(record.Instrument)
	parameters := models.StrategyParameterSummary{
		FastWindow: int64(record.ParameterSummary.FastWindow),
		SlowWindow: int64(record.ParameterSummary.SlowWindow),
	}

	return models.StrategyVersionRow{
		StrategyID:       record.StrategyID,
		Version:          record.Version,
		DisplayName:      record.DisplayName,
		Status:           record.Status,
		SourceType:       record.SourceType,
		SourceLabel:      record.SourceLabel,
		ArtifactHash:     record.ArtifactHash,
		SchemaVersion:    record.SchemaVersion,
		Kind:             record.Kind,
		Instrument:       &instrument,
		Timeframe:        record.Timeframe,
		ParameterSummary: &parameters,
		Notes:            record.Notes,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func mapStrategyVersionDetail(record *app.StrategyVersionRecord) models.StrategyVersionDetail {
	row := mapStrategyVersionRow(*record)
	definition := mapStrategyDefinition(record.Definition)

	return models.StrategyVersionDetail{
		StrategyID:       row.StrategyID,
		Version:          row.Version,
		DisplayName:      row.DisplayName,
		Status:           row.Status,
		SourceType:       row.SourceType,
		SourceLabel:      row.SourceLabel,
		ArtifactHash:     row.ArtifactHash,
		SchemaVersion:    row.SchemaVersion,
		Kind:             row.Kind,
		Instrument:       row.Instrument,
		Timeframe:        row.Timeframe,
		ParameterSummary: row.ParameterSummary,
		Notes:            row.Notes,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Definition:       &definition,
		ParentStrategyID: record.ParentStrategyID,
		ParentVersion:    record.ParentVersion,
	}
}

func mapStrategyVersionCandidate(
	candidate *app.StrategyVersionCandidate,
) models.StrategyVersionCandidate {
	definition := mapStrategyDefinition(candidate.Definition)

	return models.StrategyVersionCandidate{
		StrategyID:       candidate.StrategyID,
		Version:          candidate.Version,
		DisplayName:      candidate.DisplayName,
		Status:           candidate.Status,
		SourceType:       candidate.SourceType,
		SourceLabel:      candidate.SourceLabel,
		Notes:            candidate.Notes,
		Definition:       &definition,
		ParentStrategyID: candidate.ParentStrategyID,
		ParentVersion:    candidate.ParentVersion,
	}
}

func mapStrategyValidationPreview(
	preview app.StrategyValidationPreview,
) models.StrategyValidationPreview {
	instrument := mapStrategyInstrument(preview.Instrument)
	parameters := models.StrategyParameterSummary{
		FastWindow: int64(preview.ParameterSummary.FastWindow),
		SlowWindow: int64(preview.ParameterSummary.SlowWindow),
	}

	return models.StrategyValidationPreview{
		SchemaVersion:    preview.SchemaVersion,
		Kind:             preview.Kind,
		Instrument:       &instrument,
		Timeframe:        preview.Timeframe,
		ParameterSummary: &parameters,
		CanonicalJson:    preview.CanonicalJSON,
		ArtifactHash:     preview.ArtifactHash,
		ExistingArtifact: preview.ExistingArtifact,
	}
}
