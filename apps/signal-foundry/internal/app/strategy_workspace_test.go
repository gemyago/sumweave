package app

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	rtstrategy "github.com/gemyago/signal-foundry/runtime/strategy"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type strategyArtifactLookupStub struct {
	getFunc func(context.Context, string) (*rtstrategy.Artifact, error)
}

func (s *strategyArtifactLookupStub) Get(
	ctx context.Context,
	hash string,
) (*rtstrategy.Artifact, error) {
	if s.getFunc == nil {
		return nil, rtstrategy.ErrArtifactNotFound
	}

	return s.getFunc(ctx, hash)
}

type strategyVersionRegistryStub struct {
	createFunc      func(context.Context, rtstrategy.CreateVersionFromDSLV0Params) (rtstrategy.Version, error)
	getFunc         func(context.Context, string, string) (*rtstrategy.Version, error)
	listFunc        func(context.Context) ([]rtstrategy.Version, error)
	duplicateFunc   func(context.Context, string, string) (rtstrategy.VersionCandidate, error)
	ensureDemosFunc func(context.Context) ([]rtstrategy.Version, error)
}

func (s *strategyVersionRegistryStub) CreateVersionFromDSLV0(
	ctx context.Context,
	params rtstrategy.CreateVersionFromDSLV0Params,
) (rtstrategy.Version, error) {
	if s.createFunc == nil {
		return rtstrategy.Version{}, errors.New("unexpected CreateVersionFromDSLV0 call")
	}

	return s.createFunc(ctx, params)
}

func (s *strategyVersionRegistryStub) GetVersion(
	ctx context.Context,
	strategyID string,
	version string,
) (*rtstrategy.Version, error) {
	if s.getFunc == nil {
		return nil, errors.New("unexpected GetVersion call")
	}

	return s.getFunc(ctx, strategyID, version)
}

func (s *strategyVersionRegistryStub) ListVersions(
	ctx context.Context,
) ([]rtstrategy.Version, error) {
	if s.listFunc == nil {
		return nil, errors.New("unexpected ListVersions call")
	}

	return s.listFunc(ctx)
}

func (s *strategyVersionRegistryStub) DuplicateVersionAsHumanCandidate(
	ctx context.Context,
	strategyID string,
	version string,
) (rtstrategy.VersionCandidate, error) {
	if s.duplicateFunc == nil {
		return rtstrategy.VersionCandidate{}, errors.New("unexpected duplicate call")
	}

	return s.duplicateFunc(ctx, strategyID, version)
}

func (s *strategyVersionRegistryStub) EnsureDemoStrategyVersions(
	ctx context.Context,
) ([]rtstrategy.Version, error) {
	if s.ensureDemosFunc == nil {
		return []rtstrategy.Version{}, nil
	}

	return s.ensureDemosFunc(ctx)
}

