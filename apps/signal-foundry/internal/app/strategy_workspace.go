package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"go.uber.org/dig"
)

const (
	strategySourceLabelHuman    = "Human"
	strategySourceLabelDemo     = "Demo example"
	strategySourceLabelAIDraft  = "AI draft"
	strategyFieldPathFastWindow = "definition.parameters.fastWindow"
	strategyFieldPathSlowWindow = "definition.parameters.slowWindow"
)

type strategyArtifactLookup interface {
	Get(ctx context.Context, hash string) (*rtstrategy.Artifact, error)
}

type strategyVersionRegistry interface {
	CreateVersionFromDSLV0(
		ctx context.Context,
		params rtstrategy.CreateVersionFromDSLV0Params,
	) (rtstrategy.Version, error)
	GetVersion(ctx context.Context, strategyID string, version string) (*rtstrategy.Version, error)
	ListVersions(ctx context.Context) ([]rtstrategy.Version, error)
	DuplicateVersionAsHumanCandidate(
		ctx context.Context,
		strategyID string,
		version string,
	) (rtstrategy.VersionCandidate, error)
	EnsureDemoStrategyVersions(ctx context.Context) ([]rtstrategy.Version, error)
}

type StrategyWorkspaceServiceDeps struct {
	dig.In

	ArtifactStore   *rtstrategy.ArtifactDatabaseStore
	VersionRegistry *rtstrategy.VersionRegistryService
}

type StrategyWorkspaceService struct {
	artifactStore   strategyArtifactLookup
	versionRegistry strategyVersionRegistry
}

type StrategyDefinitionInput struct {
	Kind       string
	Instrument StrategyInstrumentInput
	Timeframe  string
	Parameters StrategyParameterSummary
}

type StrategyInstrumentInput struct {
	Venue      string
	Symbol     string
	AssetClass string
	Active     bool
}

type StrategyParameterSummary struct {
	FastWindow int
	SlowWindow int
}

type StrategyFieldError struct {
	Path    string
	Message string
}

type StrategyValidationPreview struct {
	SchemaVersion    string
	Kind             string
	Instrument       StrategyInstrumentInput
	Timeframe        string
	ParameterSummary StrategyParameterSummary
	CanonicalJSON    string
	ArtifactHash     string
	ExistingArtifact bool
}

type StrategyValidationResult struct {
	Valid   bool
	Preview *StrategyValidationPreview
	Errors  []StrategyFieldError
}

type CreateStrategyVersionParams struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Notes            string
	ParentStrategyID string
	ParentVersion    string
	Definition       StrategyDefinitionInput
}

type StrategyVersionRecord struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Status           string
	SourceType       string
	SourceLabel      string
	ArtifactHash     string
	SchemaVersion    string
	Kind             string
	Instrument       StrategyInstrumentInput
	Timeframe        string
	ParameterSummary StrategyParameterSummary
	Notes            string
	ParentStrategyID string
	ParentVersion    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Definition       StrategyDefinitionInput
}

type StrategyVersionCandidate struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Status           string
	SourceType       string
	SourceLabel      string
	Notes            string
	ParentStrategyID string
	ParentVersion    string
	Definition       StrategyDefinitionInput
}

type strategyValidationError struct {
	Errors []StrategyFieldError
}

func (p *strategyValidationError) Error() string {
	if len(p.Errors) == 0 {
		return "strategy validation failed"
	}

	return fmt.Sprintf("strategy validation failed: %s: %s", p.Errors[0].Path, p.Errors[0].Message)
}

func NewStrategyWorkspaceService(
	deps StrategyWorkspaceServiceDeps,
) (*StrategyWorkspaceService, error) {
	if deps.ArtifactStore == nil {
		return nil, errors.New("strategy artifact store is required")
	}
	if deps.VersionRegistry == nil {
		return nil, errors.New("strategy version registry is required")
	}

	return &StrategyWorkspaceService{
		artifactStore:   deps.ArtifactStore,
		versionRegistry: deps.VersionRegistry,
	}, nil
}

