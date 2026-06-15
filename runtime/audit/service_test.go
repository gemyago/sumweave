package audit

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type sqliteAuditIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteAuditIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteAuditTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestAuditService(t *testing.T) {
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
			fake.IntBetween(2020, 2032),
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

		store, err := NewDatabaseStore(dsn, DatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeService := func(t *testing.T) (*Service, *DatabaseStore) {
		t.Helper()

		store := makeStore(t, ":memory:", "")
		svc, err := NewService(store)
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

	makeTimeRange := func(t *testing.T, fake faker.Faker, start time.Time) domain.TimeRange {
		t.Helper()

		timeRange, err := domain.NewTimeRange(
			start,
			start.Add(time.Duration(fake.IntBetween(1, 180))*time.Minute),
		)
		require.NoError(t, err)

		return timeRange
	}

	makeTrace := func(t *testing.T, fake faker.Faker, instrument domain.Instrument, decisionTime time.Time) domain.DecisionTrace {
		t.Helper()

		trace, err := domain.NewDecisionTrace(domain.DecisionTraceParams{
			TraceID:              randomWord(t, fake, "trace-id"),
			Mode:                 domain.DecisionModeBacktest,
			DecisionTime:         decisionTime,
			StrategyID:           randomWord(t, fake, "strategy-id"),
			StrategyVersion:      randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash: randomWord(t, fake, "strategy-artifact"),
			Instrument:           instrument,
			Timeframe:            domain.Timeframe1m,
			DatasetReference:     randomWord(t, fake, "dataset-ref"),
			RunReference:         randomWord(t, fake, "run-ref"),
			InputRange:           makeTimeRange(t, fake, decisionTime.Add(-5*time.Minute)),
			AnalyticsReference:   randomWord(t, fake, "analytics-ref"),
			DataQuality:          domain.DataQualityValidated,
			EvaluatorName:        randomWord(t, fake, "evaluator-name"),
			EvaluatorVersion:     randomWord(t, fake, "evaluator-version"),
			Result:               domain.DecisionTraceResultIntentCreated,
			ReasonCodes:          []string{"OK"},
			Metadata:             map[string]string{"scope": "audit"},
		})
		require.NoError(t, err)

		return trace
	}

	makeIntent := func(t *testing.T, fake faker.Faker, trace domain.DecisionTrace, createdAt time.Time) domain.OrderIntent {
		t.Helper()

		limitPrice := fake.Float64(2, 1, 10000)
		intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
			IntentID:                 randomWord(t, fake, "intent-id"),
			TraceID:                  string(trace.TraceID),
			StrategyID:               trace.StrategyID,
			StrategyVersion:          trace.StrategyVersion,
			StrategyArtifactHash:     trace.StrategyArtifactHash,
			Mode:                     trace.Mode,
			Instrument:               trace.Instrument,
			Timeframe:                trace.Timeframe,
			ActionKind:               domain.CandidateActionKindLong,
			OrderType:                domain.OrderTypeLimit,
			RequestedQuantity:        fake.Float64(2, 1, 100),
			RequestedLimitPrice:      &limitPrice,
			SourceReasonCode:         "OK",
			CandidateActionReference: randomWord(t, fake, "candidate-action-ref"),
			CreatedTime:              createdAt,
			Status:                   domain.OrderIntentStatusCreated,
			Metadata:                 map[string]string{"flow": "paper-backtest"},
		})
		require.NoError(t, err)

		return intent
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteAuditIndexListRow
		require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteAuditIndexInfoRow
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

	t.Run("records trace and intent with UTC normalization and exact trace linkage", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		trace := makeTrace(t, fake, instrument, randomTime(t, fake))

		persistedTrace, err := svc.RecordTrace(t.Context(), trace)
		require.NoError(t, err)
		require.Equal(t, time.UTC, persistedTrace.DecisionTime.Time().Location())

		intent := makeIntent(t, fake, persistedTrace, randomTime(t, fake))
		persistedIntent, err := svc.CreateOrderIntent(t.Context(), intent)
		require.NoError(t, err)
		require.Equal(t, persistedTrace.TraceID, persistedIntent.TraceID)
		require.Equal(t, time.UTC, persistedIntent.CreatedTime.Time().Location())
	})

	t.Run("rejects unknown trace references for intents", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		trace := makeTrace(t, fake, instrument, randomTime(t, fake))
		intent := makeIntent(t, fake, trace, randomTime(t, fake))

		_, err := svc.CreateOrderIntent(t.Context(), intent)

		require.ErrorIs(t, err, ErrTraceNotFound)
	})

	t.Run("enforces stable order intent status transitions", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		trace, err := svc.RecordTrace(t.Context(), makeTrace(t, fake, instrument, randomTime(t, fake)))
		require.NoError(t, err)
		intent, err := svc.CreateOrderIntent(t.Context(), makeIntent(t, fake, trace, randomTime(t, fake)))
		require.NoError(t, err)

		persisted, err := svc.UpdateOrderIntentStatus(
			t.Context(),
			string(intent.IntentID),
			domain.OrderIntentStatusSentToGovernor,
		)
		require.NoError(t, err)
		require.Equal(t, domain.OrderIntentStatusSentToGovernor, persisted.Status)

		_, err = svc.UpdateOrderIntentStatus(
			t.Context(),
			string(intent.IntentID),
			domain.OrderIntentStatusExecutionCreated,
		)
		require.ErrorIs(t, err, ErrValidation)
	})

	t.Run("domain validation rejects metadata bounds overflow", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		instrument := makeInstrument(t, fake)
		oversized := make(map[string]string, 17)
		for idx := range 17 {
			overSizedKey := randomWord(t, fake, "meta-key") + strconv.Itoa(idx)
			overSizedValue := randomWord(t, fake, "meta-value") + strconv.Itoa(idx)
			oversized[overSizedKey] = overSizedValue
		}

		_, err := domain.NewDecisionTrace(domain.DecisionTraceParams{
			TraceID:              randomWord(t, fake, "trace-id"),
			Mode:                 domain.DecisionModePaper,
			DecisionTime:         randomTime(t, fake),
			StrategyID:           randomWord(t, fake, "strategy-id"),
			StrategyVersion:      randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash: randomWord(t, fake, "strategy-artifact"),
			Instrument:           instrument,
			Timeframe:            domain.Timeframe1m,
			InputRange:           makeTimeRange(t, fake, randomTime(t, fake)),
			DataQuality:          domain.DataQualityValidated,
			EvaluatorName:        randomWord(t, fake, "evaluator-name"),
			EvaluatorVersion:     randomWord(t, fake, "evaluator-version"),
			Result:               domain.DecisionTraceResultError,
			ReasonCodes:          []string{"INVALID_INTENT"},
			Metadata:             oversized,
		})

		require.Error(t, err)
	})

	t.Run("store auto-migrates explicit audit schema and idempotent writes", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var traceColumns []sqliteAuditTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"decision_traces"),
			).Scan(&traceColumns).Error,
		)
		require.Equal(t, []string{
			"trace_id",
			"mode",
			"decision_time",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"venue",
			"symbol",
			"asset_class",
			"timeframe",
			"dataset_reference",
			"run_reference",
			"input_start_at",
			"input_end_at",
			"analytics_reference",
			"data_quality",
			"evaluator_name",
			"evaluator_version",
			"result",
			"reason_codes_json",
			"metadata_json",
			"created_at",
			"updated_at",
		}, columnNames(traceColumns))

		var intentColumns []sqliteAuditTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"order_intents"),
			).Scan(&intentColumns).Error,
		)
		require.Equal(t, []string{
			"intent_id",
			"trace_id",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"mode",
			"venue",
			"symbol",
			"asset_class",
			"timeframe",
			"action_kind",
			"order_type",
			"requested_quantity",
			"requested_notional",
			"requested_limit_price",
			"reduce_only",
			"source_reason_code",
			"candidate_action_reference",
			"created_time",
			"status",
			"metadata_json",
			"created_at",
			"updated_at",
		}, columnNames(intentColumns))

		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"decision_traces", []string{"trace_id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"order_intents", []string{"intent_id"}))

		svc, err := NewService(store)
		require.NoError(t, err)
		instrument := makeInstrument(t, fake)
		trace := makeTrace(t, fake, instrument, randomTime(t, fake))
		persistedTrace, err := svc.RecordTrace(t.Context(), trace)
		require.NoError(t, err)
		_, err = svc.RecordTrace(t.Context(), trace)
		require.NoError(t, err)
		require.Equal(t, int64(1), readCount(t, store, tablePrefix+"decision_traces"))

		intent := makeIntent(t, fake, persistedTrace, randomTime(t, fake))
		_, err = svc.CreateOrderIntent(t.Context(), intent)
		require.NoError(t, err)
		_, err = svc.CreateOrderIntent(t.Context(), intent)
		require.NoError(t, err)
		require.Equal(t, int64(1), readCount(t, store, tablePrefix+"order_intents"))
	})

	t.Run("queries traces and intents by strategy instrument mode and time range", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrumentA := makeInstrument(t, fake)
		instrumentB := makeInstrument(t, fake)
		baseTime := randomTime(t, fake).UTC()

		traceA1, err := svc.RecordTrace(t.Context(), makeTrace(t, fake, instrumentA, baseTime.Add(1*time.Minute)))
		require.NoError(t, err)
		traceA2Draft := makeTrace(t, fake, instrumentA, baseTime.Add(2*time.Minute))
		traceA2Draft.StrategyID = traceA1.StrategyID
		traceA2Draft.StrategyVersion = traceA1.StrategyVersion
		traceA2Draft.StrategyArtifactHash = traceA1.StrategyArtifactHash
		traceA2, err := svc.RecordTrace(t.Context(), traceA2Draft)
		require.NoError(t, err)
		traceB, err := svc.RecordTrace(t.Context(), makeTrace(t, fake, instrumentB, baseTime.Add(3*time.Minute)))
		require.NoError(t, err)

		intentA1, err := svc.CreateOrderIntent(t.Context(), makeIntent(t, fake, traceA1, baseTime.Add(4*time.Minute)))
		require.NoError(t, err)
		intentA2, err := svc.CreateOrderIntent(t.Context(), makeIntent(t, fake, traceA2, baseTime.Add(5*time.Minute)))
		require.NoError(t, err)
		_, err = svc.CreateOrderIntent(t.Context(), makeIntent(t, fake, traceB, baseTime.Add(6*time.Minute)))
		require.NoError(t, err)

		queryRange, err := domain.NewTimeRange(baseTime, baseTime.Add(6*time.Minute))
		require.NoError(t, err)
		mode := domain.DecisionModeBacktest

		traces, err := svc.ListTraces(t.Context(), TraceQuery{
			StrategyID: traceA1.StrategyID,
			Instrument: &instrumentA,
			Mode:       &mode,
			TimeRange:  &queryRange,
		})
		require.NoError(t, err)
		require.Equal(t, []domain.DecisionTrace{traceA1, traceA2}, traces)

		intents, err := svc.ListOrderIntents(t.Context(), OrderIntentQuery{
			StrategyID: traceA1.StrategyID,
			Instrument: &instrumentA,
			Mode:       &mode,
			TimeRange:  &queryRange,
		})
		require.NoError(t, err)
		require.Equal(t, []domain.OrderIntent{intentA1, intentA2}, intents)
	})
}

func columnNames(rows []sqliteAuditTableInfoRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	return names
}
