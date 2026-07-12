package governor

import (
	"context"
	"database/sql"
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
	governorPolicyArtifactHashColumn   = "hash"
	governorPolicyActiveScopeColumn    = "scope"
	governorPolicyActiveScopePaper     = policyArtifactModePaper
	governorPolicySelectorUpdatedAtCol = "updated_at"
)

// ErrArtifactNotFound marks missing persisted governor policy artifacts.
var ErrArtifactNotFound = errors.New("governor policy artifact not found")

// Artifact stores an immutable canonical governor policy artifact.
type Artifact struct {
	SchemaVersion string
	ArtifactKind  string
	Mode          string
	Policy        Policy
	CanonicalJSON []byte
	Hash          string
	CreatedAt     time.Time
}

// NewArtifactFromPolicyV0 validates a GovernorPolicy v0 payload and builds its canonical artifact value.
func NewArtifactFromPolicyV0(raw []byte) (Artifact, error) {
	canonicalArtifact, err := canonicalizePolicyArtifactV0(raw)
	if err != nil {
		return Artifact{}, err
	}

	return Artifact{
		SchemaVersion: canonicalArtifact.SchemaVersion,
		ArtifactKind:  canonicalArtifact.ArtifactKind,
		Mode:          canonicalArtifact.Mode,
		Policy:        canonicalArtifact.Policy,
		CanonicalJSON: append([]byte(nil), canonicalArtifact.CanonicalJSON...),
		Hash:          canonicalArtifact.Hash,
	}, nil
}

type governorPolicyArtifactModel struct {
	Hash          string    `gorm:"column:hash;size:64;not null;primaryKey;uniqueIndex:idx_governor_policy_artifacts_hash"`
	SchemaVersion string    `gorm:"column:schema_version;size:64;not null"`
	ArtifactKind  string    `gorm:"column:artifact_kind;size:64;not null"`
	Mode          string    `gorm:"column:mode;size:32;not null"`
	CanonicalJSON []byte    `gorm:"column:canonical_json;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (governorPolicyArtifactModel) TableName(namer schema.Namer) string {
	return namer.TableName("governor_policy_artifacts")
}

type governorPolicyActiveSelectorModel struct {
	Scope      string    `gorm:"column:scope;size:32;not null;primaryKey;uniqueIndex:idx_governor_policy_active_selectors_scope"`
	PolicyHash string    `gorm:"column:policy_hash;size:64;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (governorPolicyActiveSelectorModel) TableName(namer schema.Namer) string {
	return namer.TableName("governor_policy_active_selectors")
}

// ArtifactDatabaseStoreOpts configures persistence concerns for governor policy artifacts.
type ArtifactDatabaseStoreOpts struct {
	TablePrefix string
}

// ArtifactDatabaseStore persists immutable governor policy artifacts with GORM.
type ArtifactDatabaseStore struct {
	db *gorm.DB
}

// NewArtifactDatabaseStore builds a governor policy artifact store from an existing [sql.DB] handle.
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

// AutoMigrate creates or updates the governor policy artifact relational schema.
func (s *ArtifactDatabaseStore) AutoMigrate() error {
	return s.db.AutoMigrate(&governorPolicyArtifactModel{}, &governorPolicyActiveSelectorModel{})
}

// Create validates, canonicalizes, and persists an immutable governor policy artifact.
func (s *ArtifactDatabaseStore) Create(
	ctx context.Context,
	raw []byte,
) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}

	artifact, err := NewArtifactFromPolicyV0(raw)
	if err != nil {
		return Artifact{}, err
	}

	model := governorPolicyArtifactToModel(artifact)
	if err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: governorPolicyArtifactHashColumn}},
		DoNothing: true,
	}).Create(&model).Error; err != nil {
		return Artifact{}, fmt.Errorf("create governor policy artifact: %w", err)
	}

	persisted, err := s.findArtifactModelByHash(ctx, s.db.WithContext(ctx), artifact.Hash)
	if err != nil {
		return Artifact{}, err
	}

	return governorPolicyArtifactModelToValue(persisted)
}

// CreateWithActivate persists or reuses a paper governor policy artifact and activates it.
func (s *ArtifactDatabaseStore) CreateWithActivate(
	ctx context.Context,
	raw []byte,
) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}

	artifact, err := NewArtifactFromPolicyV0(raw)
	if err != nil {
		return Artifact{}, err
	}

	var persisted governorPolicyArtifactModel
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := governorPolicyArtifactToModel(artifact)
		if createErr := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: governorPolicyArtifactHashColumn}},
			DoNothing: true,
		}).Create(&model).Error; createErr != nil {
			return fmt.Errorf("create governor policy artifact: %w", createErr)
		}

		selector := governorPolicyActiveSelectorModel{
			Scope:      governorPolicyActiveScopePaper,
			PolicyHash: artifact.Hash,
		}
		if createErr := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: governorPolicyActiveScopeColumn}},
			DoUpdates: clause.AssignmentColumns([]string{
				"policy_hash",
				governorPolicySelectorUpdatedAtCol,
			}),
		}).Create(&selector).Error; createErr != nil {
			return fmt.Errorf("activate governor policy artifact: %w", createErr)
		}

		found, findErr := s.findArtifactModelByHash(ctx, tx, artifact.Hash)
		if findErr != nil {
			return findErr
		}

		persisted = found
		return nil
	})
	if err != nil {
		return Artifact{}, err
	}

	return governorPolicyArtifactModelToValue(persisted)
}