func (s *StrategyWorkspaceService) ValidateDefinition(
	ctx context.Context,
	definition StrategyDefinitionInput,
) (StrategyValidationResult, error) {
	if err := ctx.Err(); err != nil {
		return StrategyValidationResult{}, err
	}

	payload, preview, fieldErrors := buildValidationPreview(definition)
	if len(fieldErrors) > 0 {
		return StrategyValidationResult{Valid: false, Errors: fieldErrors}, nil
	}

	artifact, artifactValidationMessage := buildArtifactFromPayload(payload)
	if artifactValidationMessage != "" {
		return StrategyValidationResult{Valid: false, Errors: []StrategyFieldError{{
			Path:    "definition",
			Message: artifactValidationMessage,
		}}}, nil
	}

	preview.SchemaVersion = artifact.SchemaVersion
	preview.CanonicalJSON = string(artifact.CanonicalJSON)
	preview.ArtifactHash = artifact.Hash

	if _, lookupErr := s.artifactStore.Get(ctx, artifact.Hash); lookupErr == nil {
		preview.ExistingArtifact = true
	} else if !errors.Is(lookupErr, rtstrategy.ErrArtifactNotFound) {
		return StrategyValidationResult{}, fmt.Errorf(
			"read strategy artifact by hash: %w",
			lookupErr,
		)
	}

	return StrategyValidationResult{
		Valid:   true,
		Preview: &preview,
		Errors:  []StrategyFieldError{},
	}, nil
}

func buildArtifactFromPayload(payload []byte) (rtstrategy.Artifact, string) {
	artifact, err := rtstrategy.NewArtifactFromDSLV0(payload)
	if err != nil {
		return rtstrategy.Artifact{}, err.Error()
	}

	return artifact, ""
}

func (s *StrategyWorkspaceService) CreateVersion(
	ctx context.Context,
	params CreateStrategyVersionParams,
) (*StrategyVersionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := s.ensureDemos(ctx); err != nil {
		return nil, err
	}

	if problem := validateCreateStrategyVersionParams(params); problem != nil {
		return nil, NewErrInvalidInput("request", problem.Error())
	}

	validation, err := s.ValidateDefinition(ctx, params.Definition)
	if err != nil {
		return nil, err
	}
	if !validation.Valid {
		return nil, NewErrInvalidInput(
			"definition",
			(&strategyValidationError{Errors: validation.Errors}).Error(),
		)
	}

	payload, _, _ := buildValidationPreview(params.Definition)
	created, err := s.versionRegistry.CreateVersionFromDSLV0(
		ctx,
		rtstrategy.CreateVersionFromDSLV0Params{
			StrategyID:       params.StrategyID,
			Version:          params.Version,
			DisplayName:      params.DisplayName,
			Status:           rtstrategy.VersionStatusReady,
			SourceType:       rtstrategy.VersionSourceTypeHuman,
			Notes:            params.Notes,
			ParentStrategyID: params.ParentStrategyID,
			ParentVersion:    params.ParentVersion,
			RawStrategyDSLV0: payload,
		},
	)
	if err != nil {
		return nil, mapStrategyPersistenceError(params.StrategyID, params.Version, err)
	}

	record := mapStrategyVersionRecord(created)
	return &record, nil
}

func (s *StrategyWorkspaceService) ListVersions(
	ctx context.Context,
) ([]StrategyVersionRecord, error) {
	if err := s.ensureDemos(ctx); err != nil {
		return nil, err
	}

	versions, err := s.versionRegistry.ListVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list strategy versions: %w", err)
	}

	records := make([]StrategyVersionRecord, 0, len(versions))
	for _, version := range versions {
		records = append(records, mapStrategyVersionRecord(version))
	}

	return records, nil
}

func (s *StrategyWorkspaceService) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*StrategyVersionRecord, error) {
	if err := s.ensureDemos(ctx); err != nil {
		return nil, err
	}

	fetched, err := s.versionRegistry.GetVersion(ctx, strategyID, version)
	if err != nil {
		return nil, mapStrategyPersistenceError(strategyID, version, err)
	}

	record := mapStrategyVersionRecord(*fetched)
	return &record, nil
}

func (s *StrategyWorkspaceService) DuplicateVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*StrategyVersionCandidate, error) {
	if err := s.ensureDemos(ctx); err != nil {
		return nil, err
	}

	candidate, err := s.versionRegistry.DuplicateVersionAsHumanCandidate(ctx, strategyID, version)
	if err != nil {
		return nil, mapStrategyPersistenceError(strategyID, version, err)
	}

	mapped := StrategyVersionCandidate{
		StrategyID:       candidate.StrategyID,
		Version:          candidate.Version,
		DisplayName:      candidate.DisplayName,
		Status:           candidate.Status,
		SourceType:       string(candidate.SourceType),
		SourceLabel:      sourceLabel(candidate.SourceType),
		Notes:            candidate.Notes,
		ParentStrategyID: candidate.ParentStrategyID,
		ParentVersion:    candidate.ParentVersion,
		Definition: StrategyDefinitionInput{
			Kind:       candidate.Strategy.Kind.String(),
			Instrument: mapStrategyInstrument(candidate.Strategy.Instrument),
			Timeframe:  candidate.Strategy.Timeframe.String(),
			Parameters: StrategyParameterSummary{
				FastWindow: candidate.Strategy.Parameters.FastWindow,
				SlowWindow: candidate.Strategy.Parameters.SlowWindow,
			},
		},
	}

	return &mapped, nil
}

