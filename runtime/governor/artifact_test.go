package governor

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

type sqliteArtifactIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteArtifactIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteArtifactTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestArtifactDatabaseStore(t *testing.T) {
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

	marshalAllowedActionKinds := func(actionKinds []string) string {
		quoted := make([]string, 0, len(actionKinds))
		for _, actionKind := range actionKinds {
			quoted = append(quoted, strconv.Quote(actionKind))
		}

		return "[" + strings.Join(quoted, ",") + "]"
	}

	makeRawPayload := func(
		t *testing.T,
		fake faker.Faker,
		index int,
	) []byte {
		t.Helper()

		minimumQuality := []domain.DataQuality{
			domain.DataQualityRaw,
			domain.DataQualityValidated,
		}[index%2]
		maximumApprovedCount := fake.IntBetween(0, 5) + index

		return fmt.Appendf(
			nil,
			`{"mode":%q,"allowedActionKinds":%s,"minimumQuality":%q,"maximumApprovedCount":%d}`,
			"  PAPER  ",
			marshalAllowedActionKinds([]string{" SHORT ", " long "}),
			"  "+minimumQuality.String()+"  ",
			maximumApprovedCount,
		)
	}

	makeEquivalentPayload := func(artifact Artifact) []byte {
		return fmt.Appendf(
			nil,
			`{"maximumApprovedCount":%d,"minimumQuality":%q,"allowedActionKinds":%s,"mode":%q}`,
			artifact.Policy.MaximumApprovedCount,
			"  "+artifact.Policy.MinimumQuality.String()+"  ",
			marshalAllowedActionKinds([]string{
				"  " + domain.CandidateActionKindShort.String() + "  ",
				"  " + domain.CandidateActionKindLong.String() + "  ",
			}),
			" paper ",
		)
	}

	makeStore := func(t *testing.T, dsn string, tablePrefix string) *ArtifactDatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewArtifactDatabaseStore(sqlDB, dsn, ArtifactDatabaseStoreOpts{
			TablePrefix: tablePrefix,
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	readCount := func(t *testing.T, store *ArtifactDatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(
		t *testing.T,
		store *ArtifactDatabaseStore,
		tableName string,
		want []string,
	) bool {
		t.Helper()

		var indexes []sqliteArtifactIndexListRow
		require.NoError(
			t,
			store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error,
		)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteArtifactIndexInfoRow
			require.NoError(
				t,
				store.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error,
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

	t.Run("NewArtifactDatabaseStore", func(t *testing.T) {
		t.Parallel()

		t.Run("creates a sqlite backed store", func(t *testing.T) {
			t.Parallel()

			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			store, err := NewArtifactDatabaseStore(sqlDB, ":memory:", ArtifactDatabaseStoreOpts{})
			require.NoError(t, err)
			require.NotNil(t, store)
		})

		t.Run("requires a dsn", func(t *testing.T) {
			t.Parallel()

			sqlDB, err := sqlconn.Open(":memory:")
			require.NoError(t, err)
			defer func() { require.NoError(t, sqlDB.Close()) }()

			store, err := NewArtifactDatabaseStore(sqlDB, "", ArtifactDatabaseStoreOpts{})
			require.Error(t, err)
			require.Nil(t, store)
		})

		t.Run("requires a sql database", func(t *testing.T) {
			t.Parallel()

			store, err := NewArtifactDatabaseStore(nil, ":memory:", ArtifactDatabaseStoreOpts{})
			require.Error(t, err)
			require.Nil(t, store)
		})
	})

	t.Run("AutoMigrate", func(t *testing.T) {
		t.Parallel()

		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		defer func() { require.NoError(t, sqlDB.Close()) }()

		store, err := NewArtifactDatabaseStore(sqlDB, ":memory:", ArtifactDatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		require.NoError(t, store.AutoMigrate())
	})

	t.Run("migration uses explicit tables and columns with unique hash index", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var artifactColumns []sqliteArtifactTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"governor_policy_artifacts"),
			).Scan(&artifactColumns).Error,
		)
		require.Equal(t, []string{
			"hash",
			"schema_version",
			"artifact_kind",
			"mode",
			"canonical_json",
			"created_at",
		}, columnNames(artifactColumns))
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"governor_policy_artifacts",
				[]string{"hash"},
			),
		)

		var selectorColumns []sqliteArtifactTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"governor_policy_active_selectors"),
			).Scan(&selectorColumns).Error,
		)
		require.Equal(t, []string{"scope", "policy_hash", "updated_at"}, columnNames(selectorColumns))
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"governor_policy_active_selectors",
				[]string{"scope"},
			),
		)
	})

	t.Run("Create and Get persist immutable artifacts", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		ctx := t.Context()

		created, err := store.Create(ctx, makeRawPayload(t, fake, 1))
		require.NoError(t, err)
		require.NotZero(t, created.CreatedAt)
		require.NotZero(t, created.CreatedAt)
		require.Equal(t, int64(1), readCount(t, store, "governor_policy_artifacts"))
		require.Equal(t, int64(0), readCount(t, store, "governor_policy_active_selectors"))

		fetched, err := store.Get(ctx, created.Hash)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		require.Equal(t, created, *fetched)

		duplicateCreated, err := store.Create(ctx, makeEquivalentPayload(created))
		require.NoError(t, err)
		require.Equal(t, created, duplicateCreated)
		require.Equal(t, int64(1), readCount(t, store, "governor_policy_artifacts"))

		missing, err := store.Get(ctx, randomWord(t, fake, "missing-hash"))
		require.ErrorIs(t, err, ErrArtifactNotFound)
		require.Nil(t, missing)
	})

	t.Run("GetActive returns not found without a paper activation", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")

		artifact, err := store.Create(t.Context(), makeRawPayload(t, fake, 1))
		require.NoError(t, err)

		require.NoError(
			t,
			store.db.Exec(
				"INSERT INTO governor_policy_active_selectors (scope, policy_hash, updated_at) VALUES (?, ?, ?)",
				"live",
				artifact.Hash,
				artifact.CreatedAt.Add(time.Minute),
			).Error,
		)

		active, err := store.GetActive(t.Context())
		require.ErrorIs(t, err, ErrArtifactNotFound)
		require.Nil(t, active)
	})

	t.Run("CreateWithActivate persists or reuses the artifact and activates paper scope", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		ctx := t.Context()

		created, err := store.CreateWithActivate(ctx, makeRawPayload(t, fake, 2))
		require.NoError(t, err)
		require.Equal(t, int64(1), readCount(t, store, "governor_policy_artifacts"))
		require.Equal(t, int64(1), readCount(t, store, "governor_policy_active_selectors"))

		active, err := store.GetActive(ctx)
		require.NoError(t, err)
		require.NotNil(t, active)
		require.Equal(t, created, *active)
	})

	t.Run("duplicate CreateWithActivate is idempotent and only changes the selector", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		ctx := t.Context()

		firstCreated, err := store.Create(ctx, makeRawPayload(t, fake, 1))
		require.NoError(t, err)

		secondCreated, err := store.CreateWithActivate(ctx, makeRawPayload(t, fake, 2))
		require.NoError(t, err)

		duplicateActivated, err := store.CreateWithActivate(ctx, makeEquivalentPayload(firstCreated))
		require.NoError(t, err)
		require.Equal(t, firstCreated, duplicateActivated)
		require.NotEqual(t, firstCreated.Hash, secondCreated.Hash)
		require.Equal(t, int64(2), readCount(t, store, "governor_policy_artifacts"))
		require.Equal(t, int64(1), readCount(t, store, "governor_policy_active_selectors"))

		fetchedFirst, err := store.Get(ctx, firstCreated.Hash)
		require.NoError(t, err)
		require.Equal(t, firstCreated, *fetchedFirst)

		active, err := store.GetActive(ctx)
		require.NoError(t, err)
		require.Equal(t, firstCreated, *active)
	})

	t.Run("schema enforces unique hashes", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		artifact, err := store.Create(t.Context(), makeRawPayload(t, fake, 3))
		require.NoError(t, err)

		err = store.db.Exec(
			"INSERT INTO governor_policy_artifacts (hash, schema_version, artifact_kind, mode, canonical_json, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			artifact.Hash,
			artifact.SchemaVersion,
			artifact.ArtifactKind,
			artifact.Mode,
			artifact.CanonicalJSON,
			artifact.CreatedAt.Add(time.Minute),
		).Error
		require.Error(t, err)
	})
}
