package strategy

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/gormsignalfoundry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	// ArtifactSchemaVersion identifies the canonical strategy artifact payload version.
	ArtifactSchemaVersion = "strategy-artifact.v0"
	// ArtifactKind identifies persisted strategy definition artifacts.
	ArtifactKind               = "strategy"
	strategyArtifactHashColumn = "hash"
)

// ErrArtifactNotFound marks missing persisted strategy artifacts.
var ErrArtifactNotFound = errors.New("strategy artifact not found")

// Artifact stores an immutable canonical strategy definition artifact.
type Artifact struct {
	SchemaVersion string
	ArtifactKind  string
	Strategy      canonicalStrategyDSLV0
	CanonicalJSON []byte
	Hash          string
	CreatedAt     time.Time
}

type strategyArtifactCanonicalPayloadV0 struct {
	SchemaVersion string                              `json:"schemaVersion"`
	ArtifactKind  string                              `json:"artifactKind"`
	Strategy      strategyArtifactCanonicalStrategyV0 `json:"strategy"`
}

type strategyArtifactCanonicalStrategyV0 struct {
	Kind       string                                `json:"kind"`
	Instrument strategyArtifactCanonicalInstrumentV0 `json:"instrument"`
	Timeframe  string                                `json:"timeframe"`
	Parameters strategyArtifactCanonicalParamsV0     `json:"parameters"`
}

type strategyArtifactCanonicalInstrumentV0 struct {
	Venue      string `json:"venue"`
	Symbol     string `json:"symbol"`
	AssetClass string `json:"assetClass"`
	Active     bool   `json:"active"`
}

type strategyArtifactCanonicalParamsV0 struct {
	FastWindow int `json:"fastWindow"`
	SlowWindow int `json:"slowWindow"`
}

// NewArtifactFromDSLV0 validates a Strategy DSL v0 payload and builds its canonical artifact value.
func NewArtifactFromDSLV0(raw []byte) (Artifact, error) {
	canonicalDSL, err := parseDSLV0(raw)
	if err != nil {
		return Artifact{}, err
	}

	payload := strategyArtifactCanonicalPayloadV0{
		SchemaVersion: ArtifactSchemaVersion,
		ArtifactKind:  ArtifactKind,
		Strategy: strategyArtifactCanonicalStrategyV0{
			Kind: canonicalDSL.Kind.String(),
			Instrument: strategyArtifactCanonicalInstrumentV0{
				Venue:      canonicalDSL.Instrument.Venue.String(),
				Symbol:     canonicalDSL.Instrument.Symbol.String(),
				AssetClass: canonicalDSL.Instrument.AssetClass.String(),
				Active:     canonicalDSL.Instrument.Active,
			},
			Timeframe: canonicalDSL.Timeframe.String(),
			Parameters: strategyArtifactCanonicalParamsV0{
				FastWindow: canonicalDSL.Parameters.FastWindow,
				SlowWindow: canonicalDSL.Parameters.SlowWindow,
			},
		},
	}

	canonicalJSON, err := json.Marshal(payload)
	if err != nil {
		return Artifact{}, fmt.Errorf("marshal strategy artifact canonical payload: %w", err)
	}

	hash := sha256.Sum256(canonicalJSON)

	return Artifact{
		SchemaVersion: payload.SchemaVersion,
		ArtifactKind:  payload.ArtifactKind,
		Strategy:      canonicalDSL,
		CanonicalJSON: append([]byte(nil), canonicalJSON...),
		Hash:          hex.EncodeToString(hash[:]),
	}, nil
}

type strategyArtifactModel struct {
	Hash          string    `gorm:"column:hash;size:64;not null;primaryKey;uniqueIndex:idx_strategy_artifacts_hash"`
	SchemaVersion string    `gorm:"column:schema_version;size:64;not null"`
	ArtifactKind  string    `gorm:"column:artifact_kind;size:64;not null"`
	CanonicalJSON []byte    `gorm:"column:canonical_json;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime;index:idx_strategy_artifacts_created_at"`
}

func (strategyArtifactModel) TableName(namer schema.Namer) string {
	return namer.TableName("strategy_artifacts")
}

// ArtifactDatabaseStoreOpts configures persistence concerns for strategy artifacts.
type ArtifactDatabaseStoreOpts struct {
	TablePrefix string
}

// ArtifactDatabaseStore persists immutable strategy artifacts with GORM.
type ArtifactDatabaseStore struct {
	db *gorm.DB
}

