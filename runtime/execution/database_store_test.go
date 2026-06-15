package execution

import (
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
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

	t.Run("AutoMigrate upgrades legacy execution_commands rows without failing", func(t *testing.T) {
		t.Parallel()

		fake := newFake(t)
		store, err := NewDatabaseStore(":memory:", DatabaseStoreOpts{})
		require.NoError(t, err)

		decisionTime := time.Date(
			fake.IntBetween(2022, 2031),
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 999999999),
			time.UTC,
		)
		eventTime := decisionTime.Add(time.Duration(fake.IntBetween(1, 30)) * time.Minute)
		quantity := float64(fake.IntBetween(1, 100)) + 0.25
		notional := quantity * float64(fake.IntBetween(10, 100))
		limitPrice := notional / quantity

		commandID := randomWord(t, fake, "command")
		traceID := randomWord(t, fake, "trace")
		intentID := randomWord(t, fake, "intent")
		decisionRef := randomWord(t, fake, "decision-ref")
		strategyID := randomWord(t, fake, "strategy")
		strategyVersion := randomWord(t, fake, "version")
		strategyArtifactHash := randomWord(t, fake, "artifact")
		venue := randomWord(t, fake, "venue")
		symbol := strings.ToUpper(randomWord(t, fake, "symbol"))

		require.NoError(t, store.db.Exec(`
			CREATE TABLE execution_commands (
				command_id TEXT NOT NULL PRIMARY KEY,
				trace_id TEXT,
				intent_id TEXT,
				governor_decision_reference TEXT NOT NULL,
				mode TEXT NOT NULL,
				strategy_id TEXT NOT NULL,
				strategy_version TEXT NOT NULL,
				strategy_artifact_hash TEXT NOT NULL,
				venue TEXT NOT NULL,
				symbol TEXT NOT NULL,
				asset_class TEXT NOT NULL,
				action_kind TEXT NOT NULL,
				order_type TEXT NOT NULL,
				limit_price REAL,
				reduce_only NUMERIC NOT NULL,
				approved_quantity REAL NOT NULL,
				approved_notional REAL NOT NULL,
				decision_status TEXT NOT NULL,
				decision_reason TEXT NOT NULL,
				decision_time DATETIME NOT NULL,
				status TEXT NOT NULL,
				event_time DATETIME NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)
		`).Error)

		require.NoError(t, store.db.Exec(`
			INSERT INTO execution_commands (
				command_id,
				trace_id,
				intent_id,
				governor_decision_reference,
				mode,
				strategy_id,
				strategy_version,
				strategy_artifact_hash,
				venue,
				symbol,
				asset_class,
				action_kind,
				order_type,
				limit_price,
				reduce_only,
				approved_quantity,
				approved_notional,
				decision_status,
				decision_reason,
				decision_time,
				status,
				event_time,
				created_at,
				updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			commandID,
			traceID,
			intentID,
			decisionRef,
			domain.DecisionModeBacktest.String(),
			strategyID,
			strategyVersion,
			strategyArtifactHash,
			venue,
			symbol,
			domain.AssetClassCrypto.String(),
			domain.CandidateActionKindLong.String(),
			domain.OrderTypeLimit.String(),
			limitPrice,
			true,
			quantity,
			notional,
			domain.GovernorDecisionStatusApproved.String(),
			domain.GovernorDecisionReasonEligible.String(),
			decisionTime,
			domain.ExecutionCommandStatusCreated.String(),
			eventTime,
			eventTime,
			eventTime,
		).Error)

		require.NoError(t, store.AutoMigrate())

		var columns []sqliteExecutionCommandColumnRow
		require.NoError(
			t,
			store.db.Raw("PRAGMA table_info('execution_commands')").Scan(&columns).Error,
		)

		columnByName := make(map[string]sqliteExecutionCommandColumnRow, len(columns))
		for _, column := range columns {
			columnByName[column.Name] = column
		}

		require.Zero(t, columnByName["approved_instrument_active"].NotNull)
		require.Zero(t, columnByName["approved_timeframe"].NotNull)
		require.Zero(t, columnByName["approved_quality"].NotNull)

		model, err := store.findExecutionCommandModel(t.Context(), commandID)
		require.NoError(t, err)

		command, err := executionCommandModelToDomain(model)
		require.NoError(t, err)
		require.Equal(t, commandID, string(command.CommandID))
		require.InDelta(t, quantity, command.Quantity, 0)
		require.InDelta(t, notional, command.Notional, 0)
		require.True(t, command.ApprovedDecision.CandidateAction.Strategy.Instrument.Active)
		require.Equal(t, domain.Timeframe1m, command.ApprovedDecision.CandidateAction.Strategy.Timeframe)
		require.Equal(t, domain.DataQualityValidated, command.ApprovedDecision.CandidateAction.Quality)
		require.Equal(t, decisionTime, command.ApprovedDecision.CandidateAction.InputRange.Start)
		require.Equal(t, decisionTime.Add(time.Minute), command.ApprovedDecision.CandidateAction.InputRange.End)
	})
}
