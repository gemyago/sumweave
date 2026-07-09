package execution

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

type sqliteSnapshotIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteSnapshotIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteSnapshotTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestSnapshotService(t *testing.T) {
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

	randomTime := func(t *testing.T, fake faker.Faker) time.Time {
		t.Helper()

		return time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord(t, fake, "zone"), fake.IntBetween(-11, 12)*3600),
		)
	}

	makeStore := func(t *testing.T, dsn string, tablePrefix string) *DatabaseStore {
		t.Helper()

		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewDatabaseStore(sqlDB, dsn, DatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeService := func(t *testing.T) (*SnapshotService, *DatabaseStore) {
		t.Helper()

		store := makeStore(t, ":memory:", "")
		svc, err := NewSnapshotService(store)
		require.NoError(t, err)

		return svc, store
	}

	makeInstrument := func(t *testing.T, fake faker.Faker) domain.Instrument {
		t.Helper()

		instrument, err := domain.NewInstrument(domain.InstrumentParams{
			Venue:      domain.Venue(randomWord(t, fake, "venue")),
			Symbol:     domain.Symbol(strings.ToUpper(randomWord(t, fake, "symbol"))),
			AssetClass: domain.AssetClassCrypto,
			Active:     true,
		})
		require.NoError(t, err)

		return instrument
	}

	makeFill := func(
		t *testing.T,
		instrument domain.Instrument,
		strategyID string,
		strategyVersion string,
		strategyArtifactHash string,
		mode domain.DecisionMode,
		actionKind domain.CandidateActionKind,
		fillID string,
		quantity float64,
		price float64,
		eventTime time.Time,
	) domain.ExecutionFill {
		t.Helper()

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy: domain.StrategyIdentity{
				Instrument: instrument,
				Timeframe:  domain.Timeframe1m,
				Kind:       domain.StrategyKindMovingAverageCrossover,
			},
			Kind:         actionKind,
			DecisionTime: eventTime.Add(-time.Minute),
			InputRange: domain.TimeRange{
				Start: eventTime.Add(-2 * time.Minute).UTC(),
				End:   eventTime.Add(-time.Minute).UTC(),
			},
			Quality: domain.DataQualityValidated,
		})
		require.NoError(t, err)

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: action,
			Status:          domain.GovernorDecisionStatusApproved,
			Reason:          domain.GovernorDecisionReasonOK,
			DecisionTime:    eventTime.Add(-time.Minute),
		})
		require.NoError(t, err)

		limitPrice := price
		command, err := domain.NewExecutionCommand(domain.ExecutionCommandParams{
			CommandID:                 randomWord(t, newFake(t), "command"),
			Mode:                      mode,
			StrategyID:                strategyID,
			StrategyVersion:           strategyVersion,
			StrategyArtifactHash:      strategyArtifactHash,
			Instrument:                instrument,
			Venue:                     instrument.Venue,
			ActionKind:                actionKind,
			OrderType:                 domain.OrderTypeLimit,
			LimitPrice:                &limitPrice,
			GovernorDecisionReference: randomWord(t, newFake(t), "decision-ref"),
			ApprovedDecision:          decision,
			Status:                    domain.ExecutionCommandStatusCreated,
			Quantity:                  quantity,
			Notional:                  quantity * price,
			EventTime:                 eventTime,
		})
		require.NoError(t, err)

		order, err := domain.NewExecutionOrder(domain.ExecutionOrderParams{
			OrderID:              randomWord(t, newFake(t), "order"),
			Command:              command,
			Mode:                 mode,
			StrategyID:           strategyID,
			StrategyVersion:      strategyVersion,
			StrategyArtifactHash: strategyArtifactHash,
			Venue:                instrument.Venue,
			Instrument:           instrument,
			OrderType:            domain.OrderTypeLimit,
			TimeInForce:          domain.TimeInForceGTC,
			ClientOrderID:        randomWord(t, newFake(t), "client-order"),
			Status:               domain.ExecutionOrderStatusFilled,
			Quantity:             quantity,
			Notional:             quantity * price,
			LimitPrice:           &limitPrice,
			EventTime:            eventTime,
		})
		require.NoError(t, err)

		fill, err := domain.NewExecutionFill(domain.ExecutionFillParams{
			FillID:                    fillID,
			Order:                     order,
			SourceMarketDataReference: randomWord(t, newFake(t), "fill-source"),
			FeeAmount:                 0,
			SlippageAmount:            0,
			Metadata:                  map[string]string{"simulator": "closed-candle-limit-v0"},
			Quantity:                  quantity,
			Price:                     price,
			EventTime:                 eventTime,
		})
		require.NoError(t, err)

		return fill
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteSnapshotIndexListRow
		require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteSnapshotIndexInfoRow
			require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_info('%s')", indexRow.Name)).Scan(&columns).Error)

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

	t.Run("projects deterministic position snapshots from fills", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		strategyID := randomWord(t, fake, "strategy")
		strategyVersion := randomWord(t, fake, "strategy-version")
		strategyArtifactHash := randomWord(t, fake, "strategy-hash")
		baseTime := randomTime(t, fake)

		positionSnapshots, err := svc.RecordPositionSnapshots(t.Context(), []domain.ExecutionFill{
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindShort,
				"fill-04",
				1.5,
				125,
				baseTime.Add(3*time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-02",
				2,
				110,
				baseTime.Add(time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-01",
				1,
				100,
				baseTime.Add(time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindShort,
				"fill-03",
				1.5,
				120,
				baseTime.Add(2*time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindShort,
				"fill-05",
				2.5,
				130,
				baseTime.Add(4*time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-06",
				1,
				120,
				baseTime.Add(5*time.Minute),
			),
		})
		require.NoError(t, err)
		require.Len(t, positionSnapshots, 6)

		require.Equal(t, "fill-01", string(positionSnapshots[0].SourceFillID))
		require.InDelta(t, 1, positionSnapshots[0].Quantity, 0)
		require.InDelta(t, 100, *positionSnapshots[0].AverageEntryPrice, 0)

		require.Equal(t, "fill-02", string(positionSnapshots[1].SourceFillID))
		require.InDelta(t, 3, positionSnapshots[1].Quantity, 0)
		require.InDelta(t, 106.6666666667, *positionSnapshots[1].AverageEntryPrice, 1e-9)

		require.Equal(t, "fill-03", string(positionSnapshots[2].SourceFillID))
		require.InDelta(t, 1.5, positionSnapshots[2].Quantity, 0)
		require.InDelta(t, 20, positionSnapshots[2].RealizedPnL, 1e-9)
		require.InDelta(t, 160, positionSnapshots[2].ExposureNotional, 1e-9)

		require.Equal(t, "fill-04", string(positionSnapshots[3].SourceFillID))
		require.Zero(t, positionSnapshots[3].Quantity)
		require.Nil(t, positionSnapshots[3].AverageEntryPrice)
		require.InDelta(t, 47.5, positionSnapshots[3].RealizedPnL, 1e-9)

		require.Equal(t, "fill-05", string(positionSnapshots[4].SourceFillID))
		require.InDelta(t, -2.5, positionSnapshots[4].Quantity, 0)
		require.InDelta(t, 130, *positionSnapshots[4].AverageEntryPrice, 0)

		require.Equal(t, "fill-06", string(positionSnapshots[5].SourceFillID))
		require.InDelta(t, -1.5, positionSnapshots[5].Quantity, 0)
		require.InDelta(t, 57.5, positionSnapshots[5].RealizedPnL, 1e-9)
		require.InDelta(t, 195, positionSnapshots[5].ExposureNotional, 1e-9)
		require.Equal(t, time.UTC, positionSnapshots[5].EventTime.Time().Location())
		require.Equal(t, map[string]string{
			"funding_model":     "deferred",
			"leverage_model":    "not-modeled",
			"liquidation_model": "deferred",
			"margin_model":      "deferred",
			"projection":        "fill-ledger-v0",
		}, positionSnapshots[5].Metadata)
	})

	t.Run("rejects unsupported position reversal", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		strategyID := randomWord(t, fake, "strategy")
		strategyVersion := randomWord(t, fake, "strategy-version")
		strategyArtifactHash := randomWord(t, fake, "strategy-hash")
		baseTime := randomTime(t, fake)

		_, err := svc.RecordPositionSnapshots(t.Context(), []domain.ExecutionFill{
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-01",
				1,
				100,
				baseTime,
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindShort,
				"fill-02",
				2,
				110,
				baseTime.Add(time.Minute),
			),
		})

		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "reversal is unsupported")
	})

	t.Run("projects portfolio snapshots with optional unrealized pnl metadata and queries", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		instrument := makeInstrument(t, fake)
		otherInstrument := makeInstrument(t, fake)
		strategyID := randomWord(t, fake, "strategy")
		strategyVersion := randomWord(t, fake, "strategy-version")
		strategyArtifactHash := randomWord(t, fake, "strategy-hash")
		baseTime := randomTime(t, fake)

		positionSnapshots, err := svc.RecordPositionSnapshots(t.Context(), []domain.ExecutionFill{
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				domain.CandidateActionKindLong,
				"fill-a",
				2,
				100,
				baseTime,
			),
			makeFill(
				t,
				otherInstrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				domain.CandidateActionKindShort,
				"fill-b",
				1,
				200,
				baseTime.Add(time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				domain.CandidateActionKindShort,
				"fill-c",
				1,
				110,
				baseTime.Add(2*time.Minute),
			),
		})
		require.NoError(t, err)

		portfolioSnapshots, err := svc.RecordPortfolioSnapshots(t.Context(), ProjectPortfolioSnapshotsRequest{
			PositionSnapshots: positionSnapshots,
			MarkPrices: []PositionMarkPrice{
				{Instrument: instrument, Price: 120},
				{Instrument: otherInstrument, Price: 180},
			},
		})
		require.NoError(t, err)
		require.Len(t, portfolioSnapshots, 3)

		require.InDelta(t, 200, portfolioSnapshots[0].GrossExposure, 1e-9)
		require.InDelta(t, 200, portfolioSnapshots[0].NetExposure, 1e-9)
		require.Zero(t, portfolioSnapshots[0].RealizedPnL)
		require.NotNil(t, portfolioSnapshots[0].UnrealizedPnL)
		require.InDelta(t, 40, *portfolioSnapshots[0].UnrealizedPnL, 1e-9)

		require.InDelta(t, 400, portfolioSnapshots[1].GrossExposure, 1e-9)
		require.InDelta(t, 0, portfolioSnapshots[1].NetExposure, 1e-9)
		require.NotNil(t, portfolioSnapshots[1].UnrealizedPnL)
		require.InDelta(t, 60, *portfolioSnapshots[1].UnrealizedPnL, 1e-9)

		require.InDelta(t, 300, portfolioSnapshots[2].GrossExposure, 1e-9)
		require.InDelta(t, -100, portfolioSnapshots[2].NetExposure, 1e-9)
		require.InDelta(t, 10, portfolioSnapshots[2].RealizedPnL, 1e-9)
		require.NotNil(t, portfolioSnapshots[2].UnrealizedPnL)
		require.InDelta(t, 40, *portfolioSnapshots[2].UnrealizedPnL, 1e-9)
		require.Equal(t, map[string]string{
			"collateral_model":     "not-modeled",
			"funding_model":        "deferred",
			"leverage_model":       "not-modeled",
			"liquidation_model":    "deferred",
			"margin_model":         "deferred",
			"projection":           "fill-ledger-v0",
			"unrealized_pnl_model": "mark-price-v0",
		}, portfolioSnapshots[2].Metadata)

		withoutMarks, err := svc.RecordPortfolioSnapshots(t.Context(), ProjectPortfolioSnapshotsRequest{
			PositionSnapshots: positionSnapshots,
		})
		require.NoError(t, err)
		require.Nil(t, withoutMarks[len(withoutMarks)-1].UnrealizedPnL)
		require.Equal(t, "deferred", withoutMarks[len(withoutMarks)-1].Metadata["unrealized_pnl_model"])

		mode := domain.DecisionModeBacktest
		timeRange, err := domain.NewTimeRange(baseTime.Add(-time.Minute), baseTime.Add(3*time.Minute))
		require.NoError(t, err)
		queried, err := svc.QueryPortfolioSnapshots(t.Context(), PortfolioSnapshotQuery{
			Mode:      &mode,
			TimeRange: &timeRange,
		})
		require.NoError(t, err)
		require.Len(t, queried, 3)
		require.Equal(t, int64(3), readCount(t, store, "portfolio_snapshots"))
	})

	t.Run("migrates snapshot schema and supports deterministic filtered queries", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var positionColumns []sqliteSnapshotTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"position_snapshots"),
			).Scan(&positionColumns).Error,
		)
		require.Equal(t, []string{
			"snapshot_id",
			"fill_id",
			"mode",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"venue",
			"symbol",
			"asset_class",
			"quantity",
			"average_entry_price",
			"realized_pnl",
			"exposure_notional",
			"metadata_json",
			"event_time",
			"created_at",
			"updated_at",
		}, snapshotColumnNames(positionColumns))

		var portfolioColumns []sqliteSnapshotTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"portfolio_snapshots"),
			).Scan(&portfolioColumns).Error,
		)
		require.Equal(t, []string{
			"snapshot_id",
			"fill_id",
			"mode",
			"gross_exposure",
			"net_exposure",
			"realized_pnl",
			"unrealized_pnl",
			"metadata_json",
			"event_time",
			"created_at",
			"updated_at",
		}, snapshotColumnNames(portfolioColumns))

		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"position_snapshots", []string{"snapshot_id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"portfolio_snapshots", []string{"snapshot_id"}))

		svc, err := NewSnapshotService(store)
		require.NoError(t, err)
		instrument := makeInstrument(t, fake)
		otherInstrument := makeInstrument(t, fake)
		strategyID := randomWord(t, fake, "strategy")
		otherStrategyID := randomWord(t, fake, "other-strategy")
		strategyVersion := randomWord(t, fake, "strategy-version")
		strategyArtifactHash := randomWord(t, fake, "strategy-hash")
		baseTime := randomTime(t, fake)

		positionSnapshots, err := svc.RecordPositionSnapshots(t.Context(), []domain.ExecutionFill{
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-02",
				1,
				101,
				baseTime.Add(2*time.Minute),
			),
			makeFill(
				t,
				otherInstrument,
				otherStrategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModeBacktest,
				domain.CandidateActionKindLong,
				"fill-03",
				1,
				201,
				baseTime.Add(3*time.Minute),
			),
			makeFill(
				t,
				instrument,
				strategyID,
				strategyVersion,
				strategyArtifactHash,
				domain.DecisionModePaper,
				domain.CandidateActionKindLong,
				"fill-01",
				1,
				100,
				baseTime.Add(time.Minute),
			),
		})
		require.NoError(t, err)

		mode := domain.DecisionModePaper
		timeRange, err := domain.NewTimeRange(baseTime, baseTime.Add(4*time.Minute))
		require.NoError(t, err)
		positionQueried, err := svc.QueryPositionSnapshots(t.Context(), PositionSnapshotQuery{
			StrategyID: strategyID,
			Instrument: &instrument,
			Mode:       &mode,
			TimeRange:  &timeRange,
		})
		require.NoError(t, err)
		require.Len(t, positionQueried, 2)
		require.Equal(t, "fill-01", string(positionQueried[0].SourceFillID))
		require.Equal(t, "fill-02", string(positionQueried[1].SourceFillID))
		require.Equal(t, int64(3), readCount(t, store, tablePrefix+"position_snapshots"))

		_, err = svc.RecordPortfolioSnapshots(t.Context(), ProjectPortfolioSnapshotsRequest{
			PositionSnapshots: positionSnapshots,
		})
		require.NoError(t, err)
		portfolioQueried, err := svc.QueryPortfolioSnapshots(t.Context(), PortfolioSnapshotQuery{
			Mode:      &mode,
			TimeRange: &timeRange,
		})
		require.NoError(t, err)
		require.Len(t, portfolioQueried, 2)
		require.Equal(t, int64(3), readCount(t, store, tablePrefix+"portfolio_snapshots"))
	})
}

func snapshotColumnNames(rows []sqliteSnapshotTableInfoRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	return names
}
