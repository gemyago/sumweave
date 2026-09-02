//go:build postgres_test

package financeapp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceRegistrationPostgres(t *testing.T) {
	fake := faker.New()

	openPrepared := func(t *testing.T) (*sql.DB, *persistence.Database, appdispatch.Config) {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		database, err := persistence.NewDatabase(db, dsn)
		require.NoError(t, err)
		return db, database, appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "sumweave_"}
	}
	newPublisher := func(t *testing.T, config appdispatch.Config, db *sql.DB) *appdispatch.Publisher {
		t.Helper()
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		return publisher
	}
	assertPublishedMessage := func(
		t *testing.T,
		db *sql.DB,
		config appdispatch.Config,
		messageID string,
		topic string,
	) {
		t.Helper()
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM "`+config.MessagesTable()+`" WHERE uuid=$1 AND topic=$2`,
			messageID,
			topic,
		).Scan(&count))
		assert.Equal(t, 1, count)
	}
	makeScheduleNow := func() time.Time {
		return time.Date(1, time.January, 1, 0, 0, 0, fake.IntBetween(1, 999999999), time.UTC)
	}
	cleanupBankSchedule := func(t *testing.T, db *sql.DB, connectionID string) {
		t.Helper()
		cleanupCtx := context.WithoutCancel(t.Context())
		t.Cleanup(func() {
			_, err := db.ExecContext(
				cleanupCtx,
				`DELETE FROM finance_bank_connection_schedules WHERE connection_id=$1`,
				connectionID,
			)
			require.NoError(t, err)
		})
	}
	cleanupFXSchedule := func(t *testing.T, db *sql.DB, scheduleID string) {
		t.Helper()
		cleanupCtx := context.WithoutCancel(t.Context())
		t.Cleanup(func() {
			_, err := db.ExecContext(
				cleanupCtx,
				`DELETE FROM finance_fx_refresh_schedules WHERE schedule_id=$1`,
				scheduleID,
			)
			require.NoError(t, err)
		})
	}

	t.Run("publishes finance commands on prepared appdispatch without observed job rows", func(t *testing.T) {
		db, _, config := openPrepared(t)
		publisher := newPublisher(t, config, db)
		jobsStore, err := jobspkg.NewStore(db, config.DatabaseDSN, jobspkg.StoreOpts{TablePrefix: "sumweave_jobs_"})
		require.NoError(t, err)
		adapter := appdispatchSemanticCommandPublisher{publisher: publisher}
		now := time.Now()
		commands := []financepkg.SemanticCommand{
			{Topic: financepkg.TransactionCSVImportCommandTopic, Payload: []byte(`{"importId":"` + fake.UUID().V4() + `"}`)},
			{Topic: financepkg.AccountCSVImportCommandTopic, Payload: []byte(`{"importId":"` + fake.UUID().V4() + `"}`)},
			{Topic: financepkg.BankConnectionSyncCommandTopic, Payload: []byte(
				`{"connectionId":"` + fake.UUID().V4() + `","scheduledAt":"` + now.Format(time.RFC3339Nano) + `"}`,
			)},
			{Topic: financepkg.FXRatesRefreshCommandTopic, Payload: []byte(`{"provider":"provider-` + fake.UUID().V4() + `"}`)},
		}
		for _, command := range commands {
			reference, publishErr := adapter.PublishSemanticCommand(t.Context(), command)
			require.NoError(t, publishErr)
			assert.NotEmpty(t, reference.MessageID)
			assertPublishedMessage(t, db, config, reference.MessageID, command.Topic)
			_, getErr := jobsStore.Get(t.Context(), reference.MessageID)
			require.ErrorIs(t, getErr, jobspkg.ErrJobNotFound)
		}

		command := financepkg.SemanticCommand{
			Topic:          financepkg.TransactionCSVImportCommandTopic,
			Payload:        []byte(`{"importId":"` + fake.UUID().V4() + `"}`),
			IdempotencyKey: "finance.csv-import:" + fake.UUID().V4(),
		}
		first, err := adapter.PublishSemanticCommand(t.Context(), command)
		require.NoError(t, err)
		second, err := adapter.PublishSemanticCommand(t.Context(), command)
		require.NoError(t, err)
		assert.Equal(t, first, second)
		command.Payload = []byte(`{"importId":"` + fake.UUID().V4() + `"}`)
		_, err = adapter.PublishSemanticCommand(t.Context(), command)
		require.ErrorIs(t, err, appdispatch.ErrPublicationConflict)
	})

	t.Run("bank schedules commit scoped dispatch state and roll back a publication conflict", func(t *testing.T) {
		db, database, config := openPrepared(t)
		publisher := newPublisher(t, config, db)
		adapter := appdispatchSemanticCommandPublisher{publisher: publisher}
		store := persistence.NewBankConnectionScheduleStore(database)
		newService := func(now time.Time) *financepkg.BankConnectionScheduleService {
			return financepkg.NewBankConnectionScheduleService(
				store,
				financepkg.WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
				financepkg.WithBankConnectionScheduleServicePublisher(adapter),
			)
		}

		now := makeScheduleNow()
		dueAt := now.Add(-time.Hour)
		schedule := domain.BankConnectionSchedule{
			ConnectionID: fake.UUID().V4(), Interval: time.Hour, NextRunAt: &dueAt,
			Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		cleanupBankSchedule(t, db, schedule.ConnectionID)
		require.NoError(t, store.Save(t.Context(), schedule))
		first, err := newService(now).EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Positive(t, first)
		actual, err := store.Get(t.Context(), schedule.ConnectionID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.After(*schedule.NextRunAt))
		assert.NotEmpty(t, actual.LastJobID)
		assertPublishedMessage(t, db, config, actual.LastJobID, financepkg.BankConnectionSyncCommandTopic)
		second, err := newService(now).EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Zero(t, second)

		conflictNow := makeScheduleNow()
		conflictDueAt := conflictNow.Add(-time.Hour)
		conflict := domain.BankConnectionSchedule{
			ConnectionID: fake.UUID().V4(), Interval: time.Hour, NextRunAt: &conflictDueAt,
			Enabled: true, CreatedAt: conflictNow, UpdatedAt: conflictNow,
		}
		cleanupBankSchedule(t, db, conflict.ConnectionID)
		require.NoError(t, store.Save(t.Context(), conflict))
		persistedConflict, err := store.Get(t.Context(), conflict.ConnectionID)
		require.NoError(t, err)
		_, err = adapter.PublishSemanticCommand(t.Context(), financepkg.SemanticCommand{
			Topic:          "finance.conflict." + fake.UUID().V4(),
			Payload:        []byte(`{"value":"` + fake.UUID().V4() + `"}`),
			IdempotencyKey: fmt.Sprintf("finance.bank-connection-sync:%s:%s", conflict.ConnectionID, persistedConflict.NextRunAt.Format(time.RFC3339Nano)),
		})
		require.NoError(t, err)
		_, err = newService(conflictNow).EnqueueDue(t.Context())
		require.ErrorIs(t, err, appdispatch.ErrPublicationConflict)
		conflictActual, getErr := store.Get(t.Context(), conflict.ConnectionID)
		require.NoError(t, getErr)
		assert.True(t, conflictActual.NextRunAt.Equal(*persistedConflict.NextRunAt))
		assert.Nil(t, conflictActual.LastScheduledAt)
		assert.Empty(t, conflictActual.LastJobID)
	})

	t.Run("FX schedules commit scoped dispatch state and roll back a publication conflict", func(t *testing.T) {
		db, database, config := openPrepared(t)
		publisher := newPublisher(t, config, db)
		adapter := appdispatchSemanticCommandPublisher{publisher: publisher}
		store := persistence.NewFXRefreshScheduleStore(database)
		newService := func(now time.Time) *financepkg.FXRefreshScheduleService {
			return financepkg.NewFXRefreshScheduleService(
				store,
				financepkg.WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
				financepkg.WithFXRefreshScheduleServicePublisher(adapter),
			)
		}

		now := makeScheduleNow()
		dueAt := now.Add(-time.Hour)
		schedule := domain.FXRefreshSchedule{
			ScheduleID: fake.UUID().V4(), Provider: "provider-" + fake.UUID().V4(),
			Interval: time.Hour, NextRunAt: &dueAt, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		cleanupFXSchedule(t, db, schedule.ScheduleID)
		require.NoError(t, store.Save(t.Context(), schedule))
		first, err := newService(now).EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Positive(t, first)
		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.After(*schedule.NextRunAt))
		assert.NotEmpty(t, actual.LastJobID)
		assertPublishedMessage(t, db, config, actual.LastJobID, financepkg.FXRatesRefreshCommandTopic)
		second, err := newService(now).EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Zero(t, second)

		conflictNow := makeScheduleNow()
		conflictDueAt := conflictNow.Add(-time.Hour)
		conflict := domain.FXRefreshSchedule{
			ScheduleID: fake.UUID().V4(), Provider: "provider-" + fake.UUID().V4(),
			Interval: time.Hour, NextRunAt: &conflictDueAt, Enabled: true, CreatedAt: conflictNow, UpdatedAt: conflictNow,
		}
		cleanupFXSchedule(t, db, conflict.ScheduleID)
		require.NoError(t, store.Save(t.Context(), conflict))
		persistedConflict, err := store.Get(t.Context(), conflict.ScheduleID)
		require.NoError(t, err)
		_, err = adapter.PublishSemanticCommand(t.Context(), financepkg.SemanticCommand{
			Topic:          "finance.conflict." + fake.UUID().V4(),
			Payload:        []byte(`{"value":"` + fake.UUID().V4() + `"}`),
			IdempotencyKey: fmt.Sprintf("finance.fx-rates-refresh:%s:%s", conflict.ScheduleID, persistedConflict.NextRunAt.Format(time.RFC3339Nano)),
		})
		require.NoError(t, err)
		_, err = newService(conflictNow).EnqueueDue(t.Context())
		require.ErrorIs(t, err, appdispatch.ErrPublicationConflict)
		conflictActual, getErr := store.Get(t.Context(), conflict.ScheduleID)
		require.NoError(t, getErr)
		assert.True(t, conflictActual.NextRunAt.Equal(*persistedConflict.NextRunAt))
		assert.Nil(t, conflictActual.LastScheduledAt)
		assert.Empty(t, conflictActual.LastJobID)
	})
}