func (s *StrategyWorkspaceService) ensureDemos(ctx context.Context) error {
	if _, err := s.versionRegistry.EnsureDemoStrategyVersions(ctx); err != nil {
		return fmt.Errorf("ensure demo strategy versions: %w", err)
	}

	return nil
}

func buildValidationPreview(
	definition StrategyDefinitionInput,
) ([]byte, StrategyValidationPreview, []StrategyFieldError) {
	errorsList := validateDefinition(definition)
	if len(errorsList) > 0 {
		return nil, StrategyValidationPreview{}, errorsList
	}

	canonicalKind, _ := domain.NewStrategyKind(definition.Kind)
	canonicalVenue, _ := domain.NewVenue(definition.Instrument.Venue)
	canonicalSymbol, _ := domain.NewSymbol(definition.Instrument.Symbol)
	canonicalAssetClass, _ := domain.NewAssetClass(definition.Instrument.AssetClass)
	canonicalTimeframe, _ := domain.NewTimeframe(definition.Timeframe)

	payload := struct {
		Kind       string `json:"kind"`
		Instrument struct {
			Venue      string `json:"venue"`
			Symbol     string `json:"symbol"`
			AssetClass string `json:"assetClass"`
			Active     bool   `json:"active"`
		} `json:"instrument"`
		Timeframe  string `json:"timeframe"`
		Parameters struct {
			FastWindow int `json:"fastWindow"`
			SlowWindow int `json:"slowWindow"`
		} `json:"parameters"`
	}{}
	payload.Kind = canonicalKind.String()
	payload.Instrument.Venue = canonicalVenue.String()
	payload.Instrument.Symbol = canonicalSymbol.String()
	payload.Instrument.AssetClass = canonicalAssetClass.String()
	payload.Instrument.Active = definition.Instrument.Active
	payload.Timeframe = canonicalTimeframe.String()
	payload.Parameters.FastWindow = definition.Parameters.FastWindow
	payload.Parameters.SlowWindow = definition.Parameters.SlowWindow

	rawPayload, _ := json.Marshal(payload)

	return rawPayload, StrategyValidationPreview{
		Kind: payload.Kind,
		Instrument: StrategyInstrumentInput{
			Venue:      payload.Instrument.Venue,
			Symbol:     payload.Instrument.Symbol,
			AssetClass: payload.Instrument.AssetClass,
			Active:     payload.Instrument.Active,
		},
		Timeframe: payload.Timeframe,
		ParameterSummary: StrategyParameterSummary{
			FastWindow: payload.Parameters.FastWindow,
			SlowWindow: payload.Parameters.SlowWindow,
		},
	}, nil
}

func validateCreateStrategyVersionParams(
	params CreateStrategyVersionParams,
) *strategyValidationError {
	fieldErrors := make([]StrategyFieldError, 0, 4)
	if strings.TrimSpace(params.StrategyID) == "" {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "strategyId", Message: "strategy id is required"},
		)
	}
	if strings.TrimSpace(params.Version) == "" {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "version", Message: "strategy version is required"},
		)
	}
	if strings.TrimSpace(params.DisplayName) == "" {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "displayName", Message: "display name is required"},
		)
	}
	if (strings.TrimSpace(params.ParentStrategyID) == "") != (strings.TrimSpace(params.ParentVersion) == "") {
		if strings.TrimSpace(params.ParentStrategyID) == "" {
			fieldErrors = append(
				fieldErrors,
				StrategyFieldError{
					Path:    "parentStrategyId",
					Message: "parent strategy id is required when parent version is set",
				},
			)
		}
		if strings.TrimSpace(params.ParentVersion) == "" {
			fieldErrors = append(
				fieldErrors,
				StrategyFieldError{
					Path:    "parentVersion",
					Message: "parent version is required when parent strategy id is set",
				},
			)
		}
	}
	if len(fieldErrors) == 0 {
		return nil
	}

	return &strategyValidationError{Errors: fieldErrors}
}

