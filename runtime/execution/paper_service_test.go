package execution

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/data"
	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type sqliteExecutionIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteExecutionIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

type sqliteExecutionTableInfoRow struct {
	Name string `gorm:"column:name"`
}

func TestPaperService(t *testing.T) {
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

		store, err := NewDatabaseStore(dsn, DatabaseStoreOpts{TablePrefix: tablePrefix})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		return store
	}

	makeService := func(t *testing.T) (*PaperService, *DatabaseStore) {
		t.Helper()

		store := makeStore(t, ":memory:", "")
		svc, err := NewPaperService(store)
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

	makeRange := func(t *testing.T, start time.Time, minutes int) domain.TimeRange {
		t.Helper()

		timeRange, err := domain.NewTimeRange(start, start.Add(time.Duration(minutes)*time.Minute))
		require.NoError(t, err)

		return timeRange
	}

	makeAction := func(
		t *testing.T,
		instrument domain.Instrument,
		decisionTime time.Time,
		kind domain.CandidateActionKind,
	) domain.CandidateAction {
		t.Helper()

		strategyIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			Kind:       domain.StrategyKindMovingAverageCrossover,
		})
		require.NoError(t, err)

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategyIdentity,
			Kind:         kind,
			DecisionTime: decisionTime,
			InputRange:   makeRange(t, decisionTime.Add(-5*time.Minute), 5),
			Quality:      domain.DataQualityValidated,
		})
		require.NoError(t, err)

		return action
	}

	makeDecision := func(t *testing.T, action domain.CandidateAction) domain.GovernorDecision {
		t.Helper()

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: action,
			Status:          domain.GovernorDecisionStatusApproved,
			Reason:          domain.GovernorDecisionReasonOK,
			DecisionTime:    action.DecisionTime.Time(),
		})
		require.NoError(t, err)

		return decision
	}

	makeIntent := func(
		t *testing.T,
		fake faker.Faker,
		instrument domain.Instrument,
		decision domain.GovernorDecision,
		mode domain.DecisionMode,
	) domain.OrderIntent {
		t.Helper()

		limitPrice := fake.Float64(2, 50, 5000)
		intent, err := domain.NewOrderIntent(domain.OrderIntentParams{
			IntentID:                 randomWord(t, fake, "intent-id"),
			TraceID:                  randomWord(t, fake, "trace-id"),
			StrategyID:               randomWord(t, fake, "strategy-id"),
			StrategyVersion:          randomWord(t, fake, "strategy-version"),
			StrategyArtifactHash:     randomWord(t, fake, "strategy-hash"),
			Mode:                     mode,
			Instrument:               instrument,
			Timeframe:                decision.CandidateAction.Strategy.Timeframe,
			ActionKind:               decision.CandidateAction.Kind,
			OrderType:                domain.OrderTypeLimit,
			RequestedQuantity:        fake.Float64(2, 1, 25),
			RequestedNotional:        0,
			RequestedLimitPrice:      &limitPrice,
			ReduceOnly:               false,
			SourceReasonCode:         "OK",
			CandidateActionReference: randomWord(t, fake, "candidate-action-ref"),
			CreatedTime:              decision.DecisionTime.Time(),
			Status:                   domain.OrderIntentStatusApproved,
			Metadata:                 map[string]string{"flow": "paper-backtest"},
		})
		require.NoError(t, err)

		intent.RequestedNotional = intent.RequestedQuantity * *intent.RequestedLimitPrice
		intent, err = domain.NewOrderIntent(domain.OrderIntentParams{
			IntentID:                 string(intent.IntentID),
			TraceID:                  string(intent.TraceID),
			StrategyID:               intent.StrategyID,
			StrategyVersion:          intent.StrategyVersion,
			StrategyArtifactHash:     intent.StrategyArtifactHash,
			Mode:                     intent.Mode,
			Instrument:               intent.Instrument,
			Timeframe:                intent.Timeframe,
			ActionKind:               intent.ActionKind,
			OrderType:                intent.OrderType,
			RequestedQuantity:        intent.RequestedQuantity,
			RequestedNotional:        intent.RequestedNotional,
			RequestedLimitPrice:      intent.RequestedLimitPrice,
			ReduceOnly:               intent.ReduceOnly,
			SourceReasonCode:         intent.SourceReasonCode,
			CandidateActionReference: intent.CandidateActionReference,
			CreatedTime:              intent.CreatedTime.Time(),
			Status:                   intent.Status,
			Metadata:                 intent.Metadata,
		})
		require.NoError(t, err)

		return intent
	}

	makeReplayCandle := func(
		t *testing.T,
		instrument domain.Instrument,
		end time.Time,
		low float64,
		high float64,
		closePrice float64,
		identity uint64,
	) data.ReplayCandle {
		t.Helper()

		candle, err := domain.NewCandle(domain.CandleParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe1m,
			TimeRange:  makeRange(t, end.Add(-time.Minute), 1),
			Open:       closePrice,
			High:       high,
			Low:        low,
			Close:      closePrice,
			Volume:     1,
			Quality:    domain.DataQualityValidated,
			Provenance: domain.SourceProvenance{Source: "replay", RecordID: strconv.FormatUint(identity, 10)},
		})
		require.NoError(t, err)

		return data.ReplayCandle{Identity: identity, Candle: candle}
	}

	readCount := func(t *testing.T, store *DatabaseStore, tableName string) int64 {
		t.Helper()

		var count int64
		require.NoError(t, store.db.WithContext(t.Context()).Table(tableName).Count(&count).Error)

		return count
	}

	hasUniqueIndexWithColumns := func(t *testing.T, store *DatabaseStore, tableName string, want []string) bool {
		t.Helper()

		var indexes []sqliteExecutionIndexListRow
		require.NoError(t, store.db.Raw(fmt.Sprintf("PRAGMA index_list('%s')", tableName)).Scan(&indexes).Error)

		for _, indexRow := range indexes {
			if indexRow.Unique == 0 {
				continue
			}

			var columns []sqliteExecutionIndexInfoRow
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

	t.Run("auto migrates explicit ledger schema and keeps retries idempotent", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		tablePrefix := strings.ReplaceAll(randomWord(t, fake, "sf"), "-", "_") + "_"
		store := makeStore(t, ":memory:", tablePrefix)

		var commandColumns []sqliteExecutionTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"execution_commands"),
			).Scan(&commandColumns).Error,
		)
		require.Equal(t, []string{
			"command_id",
			"trace_id",
			"intent_id",
			"governor_decision_reference",
			"mode",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"venue",
			"symbol",
			"asset_class",
			"action_kind",
			"order_type",
			"limit_price",
			"reduce_only",
			"approved_quantity",
			"approved_notional",
			"approved_instrument_active",
			"approved_timeframe",
			"approved_input_start",
			"approved_input_end",
			"approved_quality",
			"decision_status",
			"decision_reason",
			"decision_time",
			"status",
			"event_time",
			"created_at",
			"updated_at",
		}, columnNames(commandColumns))

		var orderColumns []sqliteExecutionTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"execution_orders"),
			).Scan(&orderColumns).Error,
		)
		require.Equal(t, []string{
			"order_id",
			"command_id",
			"mode",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"venue",
			"symbol",
			"asset_class",
			"order_type",
			"time_in_force",
			"reduce_only",
			"client_order_id",
			"status",
			"quantity",
			"notional",
			"limit_price",
			"event_time",
			"created_at",
			"updated_at",
		}, columnNames(orderColumns))

		var fillColumns []sqliteExecutionTableInfoRow
		require.NoError(
			t,
			store.db.Raw(
				fmt.Sprintf("PRAGMA table_info('%s')", tablePrefix+"execution_fills"),
			).Scan(&fillColumns).Error,
		)
		require.Equal(t, []string{
			"fill_id",
			"command_id",
			"order_id",
			"mode",
			"strategy_id",
			"strategy_version",
			"strategy_artifact_hash",
			"venue",
			"symbol",
			"asset_class",
			"action_kind",
			"quantity",
			"price",
			"fee_amount",
			"slippage_amount",
			"source_market_data_reference",
			"metadata_json",
			"event_time",
			"created_at",
			"updated_at",
		}, columnNames(fillColumns))

		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"execution_commands", []string{"command_id"}))
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"execution_orders", []string{"order_id"}))
		require.True(
			t,
			hasUniqueIndexWithColumns(
				t,
				store,
				tablePrefix+"execution_orders",
				[]string{"client_order_id"},
			),
		)
		require.True(t, hasUniqueIndexWithColumns(t, store, tablePrefix+"execution_fills", []string{"fill_id"}))

		svc, err := NewPaperService(store)
		require.NoError(t, err)
		instrument := makeInstrument(t, fake)
		decisionTime := randomTime(t, fake)
		decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeBacktest)
		candle := makeReplayCandle(
			t,
			instrument,
			decisionTime.Add(time.Minute),
			*intent.RequestedLimitPrice-1,
			*intent.RequestedLimitPrice+1,
			*intent.RequestedLimitPrice,
			11,
		)

		_, err = svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles:    []data.ReplayCandle{candle},
		})
		require.NoError(t, err)
		_, err = svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles:    []data.ReplayCandle{candle},
		})
		require.NoError(t, err)

		require.Equal(t, int64(1), readCount(t, store, tablePrefix+"execution_commands"))
		require.Equal(t, int64(1), readCount(t, store, tablePrefix+"execution_orders"))
		require.Equal(t, int64(1), readCount(t, store, tablePrefix+"execution_fills"))
	})

	t.Run("persists refs UTC timestamps and deterministic client order id", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		decisionTime := randomTime(t, fake)
		decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModePaper)
		fillCandle := makeReplayCandle(
			t,
			instrument,
			decisionTime.Add(time.Minute),
			*intent.RequestedLimitPrice-2,
			*intent.RequestedLimitPrice+1,
			*intent.RequestedLimitPrice,
			21,
		)

		firstResult, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles:    []data.ReplayCandle{fillCandle},
		})
		require.NoError(t, err)
		secondResult, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles:    []data.ReplayCandle{fillCandle},
		})
		require.NoError(t, err)

		require.Equal(t, firstResult.Command.CommandID, secondResult.Command.CommandID)
		require.Equal(t, firstResult.Order.ClientOrderID, secondResult.Order.ClientOrderID)
		require.Equal(t, intent.TraceID, firstResult.Command.TraceID)
		require.Equal(t, intent.IntentID, firstResult.Command.IntentID)
		require.Equal(t, intent.Mode, firstResult.Command.Mode)
		require.Equal(t, intent.StrategyID, firstResult.Command.StrategyID)
		require.Equal(t, intent.Instrument, firstResult.Command.Instrument)
		require.Equal(t, intent.OrderType, firstResult.Order.OrderType)
		require.Equal(t, domain.TimeInForceGTC, firstResult.Order.TimeInForce)
		require.NotEmpty(t, firstResult.Command.GovernorDecisionReference)
		require.Equal(t, time.UTC, firstResult.Command.EventTime.Time().Location())
		require.Equal(t, time.UTC, firstResult.Order.EventTime.Time().Location())
		require.NotNil(t, firstResult.Fill)
		require.Equal(t, time.UTC, firstResult.Fill.EventTime.Time().Location())
	})

	t.Run("preserves approved decision candidate action fields across reloads", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		instrument.Active = false
		decisionTime := randomTime(t, fake)
		inputRange := makeRange(t, decisionTime.Add(-15*time.Minute), 10)

		strategyIdentity, err := domain.NewStrategyIdentity(domain.StrategyIdentityParams{
			Instrument: instrument,
			Timeframe:  domain.Timeframe5m,
			Kind:       domain.StrategyKindMovingAverageCrossover,
		})
		require.NoError(t, err)

		action, err := domain.NewCandidateAction(domain.CandidateActionParams{
			Strategy:     strategyIdentity,
			Kind:         domain.CandidateActionKindLong,
			DecisionTime: decisionTime,
			InputRange:   inputRange,
			Quality:      domain.DataQualityRaw,
		})
		require.NoError(t, err)

		decision, err := domain.NewGovernorDecision(domain.GovernorDecisionParams{
			CandidateAction: action,
			Status:          domain.GovernorDecisionStatusApproved,
			Reason:          domain.GovernorDecisionReasonOK,
			DecisionTime:    decisionTime,
		})
		require.NoError(t, err)

		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeBacktest)
		fillCandle := makeReplayCandle(
			t,
			instrument,
			decisionTime.Add(time.Minute),
			*intent.RequestedLimitPrice-1,
			*intent.RequestedLimitPrice+1,
			*intent.RequestedLimitPrice,
			61,
		)

		result, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles:    []data.ReplayCandle{fillCandle},
		})
		require.NoError(t, err)
		require.NotNil(t, result.Fill)
		require.Len(t, result.Reconciliation.Fills, 1)
		require.Equal(t, decision, result.Order.Command.ApprovedDecision)
		require.Equal(t, decision, result.Reconciliation.Order.Command.ApprovedDecision)
		require.Equal(t, decision, result.Reconciliation.Fills[0].Order.Command.ApprovedDecision)
	})

	t.Run("rejects unsupported live mode before writes", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		instrument := makeInstrument(t, fake)
		decisionTime := randomTime(t, fake)
		decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeLive)

		_, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
		})

		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "live mode is unsupported")
		require.Equal(t, int64(0), readCount(t, store, "execution_commands"))
		require.Equal(t, int64(0), readCount(t, store, "execution_orders"))
		require.Equal(t, int64(0), readCount(t, store, "execution_fills"))
	})

	t.Run("simulates long and short later candle limit fills deterministically", func(t *testing.T) {
		t.Parallel()

		t.Run("long fills when later candle low reaches limit", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			svc, _ := makeService(t)
			instrument := makeInstrument(t, fake)
			decisionTime := randomTime(t, fake)
			decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
			intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeBacktest)

			result, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
				Intent:           intent,
				ApprovedDecision: decision,
				ReplayCandles: []data.ReplayCandle{
					makeReplayCandle(
						t,
						instrument,
						decisionTime,
						*intent.RequestedLimitPrice-10,
						*intent.RequestedLimitPrice+10,
						*intent.RequestedLimitPrice,
						31,
					),
					makeReplayCandle(
						t,
						instrument,
						decisionTime.Add(time.Minute),
						*intent.RequestedLimitPrice-1,
						*intent.RequestedLimitPrice+2,
						*intent.RequestedLimitPrice,
						32,
					),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, result.Fill)
			require.Equal(t, domain.ExecutionOrderStatusFilled, result.Order.Status)
			require.Equal(t, domain.ExecutionOrderStatusFilled, result.Reconciliation.Status)
			require.InDelta(t, result.Order.Quantity, result.Fill.Quantity, 0)
			require.InDelta(t, *intent.RequestedLimitPrice, result.Fill.Price, 0)
			require.Equal(t, "replay-candle:32", result.Fill.SourceMarketDataReference)
			require.Zero(t, result.Fill.FeeAmount)
			require.Zero(t, result.Fill.SlippageAmount)
			require.Equal(t, map[string]string{
				"fee_model":      "zero",
				"slippage_model": "zero",
				"simulator":      "closed-candle-limit-v0",
			}, result.Fill.Metadata)
		})

		t.Run("short fills when later candle high reaches limit", func(t *testing.T) {
			t.Parallel()

			fake := newFake(t)
			svc, _ := makeService(t)
			instrument := makeInstrument(t, fake)
			decisionTime := randomTime(t, fake)
			decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindShort))
			intent := makeIntent(t, fake, instrument, decision, domain.DecisionModePaper)

			result, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
				Intent:           intent,
				ApprovedDecision: decision,
				ReplayCandles: []data.ReplayCandle{
					makeReplayCandle(
						t,
						instrument,
						decisionTime.Add(time.Minute),
						*intent.RequestedLimitPrice+1,
						*intent.RequestedLimitPrice-1,
						*intent.RequestedLimitPrice,
						41,
					),
					makeReplayCandle(
						t,
						instrument,
						decisionTime.Add(2*time.Minute),
						*intent.RequestedLimitPrice-1,
						*intent.RequestedLimitPrice+1,
						*intent.RequestedLimitPrice,
						42,
					),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, result.Fill)
			require.Equal(t, "replay-candle:42", result.Fill.SourceMarketDataReference)
			require.InDelta(t, result.Order.Quantity, result.Fill.Quantity, 0)
		})
	})

	t.Run("keeps order open when no later candle reaches limit", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, store := makeService(t)
		instrument := makeInstrument(t, fake)
		decisionTime := randomTime(t, fake)
		decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeBacktest)

		result, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
			ReplayCandles: []data.ReplayCandle{
				makeReplayCandle(
					t,
					instrument,
					decisionTime.Add(time.Minute),
					*intent.RequestedLimitPrice+1,
					*intent.RequestedLimitPrice+3,
					*intent.RequestedLimitPrice+2,
					51,
				),
			},
		})
		require.NoError(t, err)
		require.Nil(t, result.Fill)
		require.Equal(t, domain.ExecutionOrderStatusOpen, result.Order.Status)
		require.Equal(t, domain.ExecutionOrderStatusOpen, result.Reconciliation.Status)
		require.Equal(t, int64(0), readCount(t, store, "execution_fills"))
	})

	t.Run("rejects unsupported non limit order requests", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		svc, _ := makeService(t)
		instrument := makeInstrument(t, fake)
		decisionTime := randomTime(t, fake)
		decision := makeDecision(t, makeAction(t, instrument, decisionTime, domain.CandidateActionKindLong))
		intent := makeIntent(t, fake, instrument, decision, domain.DecisionModeBacktest)
		intent.OrderType = domain.OrderType("market")

		_, err := svc.ExecuteApprovedIntent(t.Context(), ExecuteApprovedIntentRequest{
			Intent:           intent,
			ApprovedDecision: decision,
		})

		require.ErrorIs(t, err, ErrValidation)
		require.ErrorContains(t, err, "order intent order type is required")
	})
}

func columnNames(rows []sqliteExecutionTableInfoRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	return names
}
