package execution

import (
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/gemyago/signal-foundry/runtime/internal/sqlconn"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type sqliteExecutionCommandColumnRow struct {
	Name    string `gorm:"column:name"`
	NotNull int    `gorm:"column:notnull"`
}

func TestDatabaseStore(t *testing.T) {
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

	t.Run("AutoMigrate requires complete approved decision state", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		sqlDB, err := sqlconn.Open(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

		store, err := NewDatabaseStore(sqlDB, ":memory:", DatabaseStoreOpts{})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())

		decisionTime := time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.FixedZone(randomWord(t, fake, "zone"), fake.IntBetween(-11, 12)*3600),
		)
		inputStart := decisionTime.Add(-2 * time.Minute)
		inputEnd := decisionTime.Add(-time.Minute)
		eventTime := decisionTime.Add(time.Minute)
		quantity := float64(fake.IntBetween(1, 100)) + 0.25
		notional := quantity * float64(fake.IntBetween(10, 100))
		limitPrice := notional / quantity

		var columns []sqliteExecutionCommandColumnRow
		require.NoError(
			t,
			store.db.Raw("PRAGMA table_info('execution_commands')").Scan(&columns).Error,
		)

		columnByName := make(map[string]sqliteExecutionCommandColumnRow, len(columns))
		for _, column := range columns {
			columnByName[column.Name] = column
		}

		for _, columnName := range []string{
			"approved_instrument_active",
			"approved_timeframe",
			"approved_input_start",
			"approved_input_end",
			"approved_quality",
		} {
			require.Equal(t, 1, columnByName[columnName].NotNull, columnName)
		}

		model := executionCommandModel{
			CommandID:                 randomWord(t, fake, "command"),
			TraceID:                   randomWord(t, fake, "trace"),
			IntentID:                  randomWord(t, fake, "intent"),
			GovernorDecisionReference: randomWord(t, fake, "decision-ref"),
			Mode:                      domain.DecisionModeBacktest.String(),
			StrategyID:                randomWord(t, fake, "strategy"),
			StrategyVersion:           randomWord(t, fake, "version"),
			StrategyArtifactHash:      randomWord(t, fake, "artifact"),
			Venue:                     randomWord(t, fake, "venue"),
			Symbol:                    strings.ToUpper(randomWord(t, fake, "symbol")),
			AssetClass:                domain.AssetClassCrypto.String(),
			ActionKind:                domain.CandidateActionKindLong.String(),
			OrderType:                 domain.OrderTypeLimit.String(),
			LimitPrice:                &limitPrice,
			ApprovedQuantity:          quantity,
			ApprovedNotional:          notional,
			ApprovedTimeframe:         domain.Timeframe1m.String(),
			ApprovedInputStart:        inputStart,
			ApprovedInputEnd:          inputEnd,
			ApprovedQuality:           domain.DataQualityValidated.String(),
			DecisionStatus:            domain.GovernorDecisionStatusApproved.String(),
			DecisionReason:            domain.GovernorDecisionReasonEligible.String(),
			DecisionTime:              decisionTime,
			Status:                    domain.ExecutionCommandStatusCreated.String(),
			EventTime:                 eventTime,
		}

		command, err := executionCommandModelToDomain(model)
		require.NoError(t, err)
		require.Equal(t, model.CommandID, string(command.CommandID))
		require.InDelta(t, quantity, command.Quantity, 0)
		require.InDelta(t, notional, command.Notional, 0)
		require.False(t, command.ApprovedDecision.CandidateAction.Strategy.Instrument.Active)
		require.Equal(t, domain.Timeframe1m, command.ApprovedDecision.CandidateAction.Strategy.Timeframe)
		require.Equal(t, domain.DataQualityValidated, command.ApprovedDecision.CandidateAction.Quality)
		require.Equal(t, inputStart, command.ApprovedDecision.CandidateAction.InputRange.Start)
		require.Equal(t, inputEnd, command.ApprovedDecision.CandidateAction.InputRange.End)
		require.Equal(t, decisionTime, command.ApprovedDecision.DecisionTime.Time())
		require.Equal(t, eventTime, command.EventTime.Time())

		model.ApprovedInputStart = time.Time{}
		_, err = executionCommandModelToDomain(model)
		require.Error(t, err)
	})
}
