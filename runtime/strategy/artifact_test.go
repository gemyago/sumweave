package strategy

import (
	"crypto/sha256"
	"encoding/hex"
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

func TestStrategyArtifact(t *testing.T) {
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

	makeRawPayload := func(
		kind string,
		venue string,
		symbol string,
		assetClass string,
		timeframe string,
		active bool,
		fastWindow int,
		slowWindow int,
	) []byte {
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
			kind,
			venue,
			symbol,
			assetClass,
			active,
			timeframe,
			fastWindow,
			slowWindow,
		)
	}

	makeExpectedCanonicalJSON := func(
		venue string,
		symbol string,
		assetClass string,
		timeframe string,
		active bool,
		fastWindow int,
		slowWindow int,
	) []byte {
		return fmt.Appendf(
			nil,
			`{"schemaVersion":%q,"artifactKind":%q,"strategy":{"kind":%q,"instrument":{"venue":%q,"symbol":%q,"assetClass":%q,"active":%t},"timeframe":%q,"parameters":{"fastWindow":%d,"slowWindow":%d}}}`,
			ArtifactSchemaVersion,
			ArtifactKind,
			domain.StrategyKindMovingAverageCrossover.String(),
			venue,
			symbol,
			assetClass,
			active,
			timeframe,
			fastWindow,
			slowWindow,
		)
	}

	t.Run("NewArtifactFromDSLV0", func(t *testing.T) {
		t.Parallel()

		t.Run("creates versioned artifact metadata and canonical hash", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := "  " + randomWord(t, fake, "venue") + "  "
			symbol := "  " + strings.ToUpper(randomWord(t, fake, "symbol")) + "  "
			assetClass := "  CRYPTO  "
			timeframe := " 1H "
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			artifact, err := NewArtifactFromDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				venue,
				symbol,
				assetClass,
				timeframe,
				active,
				fastWindow,
				slowWindow,
			))

			require.NoError(t, err)
			require.Zero(t, artifact.CreatedAt)
			require.Equal(t, ArtifactSchemaVersion, artifact.SchemaVersion)
			require.Equal(t, ArtifactKind, artifact.ArtifactKind)
			require.Equal(t, makeExpectedCanonicalJSON(
				strings.TrimSpace(venue),
				strings.TrimSpace(symbol),
				domain.AssetClassCrypto.String(),
				domain.Timeframe1h.String(),
				active,
				fastWindow,
				slowWindow,
			), artifact.CanonicalJSON)

			hash := sha256.Sum256(artifact.CanonicalJSON)
			require.Equal(t, hex.EncodeToString(hash[:]), artifact.Hash)
			require.Regexp(t, `^[0-9a-f]{64}$`, artifact.Hash)
		})

		t.Run("returns stable canonical bytes and hashes for equivalent payloads", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			venue := randomWord(t, fake, "venue")
			symbol := strings.ToUpper(randomWord(t, fake, "symbol"))
			active := fake.Bool()
			fastWindow := fake.IntBetween(1, 20)
			slowWindow := fastWindow + fake.IntBetween(1, 20)

			artifactA, err := NewArtifactFromDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				venue,
				symbol,
				domain.AssetClassCrypto.String(),
				domain.Timeframe1m.String(),
				active,
				fastWindow,
				slowWindow,
			))
			require.NoError(t, err)

			artifactB, err := NewArtifactFromDSLV0(fmt.Appendf(
				nil,
				`{ "parameters": { "slowWindow": %d, "fastWindow": %d }, "timeframe": %q, "instrument": { "active": %t, "assetClass": %q, "symbol": %q, "venue": %q }, "kind": %q }`,
				slowWindow,
				fastWindow,
				" 1M ",
				active,
				" crypto ",
				"  "+symbol+"  ",
				"  "+venue+"  ",
				"  MOVING-AVERAGE-CROSSOVER  ",
			))
			require.NoError(t, err)

			require.Equal(t, artifactA.CanonicalJSON, artifactB.CanonicalJSON)
			require.Equal(t, artifactA.Hash, artifactB.Hash)
			require.Equal(t, artifactA.SchemaVersion, artifactB.SchemaVersion)
			require.Equal(t, artifactA.ArtifactKind, artifactB.ArtifactKind)
		})

		t.Run("returns no artifact for invalid DSL", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)

			artifact, err := NewArtifactFromDSLV0(makeRawPayload(
				domain.StrategyKindMovingAverageCrossover.String(),
				randomWord(t, fake, "venue"),
				strings.ToUpper(randomWord(t, fake, "symbol")),
				domain.AssetClassCrypto.String(),
				domain.Timeframe1m.String(),
				fake.Bool(),
				fake.IntBetween(10, 20),
				fake.IntBetween(1, 9),
			))

			require.ErrorIs(t, err, ErrValidation)
			require.Equal(t, Artifact{}, artifact)
		})
	})
}

