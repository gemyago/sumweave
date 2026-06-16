package strategy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	strategyVersionIdentityStrategyIDColumn      = "strategy_id"
	strategyVersionIdentityStrategyVersionColumn = "strategy_version"
)

// ErrStrategyVersionNotFound marks a missing persisted strategy version.
var ErrStrategyVersionNotFound = errors.New("strategy version not found")

// ErrStrategyVersionImmutableConflict marks attempts to change saved version content.
var ErrStrategyVersionImmutableConflict = errors.New("strategy version already exists with different immutable content")

// VersionStatus is the persisted lifecycle state of a saved strategy version.
type VersionStatus string

const (
	VersionStatusReady    VersionStatus = "ready"
	VersionStatusArchived VersionStatus = "archived"
)

// VersionSourceType marks where a strategy version originated.
type VersionSourceType string

const (
	VersionSourceTypeHuman   VersionSourceType = "human"
	VersionSourceTypeDemo    VersionSourceType = "demo"
	VersionSourceTypeAIDraft VersionSourceType = "ai_draft"
)

// VersionCandidateStatusDraft marks a non-persisted editable candidate.
const VersionCandidateStatusDraft = "draft"

// Version stores product-facing metadata for an immutable saved strategy artifact.
type Version struct {
	StrategyID            string
	Version               string
	DisplayName           string
	Status                VersionStatus
	SourceType            VersionSourceType
	ArtifactHash          string
	ArtifactSchemaVersion string
	Strategy              canonicalStrategyDSLV0
	Notes                 string
	ParentStrategyID      string
	ParentVersion         string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// VersionCandidate returns a draftable human-editable response without persistence.
type VersionCandidate struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Status           string
	SourceType       VersionSourceType
	Strategy         canonicalStrategyDSLV0
	Notes            string
	ParentStrategyID string
	ParentVersion    string
}

// CreateVersionFromDSLV0Params configures immutable version creation from strict DSL v0.
type CreateVersionFromDSLV0Params struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Status           VersionStatus
	SourceType       VersionSourceType
	Notes            string
	ParentStrategyID string
	ParentVersion    string
	RawStrategyDSLV0 []byte
}

type strategyVersionArtifactStore interface {
	Create(ctx context.Context, raw []byte) (Artifact, error)
	Get(ctx context.Context, hash string) (*Artifact, error)
}

// VersionRegistryServiceDeps configures immutable version registry persistence.
type VersionRegistryServiceDeps struct {
	ArtifactStore strategyVersionArtifactStore
	TablePrefix   string
}

// VersionRegistryService persists immutable strategy version rows backed by artifacts.
type VersionRegistryService struct {
	db            *gorm.DB
	artifactStore strategyVersionArtifactStore
}

type strategyVersionModel struct {
	StrategyID            string    `gorm:"column:strategy_id;size:255;not null;uniqueIndex:idx_strategy_versions_identity,priority:1"`
	StrategyVersion       string    `gorm:"column:strategy_version;size:255;not null;uniqueIndex:idx_strategy_versions_identity,priority:2"`
	DisplayName           string    `gorm:"column:display_name;size:255;not null"`
	Status                string    `gorm:"column:status;size:32;not null"`
	SourceType            string    `gorm:"column:source_type;size:32;not null"`
	ArtifactHash          string    `gorm:"column:artifact_hash;size:64;not null;index:idx_strategy_versions_artifact_hash"`
	ArtifactSchemaVersion string    `gorm:"column:artifact_schema_version;size:64;not null"`
	StrategyKind          string    `gorm:"column:strategy_kind;size:64;not null"`
	InstrumentVenue       string    `gorm:"column:instrument_venue;size:255;not null"`
	InstrumentSymbol      string    `gorm:"column:instrument_symbol;size:255;not null"`
	InstrumentAssetClass  string    `gorm:"column:instrument_asset_class;size:64;not null"`
	InstrumentActive      bool      `gorm:"column:instrument_active;not null"`
	Timeframe             string    `gorm:"column:timeframe;size:32;not null"`
	FastWindow            int       `gorm:"column:fast_window;not null"`
	SlowWindow            int       `gorm:"column:slow_window;not null"`
	Notes                 string    `gorm:"column:notes;type:text;not null"`
	ParentStrategyID      *string   `gorm:"column:parent_strategy_id;size:255"`
	ParentStrategyVersion *string   `gorm:"column:parent_strategy_version;size:255"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null;autoCreateTime;autoUpdateTime"`
}