// Get reads a persisted governor policy artifact by canonical hash.
func (s *ArtifactDatabaseStore) Get(
	ctx context.Context,
	hash string,
) (*Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model, err := s.findArtifactModelByHash(ctx, s.db.WithContext(ctx), hash)
	if err != nil {
		return nil, err
	}

	artifact, err := governorPolicyArtifactModelToValue(model)
	if err != nil {
		return nil, err
	}

	return &artifact, nil
}

// GetActive reads the currently active paper governor policy artifact.
func (s *ArtifactDatabaseStore) GetActive(ctx context.Context) (*Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	selector, err := s.findActiveSelector(ctx, s.db.WithContext(ctx), governorPolicyActiveScopePaper)
	if err != nil {
		return nil, err
	}

	model, err := s.findArtifactModelByHash(ctx, s.db.WithContext(ctx), selector.PolicyHash)
	if err != nil {
		return nil, err
	}

	artifact, err := governorPolicyArtifactModelToValue(model)
	if err != nil {
		return nil, err
	}

	return &artifact, nil
}

func (s *ArtifactDatabaseStore) findArtifactModelByHash(
	ctx context.Context,
	db *gorm.DB,
	hash string,
) (governorPolicyArtifactModel, error) {
	var model governorPolicyArtifactModel
	if err := db.WithContext(ctx).
		First(&model, governorPolicyArtifactHashColumn+" = ?", hash).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return governorPolicyArtifactModel{}, fmt.Errorf("%w: %s", ErrArtifactNotFound, hash)
		}

		return governorPolicyArtifactModel{}, fmt.Errorf("get governor policy artifact: %w", err)
	}

	return model, nil
}

func (s *ArtifactDatabaseStore) findActiveSelector(
	ctx context.Context,
	db *gorm.DB,
	scope string,
) (governorPolicyActiveSelectorModel, error) {
	var selector governorPolicyActiveSelectorModel
	if err := db.WithContext(ctx).
		First(&selector, governorPolicyActiveScopeColumn+" = ?", scope).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return governorPolicyActiveSelectorModel{}, fmt.Errorf("%w: active %s policy", ErrArtifactNotFound, scope)
		}

		return governorPolicyActiveSelectorModel{}, fmt.Errorf("get active governor policy selector: %w", err)
	}

	return selector, nil
}

func governorPolicyArtifactToModel(artifact Artifact) governorPolicyArtifactModel {
	return governorPolicyArtifactModel{
		Hash:          artifact.Hash,
		SchemaVersion: artifact.SchemaVersion,
		ArtifactKind:  artifact.ArtifactKind,
		Mode:          artifact.Mode,
		CanonicalJSON: append([]byte(nil), artifact.CanonicalJSON...),
	}
}

func governorPolicyArtifactModelToValue(model governorPolicyArtifactModel) (Artifact, error) {
	var payload marshaledPolicyArtifactV0
	if err := json.Unmarshal(model.CanonicalJSON, &payload); err != nil {
		return Artifact{}, fmt.Errorf("decode governor policy artifact canonical payload: %w", err)
	}

	policyValue, err := governorPolicyArtifactPayloadToPolicy(payload)
	if err != nil {
		return Artifact{}, err
	}

	return Artifact{
		SchemaVersion: model.SchemaVersion,
		ArtifactKind:  model.ArtifactKind,
		Mode:          model.Mode,
		Policy:        policyValue,
		CanonicalJSON: append([]byte(nil), model.CanonicalJSON...),
		Hash:          model.Hash,
		CreatedAt:     model.CreatedAt,
	}, nil
}

func governorPolicyArtifactPayloadToPolicy(payload marshaledPolicyArtifactV0) (Policy, error) {
	mode := policyArtifactMode(payload.Mode)
	if mode != policyArtifactModePaper {
		return Policy{}, fmt.Errorf("decode governor policy artifact mode: unsupported mode %q", payload.Mode)
	}

	allowedActionKinds := make([]domain.CandidateActionKind, 0, len(payload.Policy.AllowedActionKinds))
	for _, actionKind := range payload.Policy.AllowedActionKinds {
		canonicalActionKind, err := domain.NewCandidateActionKind(actionKind)
		if err != nil {
			return Policy{}, fmt.Errorf("decode governor policy artifact action kind: %w", err)
		}

		allowedActionKinds = append(allowedActionKinds, canonicalActionKind)
	}

	minimumQuality, err := domain.NewDataQuality(payload.Policy.MinimumQuality)
	if err != nil {
		return Policy{}, fmt.Errorf("decode governor policy artifact minimum quality: %w", err)
	}

	canonical, err := canonicalizePolicy(Policy{
		AllowedActionKinds:   allowedActionKinds,
		MinimumQuality:       minimumQuality,
		MaximumApprovedCount: payload.Policy.MaximumApprovedCount,
	})
	if err != nil {
		return Policy{}, fmt.Errorf("decode governor policy artifact policy: %w", err)
	}

	return Policy{
		AllowedActionKinds:   sortedAllowedActionKinds(canonical.allowedActionKinds),
		MinimumQuality:       minimumQuality,
		MaximumApprovedCount: canonical.maximumApprovedCount,
	}, nil
}