func TestStrategyWorkspaceService(t *testing.T) {
	t.Parallel()

	makeService := func(t *testing.T) (*StrategyWorkspaceService, *rtstrategy.ArtifactDatabaseStore, *rtstrategy.VersionRegistryService) {
		t.Helper()

		dsn := filepath.Join(t.TempDir(), "strategy-workspace.db")
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		artifactStore, err := rtstrategy.NewArtifactDatabaseStore(
			sqlDB,
			dsn,
			rtstrategy.ArtifactDatabaseStoreOpts{TablePrefix: "app_"},
		)
		require.NoError(t, err)
		require.NoError(t, artifactStore.AutoMigrate())

		registry, err := rtstrategy.NewVersionRegistryService(
			sqlDB,
			dsn,
			rtstrategy.VersionRegistryServiceDeps{
				ArtifactStore: artifactStore,
				TablePrefix:   "app_",
			},
		)
		require.NoError(t, err)
		require.NoError(t, registry.AutoMigrate())

		service, err := NewStrategyWorkspaceService(StrategyWorkspaceServiceDeps{
			ArtifactStore:   artifactStore,
			VersionRegistry: registry,
		})
		require.NoError(t, err)

		return service, artifactStore, registry
	}

	makeDefinition := func(t *testing.T, fake faker.Faker, suffix string) StrategyDefinitionInput {
		t.Helper()

		fastWindow := fake.IntBetween(1, 20)
		slowWindow := fastWindow + fake.IntBetween(1, 20)
		return StrategyDefinitionInput{
			Kind: "  moving-average-crossover  ",
			Instrument: StrategyInstrumentInput{
				Venue:      "  venue-" + suffix + "  ",
				Symbol:     "  SYMBOL-" + suffix + "  ",
				AssetClass: "  CRYPTO  ",
				Active:     true,
			},
			Timeframe:  "  1h  ",
			Parameters: StrategyParameterSummary{FastWindow: fastWindow, SlowWindow: slowWindow},
		}
	}

	makeCreateParams := func(t *testing.T, fake faker.Faker, index int, definition StrategyDefinitionInput) CreateStrategyVersionParams {
		t.Helper()

		return CreateStrategyVersionParams{
			StrategyID: "  strategy-" + strings.ToLower(
				fake.Lorem().Word(),
			) + "-" + strconv.Itoa(
				index,
			) + "  ",
			Version:     "  v" + strconv.Itoa(index) + "  ",
			DisplayName: "  display-" + fake.Lorem().Word() + "  ",
			Notes:       "  notes-" + fake.Lorem().Word() + "  ",
			Definition:  definition,
		}
	}

	t.Run(
		"ValidateDefinition returns deterministic errors and preview without saving",
		func(t *testing.T) {
			t.Parallel()

			fake := faker.New()
			service, artifactStore, registry := makeService(t)

			result, err := service.ValidateDefinition(t.Context(), StrategyDefinitionInput{
				Kind: "bad-kind",
				Instrument: StrategyInstrumentInput{
					Venue:      "  ",
					Symbol:     "  ",
					AssetClass: "  bad-asset  ",
				},
				Timeframe:  "  bad-timeframe  ",
				Parameters: StrategyParameterSummary{FastWindow: 0, SlowWindow: 0},
			})
			require.NoError(t, err)
			require.False(t, result.Valid)
			require.Nil(t, result.Preview)
			require.Equal(t, []StrategyFieldError{
				{Path: "definition.kind", Message: `invalid strategy kind "bad-kind"`},
				{Path: "definition.instrument.venue", Message: "venue is required"},
				{Path: "definition.instrument.symbol", Message: "symbol is required"},
				{
					Path:    "definition.instrument.assetClass",
					Message: `invalid asset class "  bad-asset  "`,
				},
				{Path: "definition.timeframe", Message: `invalid timeframe "  bad-timeframe  "`},
				{
					Path:    "definition.parameters.fastWindow",
					Message: "moving average crossover fast window must be positive",
				},
				{
					Path:    "definition.parameters.slowWindow",
					Message: "moving average crossover slow window must be positive",
				},
			}, result.Errors)

			artifacts, err := artifactStore.List(t.Context())
			require.NoError(t, err)
			require.Empty(t, artifacts)

			versions, err := registry.ListVersions(t.Context())
			require.NoError(t, err)
			require.Empty(t, versions)

			definition := makeDefinition(t, fake, "preview")
			previewResult, err := service.ValidateDefinition(t.Context(), definition)
			require.NoError(t, err)
			require.True(t, previewResult.Valid)
			require.NotNil(t, previewResult.Preview)
			require.Equal(t, rtstrategy.ArtifactSchemaVersion, previewResult.Preview.SchemaVersion)
			require.Equal(t, "moving-average-crossover", previewResult.Preview.Kind)
			require.Equal(
				t,
				strings.TrimSpace(definition.Instrument.Venue),
				previewResult.Preview.Instrument.Venue,
			)
			require.Equal(
				t,
				strings.ToLower(strings.TrimSpace(definition.Instrument.AssetClass)),
				previewResult.Preview.Instrument.AssetClass,
			)
			require.False(t, previewResult.Preview.ExistingArtifact)

			created, err := service.CreateVersion(
				t.Context(),
				makeCreateParams(t, fake, 1, definition),
			)
			require.NoError(t, err)
			require.NotEmpty(t, created.ArtifactHash)

			repeatPreview, err := service.ValidateDefinition(t.Context(), definition)
			require.NoError(t, err)
			require.True(t, repeatPreview.Valid)
			require.NotNil(t, repeatPreview.Preview)
			require.True(t, repeatPreview.Preview.ExistingArtifact)
			require.Equal(t, created.ArtifactHash, repeatPreview.Preview.ArtifactHash)
		},
	)

	t.Run(
		"CreateVersion saves immutable versions and edit creates a new version",
		func(t *testing.T) {
			t.Parallel()

			fake := faker.New()
			service, artifactStore, registry := makeService(t)

			baseDefinition := makeDefinition(t, fake, "base")
			baseParams := makeCreateParams(t, fake, 1, baseDefinition)
			createdBase, err := service.CreateVersion(t.Context(), baseParams)
			require.NoError(t, err)
			require.Equal(t, "ready", createdBase.Status)
			require.Equal(t, "human", createdBase.SourceType)

			updatedDefinition := baseDefinition
			updatedDefinition.Parameters.FastWindow = baseDefinition.Parameters.FastWindow + 1
			updatedDefinition.Parameters.SlowWindow = baseDefinition.Parameters.SlowWindow + 2
			updatedParams := makeCreateParams(t, fake, 2, updatedDefinition)
			updatedParams.StrategyID = createdBase.StrategyID
			updatedParams.ParentStrategyID = createdBase.StrategyID
			updatedParams.ParentVersion = createdBase.Version

			createdUpdated, err := service.CreateVersion(t.Context(), updatedParams)
			require.NoError(t, err)
			require.Equal(t, createdBase.StrategyID, createdUpdated.ParentStrategyID)
			require.Equal(t, createdBase.Version, createdUpdated.ParentVersion)
			require.NotEqual(t, createdBase.ArtifactHash, createdUpdated.ArtifactHash)

			fetchedBase, err := service.GetVersion(
				t.Context(),
				createdBase.StrategyID,
				createdBase.Version,
			)
			require.NoError(t, err)
			require.Equal(t, createdBase.ArtifactHash, fetchedBase.ArtifactHash)
			require.Equal(
				t,
				baseDefinition.Parameters.FastWindow,
				fetchedBase.ParameterSummary.FastWindow,
			)

			versions, err := registry.ListVersions(t.Context())
			require.NoError(t, err)
			require.Len(t, versions, 5)

			artifacts, err := artifactStore.List(t.Context())
			require.NoError(t, err)
			require.Len(t, artifacts, 5)
		},
	)

	t.Run(
		"ListVersions includes seeded demos and DuplicateVersion returns a draftable human candidate",
		func(t *testing.T) {
			t.Parallel()

			service, _, registry := makeService(t)

			listed, err := service.ListVersions(t.Context())
			require.NoError(t, err)
			require.Len(t, listed, 3)
			for _, item := range listed {
				require.Equal(t, "ready", item.Status)
				require.Equal(t, "demo", item.SourceType)
				require.Equal(t, "Demo example", item.SourceLabel)
			}

			candidate, err := service.DuplicateVersion(
				t.Context(),
				listed[0].StrategyID,
				listed[0].Version,
			)
			require.NoError(t, err)
			require.Equal(t, listed[0].StrategyID, candidate.StrategyID)
			require.Empty(t, candidate.Version)
			require.Equal(t, "draft", candidate.Status)
			require.Equal(t, "human", candidate.SourceType)
			require.Equal(t, listed[0].StrategyID, candidate.ParentStrategyID)
			require.Equal(t, listed[0].Version, candidate.ParentVersion)

			versionsAfter, err := registry.ListVersions(t.Context())
			require.NoError(t, err)
			require.Len(t, versionsAfter, 3)
		},
	)

	t.Run("CreateVersion returns invalid input for missing identity metadata", func(t *testing.T) {
		t.Parallel()

		fake := faker.New()
		service, _, _ := makeService(t)

		_, err := service.CreateVersion(t.Context(), CreateStrategyVersionParams{
			Definition: makeDefinition(t, fake, "invalid"),
		})
		require.EqualError(
			t,
			err,
			"invalid input for field 'request': strategy validation failed: strategyId: strategy id is required",
		)
	})

	t.Run("constructor and error branches stay deterministic", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, "strategy validation failed", (&strategyValidationError{}).Error())

		_, err := NewStrategyWorkspaceService(StrategyWorkspaceServiceDeps{})
		require.EqualError(t, err, "strategy artifact store is required")

		_, err = NewStrategyWorkspaceService(StrategyWorkspaceServiceDeps{
			ArtifactStore: &rtstrategy.ArtifactDatabaseStore{},
		})
		require.EqualError(t, err, "strategy version registry is required")

		constructed, err := NewStrategyWorkspaceService(StrategyWorkspaceServiceDeps{
			ArtifactStore:   &rtstrategy.ArtifactDatabaseStore{},
			VersionRegistry: &rtstrategy.VersionRegistryService{},
		})
		require.NoError(t, err)
		require.NotNil(t, constructed)

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = constructed.ValidateDefinition(canceledCtx, makeStrategyDefinitionForErrors())
		require.ErrorIs(t, err, context.Canceled)

		_, artifactMessage := buildArtifactFromPayload([]byte(`{"kind":"bad"}`))
		require.NotEmpty(t, artifactMessage)

		service := &StrategyWorkspaceService{
			artifactStore: &strategyArtifactLookupStub{
				getFunc: func(context.Context, string) (*rtstrategy.Artifact, error) {
					return nil, errors.New("artifact lookup failed")
				},
			},
			versionRegistry: &strategyVersionRegistryStub{},
		}
		_, err = service.ValidateDefinition(t.Context(), makeStrategyDefinitionForErrors())
		require.EqualError(t, err, "read strategy artifact by hash: artifact lookup failed")

		service = &StrategyWorkspaceService{
			artifactStore:   &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{},
		}
		_, err = service.CreateVersion(canceledCtx, CreateStrategyVersionParams{})
		require.ErrorIs(t, err, context.Canceled)

		service = &StrategyWorkspaceService{
			artifactStore: &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{
				ensureDemosFunc: func(context.Context) ([]rtstrategy.Version, error) {
					return nil, errors.New("seed failed")
				},
			},
		}
		_, err = service.ListVersions(t.Context())
		require.EqualError(t, err, "ensure demo strategy versions: seed failed")
		_, err = service.GetVersion(t.Context(), "strategy-a", "v1")
		require.EqualError(t, err, "ensure demo strategy versions: seed failed")
		_, err = service.DuplicateVersion(t.Context(), "strategy-a", "v1")
		require.EqualError(t, err, "ensure demo strategy versions: seed failed")
		_, err = service.CreateVersion(t.Context(), CreateStrategyVersionParams{
			StrategyID:  "strategy-a",
			Version:     "v1",
			DisplayName: "Example",
			Definition:  makeStrategyDefinitionForErrors(),
		})
		require.EqualError(t, err, "ensure demo strategy versions: seed failed")

		service = &StrategyWorkspaceService{
			artifactStore: &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{
				listFunc: func(context.Context) ([]rtstrategy.Version, error) {
					return nil, errors.New("list failed")
				},
			},
		}
		_, err = service.ListVersions(t.Context())
		require.EqualError(t, err, "list strategy versions: list failed")

		service = &StrategyWorkspaceService{
			artifactStore:   &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{},
		}
		_, err = service.CreateVersion(t.Context(), CreateStrategyVersionParams{
			StrategyID:  "strategy-a",
			Version:     "v1",
			DisplayName: "Example",
			Definition: StrategyDefinitionInput{
				Kind: "bad-kind",
			},
		})
		require.EqualError(
			t,
			err,
			"invalid input for field 'definition': strategy validation failed: definition.kind: invalid strategy kind \"bad-kind\"",
		)

		service = &StrategyWorkspaceService{
			artifactStore: &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{
				createFunc: func(context.Context, rtstrategy.CreateVersionFromDSLV0Params) (rtstrategy.Version, error) {
					return rtstrategy.Version{}, rtstrategy.ErrStrategyVersionImmutableConflict
				},
			},
		}
		_, err = service.CreateVersion(t.Context(), CreateStrategyVersionParams{
			StrategyID:  "strategy-a",
			Version:     "v1",
			DisplayName: "Example",
			Definition:  makeStrategyDefinitionForErrors(),
		})
		require.EqualError(
			t,
			err,
			"conflict with strategy version: strategy version already exists with different immutable content",
		)

		service = &StrategyWorkspaceService{
			artifactStore: &strategyArtifactLookupStub{},
			versionRegistry: &strategyVersionRegistryStub{
				getFunc: func(context.Context, string, string) (*rtstrategy.Version, error) {
					return nil, rtstrategy.ErrStrategyVersionNotFound
				},
				duplicateFunc: func(context.Context, string, string) (rtstrategy.VersionCandidate, error) {
					return rtstrategy.VersionCandidate{}, rtstrategy.ErrStrategyVersionNotFound
				},
			},
		}
		_, err = service.GetVersion(t.Context(), "strategy-a", "v1")
		require.EqualError(t, err, "strategy version not found: strategy-a/v1")
		_, err = service.DuplicateVersion(t.Context(), "strategy-a", "v1")
		require.EqualError(t, err, "strategy version not found: strategy-a/v1")

		problem := validateCreateStrategyVersionParams(
			CreateStrategyVersionParams{ParentStrategyID: "strategy-a"},
		)
		require.Contains(
			t,
			problem.Errors,
			StrategyFieldError{
				Path:    "parentVersion",
				Message: "parent version is required when parent strategy id is set",
			},
		)

		fieldErrors := validateDefinition(StrategyDefinitionInput{
			Kind:      "moving-average-crossover",
			Timeframe: "1h",
			Instrument: StrategyInstrumentInput{
				Venue:      "binance",
				Symbol:     "BTCUSDT",
				AssetClass: "crypto",
			},
			Parameters: StrategyParameterSummary{FastWindow: 21, SlowWindow: 21},
		})
		require.Equal(
			t,
			[]StrategyFieldError{{
				Path:    strategyFieldPathFastWindow,
				Message: "moving average crossover fast window must be less than slow window",
			}},
			fieldErrors,
		)

		require.EqualError(
			t,
			mapStrategyPersistenceError("strategy-a", "v1", rtstrategy.ErrValidation),
			"invalid input for field 'definition': strategy validation failed",
		)
		fallbackErr := errors.New("fallback")
		require.ErrorIs(
			t,
			mapStrategyPersistenceError("strategy-a", "v1", fallbackErr),
			fallbackErr,
		)

		require.Equal(
			t,
			strategySourceLabelAIDraft,
			sourceLabel(rtstrategy.VersionSourceTypeAIDraft),
		)
		require.Equal(
			t,
			strategySourceLabelHuman,
			sourceLabel(rtstrategy.VersionSourceType("weird")),
		)
	})
}

func makeStrategyDefinitionForErrors() StrategyDefinitionInput {
	return StrategyDefinitionInput{
		Kind: "moving-average-crossover",
		Instrument: StrategyInstrumentInput{
			Venue:      "binance",
			Symbol:     "BTCUSDT",
			AssetClass: "crypto",
			Active:     true,
		},
		Timeframe:  "1h",
		Parameters: StrategyParameterSummary{FastWindow: 9, SlowWindow: 21},
	}
}