func (strategyVersionModel) TableName(namer schema.Namer) string {
	return namer.TableName("strategy_versions")
}

// NewVersionRegistryService opens a database-backed version registry service.
func NewVersionRegistryService(
	dsn string,
	deps VersionRegistryServiceDeps,
) (*VersionRegistryService, error) {
	if dsn == "" {
		return nil, errors.New("dsn is required")
	}
	if deps.ArtifactStore == nil {
		return nil, errors.New("artifact store is required")
	}

	cfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(gormsignalfoundry.GormSignalFoundryTablesOpts{
		TablePrefix:    deps.TablePrefix,
		TranslateError: true,
	})
	cfg.NowFunc = func() time.Time {
		return time.Now().UTC()
	}

	db, err := gorm.Open(gormsignalfoundry.NewGormDialector(dsn), cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &VersionRegistryService{
		db:            db,
		artifactStore: deps.ArtifactStore,
	}, nil
}

// AutoMigrate creates or updates the immutable strategy version relational schema.
func (s *VersionRegistryService) AutoMigrate() error {
	return s.db.AutoMigrate(&strategyVersionModel{})
}

// CreateVersionFromDSLV0 validates, canonicalizes, persists, or reuses an immutable version row.
func (s *VersionRegistryService) CreateVersionFromDSLV0(
	ctx context.Context,
	params CreateVersionFromDSLV0Params,
) (Version, error) {
	if err := ctx.Err(); err != nil {
		return Version{}, err
	}

	canonicalParams, err := canonicalizeCreateStrategyVersionFromDSLV0Params(params)
	if err != nil {
		return Version{}, err
	}

	artifact, err := s.artifactStore.Create(ctx, canonicalParams.RawStrategyDSLV0)
	if err != nil {
		return Version{}, err
	}

	if canonicalParams.ParentStrategyID != "" {
		parentErr := s.ensureParentExists(
			ctx,
			canonicalParams.ParentStrategyID,
			canonicalParams.ParentVersion,
		)
		if parentErr != nil {
			return Version{}, parentErr
		}
	}

	requestedModel := strategyVersionModelFromArtifact(canonicalParams, artifact)
	createErr := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: strategyVersionIdentityStrategyIDColumn},
			{Name: strategyVersionIdentityStrategyVersionColumn},
		},
		DoNothing: true,
	}).Create(&requestedModel).Error
	if createErr != nil {
		return Version{}, fmt.Errorf("create strategy version: %w", createErr)
	}

	persisted, err := s.findModelByIdentity(ctx, canonicalParams.StrategyID, canonicalParams.Version)
	if err != nil {
		return Version{}, err
	}
	if !strategyVersionModelsEqualIgnoringTimestamps(persisted, requestedModel) {
		return Version{}, fmt.Errorf(
			"%w: %s/%s",
			ErrStrategyVersionImmutableConflict,
			canonicalParams.StrategyID,
			canonicalParams.Version,
		)
	}

	return strategyVersionModelToValue(persisted)
}

// GetVersion reads a persisted immutable strategy version by identity.
func (s *VersionRegistryService) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*Version, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	canonicalStrategyID, canonicalVersion, err := canonicalizeStrategyVersionIdentity(
		strategyID,
		version,
	)
	if err != nil {
		return nil, err
	}

	model, err := s.findModelByIdentity(ctx, canonicalStrategyID, canonicalVersion)
	if err != nil {
		return nil, err
	}

	value, err := strategyVersionModelToValue(model)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

// ListVersions returns persisted immutable strategy versions in stable newest-first order.
func (s *VersionRegistryService) ListVersions(ctx context.Context) ([]Version, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var models []strategyVersionModel
	if err := s.db.WithContext(ctx).
		Order("created_at DESC").
		Order("strategy_id ASC").
		Order("strategy_version DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list strategy versions: %w", err)
	}

	versions := make([]Version, 0, len(models))
	for _, model := range models {
		value, err := strategyVersionModelToValue(model)
		if err != nil {
			return nil, err
		}
		versions = append(versions, value)
	}

	return versions, nil
}