func validateDefinition(definition StrategyDefinitionInput) []StrategyFieldError {
	fieldErrors := make([]StrategyFieldError, 0, 8)

	if _, err := domain.NewStrategyKind(definition.Kind); err != nil {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "definition.kind", Message: err.Error()},
		)
	}
	if _, err := domain.NewVenue(definition.Instrument.Venue); err != nil {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "definition.instrument.venue", Message: err.Error()},
		)
	}
	if _, err := domain.NewSymbol(definition.Instrument.Symbol); err != nil {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "definition.instrument.symbol", Message: err.Error()},
		)
	}
	if _, err := domain.NewAssetClass(definition.Instrument.AssetClass); err != nil {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "definition.instrument.assetClass", Message: err.Error()},
		)
	}
	if _, err := domain.NewTimeframe(definition.Timeframe); err != nil {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{Path: "definition.timeframe", Message: err.Error()},
		)
	}
	if definition.Parameters.FastWindow <= 0 {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{
				Path:    strategyFieldPathFastWindow,
				Message: "moving average crossover fast window must be positive",
			},
		)
	}
	if definition.Parameters.SlowWindow <= 0 {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{
				Path:    strategyFieldPathSlowWindow,
				Message: "moving average crossover slow window must be positive",
			},
		)
	}
	if definition.Parameters.FastWindow > 0 && definition.Parameters.SlowWindow > 0 &&
		definition.Parameters.FastWindow >= definition.Parameters.SlowWindow {
		fieldErrors = append(
			fieldErrors,
			StrategyFieldError{
				Path:    strategyFieldPathFastWindow,
				Message: "moving average crossover fast window must be less than slow window",
			},
		)
	}

	return fieldErrors
}

func mapStrategyPersistenceError(strategyID string, version string, err error) error {
	switch {
	case errors.Is(err, rtstrategy.ErrValidation):
		return NewErrInvalidInput("definition", err.Error())
	case errors.Is(err, rtstrategy.ErrArtifactNotFound),
		errors.Is(err, rtstrategy.ErrStrategyVersionNotFound):
		return NewErrNotFound(
			"strategy version",
			strings.TrimSpace(strategyID)+"/"+strings.TrimSpace(version),
		)
	case errors.Is(err, rtstrategy.ErrStrategyVersionImmutableConflict):
		return NewErrConflict("strategy version", err.Error())
	default:
		return err
	}
}

func mapStrategyVersionRecord(version rtstrategy.Version) StrategyVersionRecord {
	return StrategyVersionRecord{
		StrategyID:    version.StrategyID,
		Version:       version.Version,
		DisplayName:   version.DisplayName,
		Status:        string(version.Status),
		SourceType:    string(version.SourceType),
		SourceLabel:   sourceLabel(version.SourceType),
		ArtifactHash:  version.ArtifactHash,
		SchemaVersion: version.ArtifactSchemaVersion,
		Kind:          version.Strategy.Kind.String(),
		Instrument:    mapStrategyInstrument(version.Strategy.Instrument),
		Timeframe:     version.Strategy.Timeframe.String(),
		ParameterSummary: StrategyParameterSummary{
			FastWindow: version.Strategy.Parameters.FastWindow,
			SlowWindow: version.Strategy.Parameters.SlowWindow,
		},
		Notes:            version.Notes,
		ParentStrategyID: version.ParentStrategyID,
		ParentVersion:    version.ParentVersion,
		CreatedAt:        version.CreatedAt.UTC(),
		UpdatedAt:        version.UpdatedAt.UTC(),
		Definition: StrategyDefinitionInput{
			Kind:       version.Strategy.Kind.String(),
			Instrument: mapStrategyInstrument(version.Strategy.Instrument),
			Timeframe:  version.Strategy.Timeframe.String(),
			Parameters: StrategyParameterSummary{
				FastWindow: version.Strategy.Parameters.FastWindow,
				SlowWindow: version.Strategy.Parameters.SlowWindow,
			},
		},
	}
}

func mapStrategyInstrument(instrument domain.Instrument) StrategyInstrumentInput {
	return StrategyInstrumentInput{
		Venue:      instrument.Venue.String(),
		Symbol:     instrument.Symbol.String(),
		AssetClass: instrument.AssetClass.String(),
		Active:     instrument.Active,
	}
}

func sourceLabel(sourceType rtstrategy.VersionSourceType) string {
	switch sourceType {
	case rtstrategy.VersionSourceTypeHuman:
		return strategySourceLabelHuman
	case rtstrategy.VersionSourceTypeDemo:
		return strategySourceLabelDemo
	case rtstrategy.VersionSourceTypeAIDraft:
		return strategySourceLabelAIDraft
	}

	return strategySourceLabelHuman
}