// NewArtifactDatabaseStore builds a strategy artifact store from an existing [sql.DB] handle.
func NewArtifactDatabaseStore(
	sqlDB *sql.DB,
	dsn string,
	opts ArtifactDatabaseStoreOpts,
) (*ArtifactDatabaseStore, error) {
	if sqlDB == nil {
		return nil, errors.New("sql database is required")
	}
	if dsn == "" {
		return nil, errors.New("dsn is required")
	}

	cfg := gormsignalfoundry.NewGormConfigForSignalFoundryTables(gormsignalfoundry.GormSignalFoundryTablesOpts{
		TablePrefix:    opts.TablePrefix,
		TranslateError: true,
	})
	cfg.NowFunc = time.Now

	db, err := gorm.Open(gormsignalfoundry.NewGormDialectorWithConn(dsn, sqlDB), cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &ArtifactDatabaseStore{db: db}, nil
}

// AutoMigrate creates or updates the strategy artifact relational schema.
func (s *ArtifactDatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(&strategyArtifactModel{})
}

// Create validates, canonicalizes, and persists an immutable strategy artifact.
func (s *ArtifactDatabaseStore) Create(
	ctx context.Context,
	raw []byte,
) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}

	artifact, err := NewArtifactFromDSLV0(raw)
	if err != nil {
		return Artifact{}, err
	}

	model := strategyArtifactToModel(artifact)
	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: strategyArtifactHashColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return Artifact{}, fmt.Errorf("create strategy artifact: %w", err)
	}

	persisted, err := s.findModelByHash(ctx, artifact.Hash)
	if err != nil {
		return Artifact{}, err
	}

	return strategyArtifactModelToValue(persisted)
}

// Get reads a persisted strategy artifact by canonical hash.
func (s *ArtifactDatabaseStore) Get(
	ctx context.Context,
	hash string,
) (*Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model, err := s.findModelByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return nil, err
		}
		return nil, err
	}

	artifact, err := strategyArtifactModelToValue(model)
	if err != nil {
		return nil, err
	}

	return &artifact, nil
}

// List returns persisted strategy artifacts in a stable order.
func (s *ArtifactDatabaseStore) List(ctx context.Context) ([]Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var models []strategyArtifactModel
	if err := s.db.WithContext(ctx).
		Order("created_at ASC").
		Order("hash ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list strategy artifacts: %w", err)
	}

	artifacts := make([]Artifact, 0, len(models))
	for _, model := range models {
		artifact, err := strategyArtifactModelToValue(model)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

func (s *ArtifactDatabaseStore) findModelByHash(
	ctx context.Context,
	hash string,
) (strategyArtifactModel, error) {
	var model strategyArtifactModel
	if err := s.db.WithContext(ctx).First(&model, strategyArtifactHashColumn+" = ?", hash).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return strategyArtifactModel{}, fmt.Errorf("%w: %s", ErrArtifactNotFound, hash)
		}
		return strategyArtifactModel{}, fmt.Errorf("get strategy artifact: %w", err)
	}

	return model, nil
}

func strategyArtifactToModel(artifact Artifact) strategyArtifactModel {
	return strategyArtifactModel{
		Hash:          artifact.Hash,
		SchemaVersion: artifact.SchemaVersion,
		ArtifactKind:  artifact.ArtifactKind,
		CanonicalJSON: append([]byte(nil), artifact.CanonicalJSON...),
	}
}

func strategyArtifactModelToValue(model strategyArtifactModel) (Artifact, error) {
	var payload strategyArtifactCanonicalPayloadV0
	if err := json.Unmarshal(model.CanonicalJSON, &payload); err != nil {
		return Artifact{}, fmt.Errorf("decode strategy artifact canonical payload: %w", err)
	}

	strategyValue, err := strategyArtifactPayloadToCanonicalStrategy(payload)
	if err != nil {
		return Artifact{}, err
	}

	return Artifact{
		SchemaVersion: model.SchemaVersion,
		ArtifactKind:  model.ArtifactKind,
		Strategy:      strategyValue,
		CanonicalJSON: append([]byte(nil), model.CanonicalJSON...),
		Hash:          model.Hash,
		CreatedAt:     model.CreatedAt,
	}, nil
}

func strategyArtifactPayloadToCanonicalStrategy(
	payload strategyArtifactCanonicalPayloadV0,
) (canonicalStrategyDSLV0, error) {
	canonicalKind, err := domain.NewStrategyKind(payload.Strategy.Kind)
	if err != nil {
		return canonicalStrategyDSLV0{}, fmt.Errorf("decode strategy artifact kind: %w", err)
	}

	strategyIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
		Instrument: domain.Instrument{
			Venue:      domain.Venue(payload.Strategy.Instrument.Venue),
			Symbol:     domain.Symbol(payload.Strategy.Instrument.Symbol),
			AssetClass: domain.AssetClass(payload.Strategy.Instrument.AssetClass),
			Active:     payload.Strategy.Instrument.Active,
		},
		Timeframe: domain.Timeframe(payload.Strategy.Timeframe),
		Kind:      canonicalKind,
	})
	if err != nil {
		return canonicalStrategyDSLV0{}, fmt.Errorf("decode strategy artifact strategy: %w", err)
	}

	parameters, err := NewMovingAverageCrossoverParams(MovingAverageCrossoverParams{
		FastWindow: payload.Strategy.Parameters.FastWindow,
		SlowWindow: payload.Strategy.Parameters.SlowWindow,
	})
	if err != nil {
		return canonicalStrategyDSLV0{}, fmt.Errorf("decode strategy artifact parameters: %w", err)
	}

	return canonicalStrategyDSLV0{
		Instrument: strategyIdentity.Instrument,
		Timeframe:  strategyIdentity.Timeframe,
		Kind:       strategyIdentity.Kind,
		Parameters: parameters,
	}, nil
}