// DuplicateVersionAsHumanCandidate returns a non-persisted human draft linked to the parent row.
func (s *VersionRegistryService) DuplicateVersionAsHumanCandidate(
	ctx context.Context,
	strategyID string,
	version string,
) (VersionCandidate, error) {
	persisted, err := s.GetVersion(ctx, strategyID, version)
	if err != nil {
		return VersionCandidate{}, err
	}

	return VersionCandidate{
		StrategyID:       persisted.StrategyID,
		Version:          "",
		DisplayName:      persisted.DisplayName,
		Status:           VersionCandidateStatusDraft,
		SourceType:       VersionSourceTypeHuman,
		Strategy:         persisted.Strategy,
		Notes:            persisted.Notes,
		ParentStrategyID: persisted.StrategyID,
		ParentVersion:    persisted.Version,
	}, nil
}

// EnsureDemoStrategyVersions idempotently persists the fixed demo strategy versions.
func (s *VersionRegistryService) EnsureDemoStrategyVersions(
	ctx context.Context,
) ([]Version, error) {
	definitions := makeStrategyDemoVersionDefinitions()
	created := make([]Version, 0, len(definitions))
	for _, definition := range definitions {
		version, err := s.CreateVersionFromDSLV0(ctx, CreateVersionFromDSLV0Params{
			StrategyID:       definition.StrategyID,
			Version:          definition.Version,
			DisplayName:      definition.DisplayName,
			Status:           VersionStatusReady,
			SourceType:       VersionSourceTypeDemo,
			Notes:            definition.Notes,
			RawStrategyDSLV0: append([]byte(nil), definition.RawStrategyDSLV0...),
		})
		if err != nil {
			return nil, err
		}
		created = append(created, version)
	}

	return created, nil
}

type canonicalCreateStrategyVersionFromDSLV0Params struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Status           VersionStatus
	SourceType       VersionSourceType
	Notes            string
	ParentStrategyID string
	ParentVersion    string
	RawStrategyDSLV0 []byte
}

type strategyDemoVersionDefinition struct {
	StrategyID       string
	Version          string
	DisplayName      string
	Notes            string
	RawStrategyDSLV0 []byte
}

const demoStrategyVersionNotes = "Demo example only; not a recommendation. Evaluation requires matching local historical data to be available."

func makeStrategyDemoVersionDefinitions() []strategyDemoVersionDefinition {
	return []strategyDemoVersionDefinition{
		{
			StrategyID:  "demo-btcusdt-1h-moving-average-crossover",
			Version:     "v1",
			DisplayName: "Demo BTCUSDT 1h moving average crossover",
			Notes:       demoStrategyVersionNotes,
			RawStrategyDSLV0: []byte(`{
				"kind": "moving-average-crossover",
				"instrument": {
					"venue": "binance",
					"symbol": "BTCUSDT",
					"assetClass": "crypto",
					"active": true
				},
				"timeframe": "1h",
				"parameters": {
					"fastWindow": 9,
					"slowWindow": 21
				}
			}`),
		},
		{
			StrategyID:  "demo-ethusdt-4h-moving-average-crossover",
			Version:     "v1",
			DisplayName: "Demo ETHUSDT 4h moving average crossover",
			Notes:       demoStrategyVersionNotes,
			RawStrategyDSLV0: []byte(`{
				"kind": "moving-average-crossover",
				"instrument": {
					"venue": "binance",
					"symbol": "ETHUSDT",
					"assetClass": "crypto",
					"active": true
				},
				"timeframe": "4h",
				"parameters": {
					"fastWindow": 12,
					"slowWindow": 48
				}
			}`),
		},
		{
			StrategyID:  "demo-solusdt-15m-moving-average-crossover",
			Version:     "v1",
			DisplayName: "Demo SOLUSDT 15m moving average crossover",
			Notes:       demoStrategyVersionNotes,
			RawStrategyDSLV0: []byte(`{
				"kind": "moving-average-crossover",
				"instrument": {
					"venue": "binance",
					"symbol": "SOLUSDT",
					"assetClass": "crypto",
					"active": true
				},
				"timeframe": "15m",
				"parameters": {
					"fastWindow": 7,
					"slowWindow": 29
				}
			}`),
		},
	}
}