func TestStrategyArtifactDatabaseStore(t *testing.T) {
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
			[]domain.Timeframe{domain.Timeframe1m, domain.Timeframe1h, domain.Timeframe4h}[index%3].String(),
			fastWindow,
			slowWindow,
		)
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

	t.Run("migration uses explicit table and column names with unique hash index", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var columns []sqliteArtifactTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"strategy_artifacts"),
			).Scan(&columns).Error,
		)
		require.Equal(t, []string{
			"hash",
			"schema_version",
			"artifact_kind",
			"canonical_json",
			"created_at",
		}, columnNames(columns))
		require.True(
			t,
			hasUniqueIndexWithColumns(t, store, tablePrefix+"strategy_artifacts", []string{"hash"}),
		)
		require.True(t, store.db.Migrator().HasIndex(&strategyArtifactModel{}, "idx_strategy_artifacts_created_at"))
	})

	t.Run("Create Get and List persist immutable artifacts", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		ctx := t.Context()

		firstRaw := makeRawPayload(t, fake, 1)
		firstCreated, err := store.Create(ctx, firstRaw)
		require.NoError(t, err)
		require.NotZero(t, firstCreated.CreatedAt)
		require.NotZero(t, firstCreated.CreatedAt)
		require.Equal(t, int64(1), readCount(t, store, "strategy_artifacts"))

		fetched, err := store.Get(ctx, firstCreated.Hash)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		require.Equal(t, firstCreated, *fetched)

		duplicateCreated, err := store.Create(ctx, fmt.Appendf(
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
		require.NoError(t, err)
		require.Equal(t, firstCreated, duplicateCreated)
		require.Equal(t, int64(1), readCount(t, store, "strategy_artifacts"))

		missing, err := store.Get(ctx, randomWord(t, fake, "missing-hash"))
		require.ErrorIs(t, err, ErrArtifactNotFound)
		require.Nil(t, missing)

		secondCreated, err := store.Create(ctx, makeRawPayload(t, fake, 2))
		require.NoError(t, err)

		thirdCreated, err := store.Create(ctx, makeRawPayload(t, fake, 3))
		require.NoError(t, err)

		listed, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, listed, 3)

		expected := []Artifact{firstCreated, secondCreated, thirdCreated}
		slices.SortStableFunc(expected, func(left, right Artifact) int {
			if compare := left.CreatedAt.Compare(right.CreatedAt); compare != 0 {
				return compare
			}
			return strings.Compare(left.Hash, right.Hash)
		})
		require.Equal(t, expected, listed)
		require.Equal(t, firstCreated.CanonicalJSON, listed[0].CanonicalJSON)
		require.Equal(t, firstCreated.Hash, listed[0].Hash)
		require.Equal(t, firstCreated.CreatedAt, listed[0].CreatedAt)
	})

	t.Run("List preserves canonical creation timestamp ordering", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))

		earlierArtifact, err := store.Create(t.Context(), makeRawPayload(t, fake, 1))
		require.NoError(t, err)
		laterArtifact, err := store.Create(t.Context(), makeRawPayload(t, fake, 2))
		require.NoError(t, err)
		require.NoError(t, store.db.Model(&strategyArtifactModel{}).
			Where("hash = ?", earlierArtifact.Hash).
			UpdateColumn("created_at", earlier).Error)
		require.NoError(t, store.db.Model(&strategyArtifactModel{}).
			Where("hash = ?", laterArtifact.Hash).
			UpdateColumn("created_at", later).Error)

		listed, err := store.List(t.Context())
		require.NoError(t, err)
		require.Equal(t, []string{earlierArtifact.Hash, laterArtifact.Hash}, []string{listed[0].Hash, listed[1].Hash})
		require.Equal(t, earlier.Format(time.RFC3339Nano), listed[0].CreatedAt.Format(time.RFC3339Nano))
		require.Equal(t, later.Format(time.RFC3339Nano), listed[1].CreatedAt.Format(time.RFC3339Nano))
	})

	t.Run("schema enforces unique hashes", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store := makeStore(t, ":memory:", "")
		artifact, err := store.Create(t.Context(), makeRawPayload(t, fake, 1))
		require.NoError(t, err)

		err = store.db.Exec(
			"INSERT INTO strategy_artifacts (hash, schema_version, artifact_kind, canonical_json, created_at) VALUES (?, ?, ?, ?, ?)",
			artifact.Hash,
			artifact.SchemaVersion,
			artifact.ArtifactKind,
			artifact.CanonicalJSON,
			artifact.CreatedAt.Add(time.Minute),
		).Error
		require.Error(t, err)
	})

	t.Run("sqlite create waits through a transient writer lock", func(t *testing.T) {
		fake := newFake(t)
		dsn := t.TempDir() + "/artifact-lock.sqlite"
		store := makeStore(t, dsn, "test_")

		firstRaw := makeRawPayload(t, fake, 1)
		firstArtifact, err := NewArtifactFromDSLV0(firstRaw)
		require.NoError(t, err)
		_, err = store.Create(t.Context(), firstRaw)
		require.NoError(t, err)

		locker, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		defer func() { require.NoError(t, locker.Close()) }()

		tx, err := locker.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(
			t.Context(),
			"UPDATE test_strategy_artifacts SET schema_version = schema_version WHERE hash = ?",
			firstArtifact.Hash,
		)
		require.NoError(t, err)

		secondRaw := makeRawPayload(t, fake, 2)
		createCh := make(chan error, 1)
		go func() {
			_, createErr := store.Create(t.Context(), secondRaw)
			createCh <- createErr
		}()

		time.Sleep(150 * time.Millisecond)
		require.NoError(t, tx.Commit())

		select {
		case createErr := <-createCh:
			require.NoError(t, createErr)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for artifact create to finish")
		}
	})
}
