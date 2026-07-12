package strategy

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestStrategyVersionRegistryService(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) faker.Faker {
		t.Helper()

		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(t.Name()))

		return faker.NewWithSeedInt64(int64(hasher.Sum64()))
	}

	randomWord := func(t *testing.T, fake faker.Faker, prefix string) string {
		t.Helper()

		return prefix + "-" + strings.ToLower(fake.Lorem().Word()) + "-" + strconv.Itoa(fake.IntBetween(1000, 9999))
	}

	makeArtifactStore := func(t *testing.T, dsn string, tablePrefix string) *ArtifactDatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewArtifactDatabaseStore(sqlDB, dsn, ArtifactDatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeService := func(
		t *testing.T,
		dsn string,
		tablePrefix string,
		artifactStore strategyVersionArtifactStore,
	) *VersionRegistryService {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		service, err := NewVersionRegistryService(sqlDB, dsn, VersionRegistryServiceDeps{
			ArtifactStore: artifactStore,
			TablePrefix:   tablePrefix,
		})
		require.NoError(t, err)
		require.NoError(t, service.AutoMigrate())

		return service
	}

	makeRawPayload := func(
		t *testing.T,
		fake faker.Faker,
		index int,
	) []byte {
		t.Helper()

		fastWindow := fake.IntBetween(1, 20)
		slowWindow := fastWindow + fake.IntBetween(1, 20)

		return fmt.Appendf(nil, `{
			"kind": %q,
			"instrument": {
				"venue": %q,
				"symbol": %q,
				"assetClass": %q,
				"active": %t
			},
			"timeframe": %q,
			"parameters": {
				"fastWindow": %d,
				"slowWindow": %d
			}
		}`,
			domain.StrategyKindMovingAverageCrossover.String(),
			"  "+randomWord(t, fake, "venue")+"-"+strconv.Itoa(index)+"  ",
			"  "+strings.ToUpper(randomWord(t, fake, "symbol"))+strconv.Itoa(index)+"  ",
			"  CRYPTO  ",
			fake.Bool(),
			[]domain.Timeframe{domain.Timeframe1m, domain.Timeframe15m, domain.Timeframe4h}[index%3].String(),
			fastWindow,
			slowWindow,
		)
	}

	makeCreateParams := func(
		t *testing.T,
		fake faker.Faker,
		index int,
		raw []byte,
	) CreateVersionFromDSLV0Params {
		t.Helper()

		return CreateVersionFromDSLV0Params{
			StrategyID:       "  " + randomWord(t, fake, "strategy") + "-" + strconv.Itoa(index) + "  ",
			Version:          "  v" + strconv.Itoa(index) + "  ",
			DisplayName:      "  " + randomWord(t, fake, "display") + "  ",
			Status:           VersionStatusReady,
			SourceType:       VersionSourceTypeHuman,
			Notes:            "  " + randomWord(t, fake, "notes") + "  ",
			RawStrategyDSLV0: append([]byte(nil), raw...),
		}
	}

	readCount := func(t *testing.T, db any, tableName string) int64 {
		t.Helper()

		switch typed := db.(type) {
		case *ArtifactDatabaseStore:
			var count int64
			require.NoError(t, typed.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)
			return count
		case *VersionRegistryService:
			var count int64
			require.NoError(t, typed.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)
			return count
		default:
			t.Fatalf("unsupported db type %T", db)
			return 0
		}
	}

	hasUniqueIndexWithColumns := func(
		t *testing.T,
		service *VersionRegistryService,
		tableName string,
		want []string,
	) bool {
		t.Helper()

		var indexes []sqliteArtifactIndexListRow
		require.NoError(
			t,
			service.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error,
		)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteArtifactIndexInfoRow
			require.NoError(
				t,
				service.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error,
			)

			got := make([]string, 0, len(columns))
			for _, column := range columns {
				got = append(got, column.Name)
			}

			if slices.Equal(got, want) {
				return true
			}
		}

		return false
	}

	columnNames := func(rows []sqliteArtifactTableInfoRow) []string {
		result := make([]string, 0, len(rows))
		for _, row := range rows {
			result = append(result, row.Name)
		}

		return result
	}

	t.Run("NewStrategyVersionRegistryService", func(t *testing.T) {
		t.Parallel()

		t.Run("creates a sqlite backed service", func(t *testing.T) {
			t.Parallel()

			artifactStore := makeArtifactStore(t, ":memory:", "")
			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			service, err := NewVersionRegistryService(sqlDB, ":memory:", VersionRegistryServiceDeps{
				ArtifactStore: artifactStore,
			})
			require.NoError(t, err)
			require.NotNil(t, service)
		})

		t.Run("requires dsn and artifact store", func(t *testing.T) {
			t.Parallel()

			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			service, err := NewVersionRegistryService(sqlDB, "", VersionRegistryServiceDeps{})
			require.Error(t, err)
			require.Nil(t, service)

			service, err = NewVersionRegistryService(sqlDB, ":memory:", VersionRegistryServiceDeps{})
			require.EqualError(t, err, "artifact store is required")
			require.Nil(t, service)
		})

		t.Run("requires sql database", func(t *testing.T) {
			t.Parallel()

			service, err := NewVersionRegistryService(nil, ":memory:", VersionRegistryServiceDeps{})
			require.Error(t, err)
			require.Nil(t, service)
		})
	})

	t.Run("AutoMigrate", func(t *testing.T) {
		t.Parallel()

		artifactStore := makeArtifactStore(t, ":memory:", "")
		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()

		service, err := NewVersionRegistryService(sqlDB, ":memory:", VersionRegistryServiceDeps{
			ArtifactStore: artifactStore,
		})
		require.NoError(t, err)
		require.NoError(t, service.AutoMigrate())
		require.NoError(t, service.AutoMigrate())
	})

	t.Run("migration uses explicit table and column names with unique version identity index", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		artifactStore := makeArtifactStore(t, ":memory:", tablePrefix)
		service := makeService(t, ":memory:", tablePrefix, artifactStore)

		var columns []sqliteArtifactTableInfoRow
		require.NoError(
			t,
			service.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"strategy_versions"),
			).Scan(&columns).Error,
		)
		require.Equal(t, []string{
			"strategy_id",
			"strategy_version",
			"display_name",
			"status",
			"source_type",
			"artifact_hash",
			"artifact_schema_version",
			"strategy_kind",
			"instrument_venue",
			"instrument_symbol",
			"instrument_asset_class",
			"instrument_active",
			"timeframe",
			"fast_window",
			"slow_window",
			"notes",
			"parent_strategy_id",
			"parent_strategy_version",
			"created_at",
			"updated_at",
		}, columnNames(columns))
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				service,
				tablePrefix+"strategy_versions",
				[]string{"strategy_id", "strategy_version"},
			),
		)
		require.True(t, service.db.Migrator().HasIndex(&strategyVersionModel{}, "idx_strategy_versions_created_at"))
	})

	t.Run("CreateVersionFromDSLV0 GetVersion and ListVersions persist immutable strategy versions", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		artifactStore := makeArtifactStore(t, ":memory:", "")
		service := makeService(t, ":memory:", "", artifactStore)
		ctx := t.Context()

		firstCreated, err := service.CreateVersionFromDSLV0(
			ctx,
			makeCreateParams(t, fake, 1, makeRawPayload(t, fake, 1)),
		)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		secondCreated, err := service.CreateVersionFromDSLV0(
			ctx,
			makeCreateParams(t, fake, 2, makeRawPayload(t, fake, 2)),
		)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		thirdCreated, err := service.CreateVersionFromDSLV0(
			ctx,
			makeCreateParams(t, fake, 3, makeRawPayload(t, fake, 3)),
		)
		require.NoError(t, err)
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		oldest := earlier.Add(-time.Hour)
		require.True(t, oldest.Before(earlier))
		require.True(t, earlier.Before(later))
		for _, update := range []struct {
			version Version
			created time.Time
		}{
			{version: firstCreated, created: earlier},
			{version: secondCreated, created: later},
			{version: thirdCreated, created: oldest},
		} {
			require.NoError(t, service.db.Model(&strategyVersionModel{}).
				Where("strategy_id = ? AND strategy_version = ?", update.version.StrategyID, update.version.Version).
				UpdateColumn("created_at", update.created).Error)
		}
		firstCreated.CreatedAt = earlier
		secondCreated.CreatedAt = later
		thirdCreated.CreatedAt = oldest

		require.Equal(t, int64(3), readCount(t, service, "strategy_versions"))
		require.NotZero(t, firstCreated.CreatedAt)
		require.NotZero(t, firstCreated.UpdatedAt)

		fetched, err := service.GetVersion(ctx, "  "+firstCreated.StrategyID+"  ", "  "+firstCreated.Version+"  ")
		require.NoError(t, err)
		require.NotNil(t, fetched)
		require.Equal(t, firstCreated.StrategyID, fetched.StrategyID)
		require.Equal(t, firstCreated.Version, fetched.Version)
		require.Equal(t, earlier.Format(time.RFC3339Nano), fetched.CreatedAt.Format(time.RFC3339Nano))

		listed, err := service.ListVersions(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{secondCreated.Version, firstCreated.Version, thirdCreated.Version}, []string{
			listed[0].Version,
			listed[1].Version,
			listed[2].Version,
		})
		require.Equal(t, later.Format(time.RFC3339Nano), listed[0].CreatedAt.Format(time.RFC3339Nano))
		require.Equal(t, earlier.Format(time.RFC3339Nano), listed[1].CreatedAt.Format(time.RFC3339Nano))
	})

	t.Run("CreateVersionFromDSLV0 validates status and source type", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		artifactStore := makeArtifactStore(t, ":memory:", "")
		service := makeService(t, ":memory:", "", artifactStore)
		baseParams := makeCreateParams(t, fake, 1, makeRawPayload(t, fake, 1))

		for _, status := range []VersionStatus{VersionStatusReady, VersionStatusArchived} {
			params := baseParams
			params.Status = status

			created, err := service.CreateVersionFromDSLV0(t.Context(), params)
			require.NoError(t, err)
			require.Equal(t, status, created.Status)
			baseParams.Version += "-next"
		}

		params := baseParams
		params.Status = VersionStatus("draft")
		created, err := service.CreateVersionFromDSLV0(t.Context(), params)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "invalid strategy version status")
		require.Equal(t, Version{}, created)

		for _, sourceType := range []VersionSourceType{
			VersionSourceTypeHuman,
			VersionSourceTypeDemo,
			VersionSourceTypeAIDraft,
		} {
			params = baseParams
			params.SourceType = sourceType
			params.Version += "-source-" + string(sourceType)
			created, err = service.CreateVersionFromDSLV0(t.Context(), params)
			require.NoError(t, err)
			require.Equal(t, sourceType, created.SourceType)
		}

		params = baseParams
		params.SourceType = VersionSourceType("machine")
		params.Version += "-invalid-source"
		created, err = service.CreateVersionFromDSLV0(t.Context(), params)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "invalid strategy version source type")
		require.Equal(t, Version{}, created)
	})

	t.Run(
		"CreateVersionFromDSLV0 reuses canonical artifacts and links hash and summary from artifact",
		func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			artifactStore := makeArtifactStore(t, ":memory:", "")
			service := makeService(t, ":memory:", "", artifactStore)

			firstRaw := makeRawPayload(t, fake, 1)
			firstParams := makeCreateParams(t, fake, 1, firstRaw)
			firstCreated, err := service.CreateVersionFromDSLV0(t.Context(), firstParams)
			require.NoError(t, err)

			artifact, err := artifactStore.Get(t.Context(), firstCreated.ArtifactHash)
			require.NoError(t, err)
			require.NotNil(t, artifact)
			require.Equal(t, artifact.Hash, firstCreated.ArtifactHash)
			require.Equal(t, artifact.SchemaVersion, firstCreated.ArtifactSchemaVersion)
			require.Equal(t, artifact.Strategy, firstCreated.Strategy)

			secondParams := makeCreateParams(t, fake, 2, fmt.Appendf(
				nil,
				`{ "parameters": { "slowWindow": %d, "fastWindow": %d }, "timeframe": %q, "instrument": { "active": %t, "assetClass": %q, "symbol": %q, "venue": %q }, "kind": %q }`,
				firstCreated.Strategy.Parameters.SlowWindow,
				firstCreated.Strategy.Parameters.FastWindow,
				"  "+firstCreated.Strategy.Timeframe.String()+"  ",
				firstCreated.Strategy.Instrument.Active,
				"  "+firstCreated.Strategy.Instrument.AssetClass.String()+"  ",
				"  "+firstCreated.Strategy.Instrument.Symbol.String()+"  ",
				"  "+firstCreated.Strategy.Instrument.Venue.String()+"  ",
				"  "+firstCreated.Strategy.Kind.String()+"  ",
			))
			secondCreated, err := service.CreateVersionFromDSLV0(t.Context(), secondParams)
			require.NoError(t, err)
			require.Equal(t, firstCreated.ArtifactHash, secondCreated.ArtifactHash)
			require.Equal(t, int64(1), readCount(t, artifactStore, "strategy_artifacts"))
			require.Equal(t, int64(2), readCount(t, service, "strategy_versions"))
		},
	)

	t.Run("CreateVersionFromDSLV0 rejects unsupported and code-like fields from strict DSL path", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		artifactStore := makeArtifactStore(t, ":memory:", "")
		service := makeService(t, ":memory:", "", artifactStore)

		params := makeCreateParams(t, fake, 1, []byte(`{
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
				"slowWindow": 21,
				"code": "alert('nope')"
			}
		}`))

		created, err := service.CreateVersionFromDSLV0(t.Context(), params)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "unknown field \"code\"")
		require.Equal(t, Version{}, created)
		require.Equal(t, int64(0), readCount(t, artifactStore, "strategy_artifacts"))
		require.Equal(t, int64(0), readCount(t, service, "strategy_versions"))
	})

	t.Run("CreateVersionFromDSLV0 stores parent linkage and preserves immutability", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		artifactStore := makeArtifactStore(t, ":memory:", "")
		service := makeService(t, ":memory:", "", artifactStore)

		parentParams := makeCreateParams(t, fake, 1, makeRawPayload(t, fake, 1))
		parent, err := service.CreateVersionFromDSLV0(t.Context(), parentParams)
		require.NoError(t, err)

		childParams := makeCreateParams(t, fake, 2, makeRawPayload(t, fake, 2))
		childParams.ParentStrategyID = parent.StrategyID
		childParams.ParentVersion = parent.Version
		child, err := service.CreateVersionFromDSLV0(t.Context(), childParams)
		require.NoError(t, err)
		require.Equal(t, parent.StrategyID, child.ParentStrategyID)
		require.Equal(t, parent.Version, child.ParentVersion)

		missingParentParams := makeCreateParams(t, fake, 3, makeRawPayload(t, fake, 3))
		missingParentParams.ParentStrategyID = randomWord(t, fake, "missing-parent")
		missingParentParams.ParentVersion = "v999"
		created, err := service.CreateVersionFromDSLV0(t.Context(), missingParentParams)
		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "does not exist")
		require.Equal(t, Version{}, created)

		conflictParams := parentParams
		conflictParams.DisplayName = parent.DisplayName + " changed"
		conflictCreated, err := service.CreateVersionFromDSLV0(t.Context(), conflictParams)
		require.ErrorIs(t, err, ErrStrategyVersionImmutableConflict)
		require.Equal(t, Version{}, conflictCreated)

		repeatedParent, err := service.CreateVersionFromDSLV0(t.Context(), parentParams)
		require.NoError(t, err)
		require.Equal(t, parent, repeatedParent)

		fetchedParent, err := service.GetVersion(t.Context(), parent.StrategyID, parent.Version)
		require.NoError(t, err)
		require.Equal(t, parent, *fetchedParent)
	})

	t.Run("EnsureDemoStrategyVersions is idempotent and duplicate returns a human candidate", func(t *testing.T) {
		t.Parallel()

		artifactStore := makeArtifactStore(t, ":memory:", "")
		service := makeService(t, ":memory:", "", artifactStore)

		for _, demo := range makeStrategyDemoVersionDefinitions() {
			artifact, err := NewArtifactFromDSLV0(demo.RawStrategyDSLV0)
			require.NoError(t, err)
			require.Equal(t, ArtifactKind, artifact.ArtifactKind)
		}

		demoDefinitions := makeStrategyDemoVersionDefinitions()
		firstSeed, err := service.EnsureDemoStrategyVersions(t.Context())
		require.NoError(t, err)
		require.Len(t, firstSeed, len(demoDefinitions))
		require.GreaterOrEqual(t, len(firstSeed), 3)
		require.Equal(t, int64(len(demoDefinitions)), readCount(t, service, "strategy_versions"))

		secondSeed, err := service.EnsureDemoStrategyVersions(t.Context())
		require.NoError(t, err)
		require.Equal(t, firstSeed, secondSeed)
		require.Equal(t, int64(len(demoDefinitions)), readCount(t, service, "strategy_versions"))
		require.Equal(t, int64(len(demoDefinitions)), readCount(t, artifactStore, "strategy_artifacts"))

		for _, demoVersion := range firstSeed {
			require.Equal(t, VersionStatusReady, demoVersion.Status)
			require.Equal(t, VersionSourceTypeDemo, demoVersion.SourceType)
			require.Contains(t, strings.ToLower(demoVersion.Notes), "example")
			require.Contains(t, strings.ToLower(demoVersion.Notes), "not a recommendation")
			require.Contains(t, strings.ToLower(demoVersion.Notes), "local historical data")
		}

		candidate, err := service.DuplicateVersionAsHumanCandidate(
			t.Context(),
			firstSeed[0].StrategyID,
			firstSeed[0].Version,
		)
		require.NoError(t, err)
		require.Equal(t, firstSeed[0].StrategyID, candidate.StrategyID)
		require.Empty(t, candidate.Version)
		require.Equal(t, VersionCandidateStatusDraft, candidate.Status)
		require.Equal(t, VersionSourceTypeHuman, candidate.SourceType)
		require.Equal(t, firstSeed[0].Strategy, candidate.Strategy)
		require.Equal(t, firstSeed[0].StrategyID, candidate.ParentStrategyID)
		require.Equal(t, firstSeed[0].Version, candidate.ParentVersion)
		require.Equal(t, int64(len(demoDefinitions)), readCount(t, service, "strategy_versions"))

		persistedAfterDuplicate, err := service.GetVersion(
			t.Context(),
			firstSeed[0].StrategyID,
			firstSeed[0].Version,
		)
		require.NoError(t, err)
		require.Equal(t, firstSeed[0], *persistedAfterDuplicate)
	})
}