func canonicalizeCreateStrategyVersionFromDSLV0Params(
	params CreateVersionFromDSLV0Params,
) (canonicalCreateStrategyVersionFromDSLV0Params, error) {
	strategyID, version, err := canonicalizeStrategyVersionIdentity(
		params.StrategyID,
		params.Version,
	)
	if err != nil {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, err
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, validationError(
			"strategy version display name is required",
		)
	}

	status, err := canonicalizeStrategyVersionStatus(params.Status)
	if err != nil {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, err
	}

	sourceType, err := canonicalizeStrategyVersionSourceType(params.SourceType)
	if err != nil {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, err
	}

	parentStrategyID := strings.TrimSpace(params.ParentStrategyID)
	parentVersion := strings.TrimSpace(params.ParentVersion)
	if (parentStrategyID == "") != (parentVersion == "") {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, validationError(
			"strategy version parent strategy id and parent version must both be set",
		)
	}
	if parentStrategyID != "" && parentStrategyID == strategyID && parentVersion == version {
		return canonicalCreateStrategyVersionFromDSLV0Params{}, validationError(
			"strategy version cannot parent itself",
		)
	}

	return canonicalCreateStrategyVersionFromDSLV0Params{
		StrategyID:       strategyID,
		Version:          version,
		DisplayName:      displayName,
		Status:           status,
		SourceType:       sourceType,
		Notes:            strings.TrimSpace(params.Notes),
		ParentStrategyID: parentStrategyID,
		ParentVersion:    parentVersion,
		RawStrategyDSLV0: append([]byte(nil), params.RawStrategyDSLV0...),
	}, nil
}

func canonicalizeStrategyVersionIdentity(strategyID string, version string) (string, string, error) {
	canonicalStrategyID := strings.TrimSpace(strategyID)
	if canonicalStrategyID == "" {
		return "", "", validationError("strategy id is required")
	}

	canonicalVersion := strings.TrimSpace(version)
	if canonicalVersion == "" {
		return "", "", validationError("strategy version is required")
	}

	return canonicalStrategyID, canonicalVersion, nil
}

func canonicalizeStrategyVersionStatus(status VersionStatus) (VersionStatus, error) {
	canonical := VersionStatus(strings.TrimSpace(string(status)))
	switch canonical {
	case VersionStatusReady, VersionStatusArchived:
		return canonical, nil
	default:
		return "", validationError(
			fmt.Sprintf("invalid strategy version status %q", status),
		)
	}
}

func canonicalizeStrategyVersionSourceType(
	sourceType VersionSourceType,
) (VersionSourceType, error) {
	canonical := VersionSourceType(strings.TrimSpace(string(sourceType)))
	switch canonical {
	case VersionSourceTypeHuman, VersionSourceTypeDemo, VersionSourceTypeAIDraft:
		return canonical, nil
	default:
		return "", validationError(
			fmt.Sprintf("invalid strategy version source type %q", sourceType),
		)
	}
}

func (s *VersionRegistryService) ensureParentExists(
	ctx context.Context,
	strategyID string,
	version string,
) error {
	_, err := s.findModelByIdentity(ctx, strategyID, version)
	if err != nil {
		if errors.Is(err, ErrStrategyVersionNotFound) {
			return validationError(
				fmt.Sprintf("strategy version parent %s/%s does not exist", strategyID, version),
			)
		}
		return err
	}

	return nil
}

func (s *VersionRegistryService) findModelByIdentity(
	ctx context.Context,
	strategyID string,
	version string,
) (strategyVersionModel, error) {
	var model strategyVersionModel
	if err := s.db.WithContext(ctx).First(
		&model,
		strategyVersionIdentityStrategyIDColumn+" = ? AND "+strategyVersionIdentityStrategyVersionColumn+" = ?",
		strategyID,
		version,
	).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return strategyVersionModel{}, fmt.Errorf(
				"%w: %s/%s",
				ErrStrategyVersionNotFound,
				strategyID,
				version,
			)
		}
		return strategyVersionModel{}, fmt.Errorf("get strategy version: %w", err)
	}

	return model, nil
}

func strategyVersionModelFromArtifact(
	params canonicalCreateStrategyVersionFromDSLV0Params,
	artifact Artifact,
) strategyVersionModel {
	return strategyVersionModel{
		StrategyID:            params.StrategyID,
		StrategyVersion:       params.Version,
		DisplayName:           params.DisplayName,
		Status:                string(params.Status),
		SourceType:            string(params.SourceType),
		ArtifactHash:          artifact.Hash,
		ArtifactSchemaVersion: artifact.SchemaVersion,
		StrategyKind:          artifact.Strategy.Kind.String(),
		InstrumentVenue:       artifact.Strategy.Instrument.Venue.String(),
		InstrumentSymbol:      artifact.Strategy.Instrument.Symbol.String(),
		InstrumentAssetClass:  artifact.Strategy.Instrument.AssetClass.String(),
		InstrumentActive:      artifact.Strategy.Instrument.Active,
		Timeframe:             artifact.Strategy.Timeframe.String(),
		FastWindow:            artifact.Strategy.Parameters.FastWindow,
		SlowWindow:            artifact.Strategy.Parameters.SlowWindow,
		Notes:                 params.Notes,
		ParentStrategyID:      optionalTrimmedStringPointer(params.ParentStrategyID),
		ParentStrategyVersion: optionalTrimmedStringPointer(params.ParentVersion),
	}
}

func strategyVersionModelsEqualIgnoringTimestamps(
	left strategyVersionModel,
	right strategyVersionModel,
) bool {
	return left.StrategyID == right.StrategyID &&
		left.StrategyVersion == right.StrategyVersion &&
		left.DisplayName == right.DisplayName &&
		left.Status == right.Status &&
		left.SourceType == right.SourceType &&
		left.ArtifactHash == right.ArtifactHash &&
		left.ArtifactSchemaVersion == right.ArtifactSchemaVersion &&
		left.StrategyKind == right.StrategyKind &&
		left.InstrumentVenue == right.InstrumentVenue &&
		left.InstrumentSymbol == right.InstrumentSymbol &&
		left.InstrumentAssetClass == right.InstrumentAssetClass &&
		left.InstrumentActive == right.InstrumentActive &&
		left.Timeframe == right.Timeframe &&
		left.FastWindow == right.FastWindow &&
		left.SlowWindow == right.SlowWindow &&
		left.Notes == right.Notes &&
		optionalStringsEqual(left.ParentStrategyID, right.ParentStrategyID) &&
		optionalStringsEqual(left.ParentStrategyVersion, right.ParentStrategyVersion)
}

func strategyVersionModelToValue(model strategyVersionModel) (Version, error) {
	strategyIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
		Instrument: domain.Instrument{
			Venue:      domain.Venue(model.InstrumentVenue),
			Symbol:     domain.Symbol(model.InstrumentSymbol),
			AssetClass: domain.AssetClass(model.InstrumentAssetClass),
			Active:     model.InstrumentActive,
		},
		Timeframe: domain.Timeframe(model.Timeframe),
		Kind:      domain.StrategyKind(model.StrategyKind),
	})
	if err != nil {
		return Version{}, fmt.Errorf("decode strategy version identity: %w", err)
	}

	parameters, err := NewMovingAverageCrossoverParams(MovingAverageCrossoverParams{
		FastWindow: model.FastWindow,
		SlowWindow: model.SlowWindow,
	})
	if err != nil {
		return Version{}, fmt.Errorf("decode strategy version parameters: %w", err)
	}

	status, err := canonicalizeStrategyVersionStatus(VersionStatus(model.Status))
	if err != nil {
		return Version{}, err
	}
	sourceType, err := canonicalizeStrategyVersionSourceType(VersionSourceType(model.SourceType))
	if err != nil {
		return Version{}, err
	}

	return Version{
		StrategyID:            model.StrategyID,
		Version:               model.StrategyVersion,
		DisplayName:           model.DisplayName,
		Status:                status,
		SourceType:            sourceType,
		ArtifactHash:          model.ArtifactHash,
		ArtifactSchemaVersion: model.ArtifactSchemaVersion,
		Strategy: canonicalStrategyDSLV0{
			Instrument: strategyIdentity.Instrument,
			Timeframe:  strategyIdentity.Timeframe,
			Kind:       strategyIdentity.Kind,
			Parameters: parameters,
		},
		Notes:            model.Notes,
		ParentStrategyID: optionalStringValue(model.ParentStrategyID),
		ParentVersion:    optionalStringValue(model.ParentStrategyVersion),
		CreatedAt:        model.CreatedAt.UTC(),
		UpdatedAt:        model.UpdatedAt.UTC(),
	}, nil
}

func optionalTrimmedStringPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func optionalStringsEqual(left *string, right *string) bool {
	return optionalStringValue(left) == optionalStringValue(right)
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
