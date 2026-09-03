//go:build postgres_test

package financeapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFinanceModuleHelpers(t *testing.T) {
	fake := faker.New()
	makeFactory := func() *apphttpclient.ClientFactory {
		return apphttpclient.NewClientFactory(apphttpclient.ClientFactoryDeps{
			RootLogger: slog.New(slog.DiscardHandler),
		})
	}

	t.Run("uses native finance construction defaults", func(t *testing.T) {
		cipher, err := makeFinanceCipher("")
		require.NoError(t, err)
		_, err = cipher.SealString(fake.UUID().V4())
		require.Error(t, err)
		_, err = cipher.OpenString(credentials.Envelope{})
		require.Error(t, err)
		client, err := newMonobankHTTPClient(makeFactory(), time.Second)
		require.NoError(t, err)
		assert.Equal(t, 62*time.Second, client.Timeout)
		assert.Equal(t, "https://api.monobank.ua", resolveMonobankBaseURL(" "))
		_, err = newFinanceHTTPClient(nil)
		require.Error(t, err)
		_, err = newMonobankHTTPClient(nil, time.Second)
		require.Error(t, err)
		_, err = newMonobankHTTPClient(makeFactory(), 0)
		require.Error(t, err)
		assert.NotNil(t, resolveFinanceLogger(nil))
	})

	t.Run(
		"keeps FX generic scheduling while registering only error-returning handlers",
		func(t *testing.T) {
			registry := jobspkg.NewRegistry()
			require.NoError(t, registerFinanceJobHandlers(registry, nil, nil, nil))
			_, err := registry.Handler(financepkg.FXRatesRefreshCommandTopic)
			require.ErrorIs(t, err, jobspkg.ErrHandlerNotRegistered)
		},
	)
}

func TestFinanceJobRegistrationAdapters(t *testing.T) {
	fake := faker.New()
	t.Run("registers the four workload names once and maps bank metadata", func(t *testing.T) {
		registry := jobspkg.NewRegistry()
		require.NoError(t, registerFinanceJobHandlers(registry, nil, nil, nil))
		require.NoError(t, registerFinanceJobHandlers(nil, nil, nil, nil))
		job := jobspkg.Job{ID: fake.UUID().V4()}
		input := financepkg.BankConnectionSyncCommand{
			ConnectionID: fake.UUID().V4(),
			Reason:       fake.Letter(),
		}
		actual := makeRunBankConnectionSyncParams(job, input)
		assert.Equal(
			t,
			financepkg.RunBankConnectionSyncParams{
				ConnectionID: input.ConnectionID,
				JobID:        job.ID,
				Reason:       input.Reason,
			},
			actual,
		)
		existingTopic := "existing." + fake.UUID().V4()
		require.NoError(
			t,
			jobspkg.RegisterTypedHandler(
				registry,
				jobspkg.TypedHandlerSpec[any]{
					JobType:  "existing",
					Topic:    existingTopic,
					Metadata: func(any) (jobspkg.JobMetadata, error) { return jobspkg.JobMetadata{JobType: "existing"}, nil },
					Run:      func(context.Context, jobspkg.Job, any) error { return nil },
				},
			),
		)
		require.NoError(
			t,
			registerFinanceJobHandler(
				registry,
				existingTopic,
				func() error { return assert.AnError },
			),
		)
	})

	t.Run("rejects incomplete native module dependencies before registration", func(t *testing.T) {
		_, err := NewModule(ModuleDeps{})
		require.Error(t, err)
		_, err = NewModule(ModuleDeps{CommandPublisher: &appdispatch.Publisher{}})
		require.Error(t, err)
		_, err = NewModule(
			ModuleDeps{
				CommandPublisher: &appdispatch.Publisher{},
				HTTPClientFactory: apphttpclient.NewClientFactory(
					apphttpclient.ClientFactoryDeps{RootLogger: slog.New(slog.DiscardHandler)},
				),
			},
		)
		require.Error(t, err)
		_, err = NewModule(ModuleDeps{
			CommandPublisher: &appdispatch.Publisher{},
			HTTPClientFactory: apphttpclient.NewClientFactory(
				apphttpclient.ClientFactoryDeps{RootLogger: slog.New(slog.DiscardHandler)},
			),
			MonobankRetryAfterFallbackDelay: time.Second,
		})
		require.Error(t, err)
	})

	t.Run(
		"maps only safe command metadata and leaves unclassified failures retryable",
		func(t *testing.T) {
			metadata, err := jobMetadata(
				jobspkg.JobType(financepkg.FXRefreshJobType),
				financepkg.CommandRequester{
					UserID: fake.UUID().V4(),
					Source: financepkg.CommandRequesterSourceSystem,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, jobspkg.JobType(financepkg.FXRefreshJobType), metadata.JobType)
			_, err = jobMetadata("type", financepkg.CommandRequester{})
			require.Error(t, err)
			failure := handledFinanceFailure(assert.AnError)
			require.ErrorIs(t, failure, assert.AnError)
			_, classified := appdispatch.BusinessFailureFrom(failure)
			assert.False(t, classified)
			assert.NoError(t, handledFinanceFailure(nil))
		},
	)

}

func TestFinanceObservedHandlerFailureClassification(t *testing.T) {
	fake := faker.New()
	terminalFailure := func() error {
		return financepkg.NewTerminalFailure(
			errors.New("provider rejected "+fake.UUID().V4()),
			"safe_code_"+fake.Letter(),
			"safe summary "+fake.Letter(),
			"safe details "+fake.Letter(),
		)
	}
	assertClassification := func(t *testing.T, err error, terminal bool) {
		t.Helper()
		failure, classified := appdispatch.BusinessFailureFrom(err)
		assert.Equal(t, terminal, classified)
		if terminal {
			expected, ok := financepkg.TerminalFailureFrom(err)
			require.True(t, ok)
			assert.Equal(t, expected.Code, failure.Code)
			assert.Equal(t, expected.Summary, failure.Summary)
			assert.Equal(t, expected.Details, failure.Details)
		}
	}

	t.Run("transaction CSV terminal and infrastructure failures", func(t *testing.T) {
		job := jobspkg.Job{ID: fake.UUID().V4()}
		input := financepkg.CSVImportCommand{ImportID: fake.UUID().V4()}
		for name, failure := range map[string]error{
			"terminal":       terminalFailure(),
			"infrastructure": errors.New("database unavailable " + fake.UUID().V4()),
		} {
			t.Run(name, func(t *testing.T) {
				service := newMockcsvImportJobService(t)
				service.EXPECT().RunCSVImportJob(
					mock.Anything,
					financepkg.RunCSVImportJobParams{ImportID: input.ImportID, JobID: job.ID},
				).Return(financepkg.CSVImportRunResult{}, failure)
				assertClassification(t, runCSVImportJob(t.Context(), service, job, input), name == "terminal")
			})
		}
	})

	t.Run("account CSV terminal and infrastructure failures", func(t *testing.T) {
		job := jobspkg.Job{ID: fake.UUID().V4()}
		input := financepkg.CSVImportCommand{ImportID: fake.UUID().V4()}
		for name, failure := range map[string]error{
			"terminal":       terminalFailure(),
			"infrastructure": errors.New("database unavailable " + fake.UUID().V4()),
		} {
			t.Run(name, func(t *testing.T) {
				service := newMockcsvImportJobService(t)
				service.EXPECT().RunCSVImportJob(
					mock.Anything,
					financepkg.RunCSVImportJobParams{ImportID: input.ImportID, JobID: job.ID},
				).Return(financepkg.CSVImportRunResult{}, failure)
				assertClassification(t, runCSVImportJob(t.Context(), service, job, input), name == "terminal")
			})
		}
	})

	t.Run("bank sync terminal and infrastructure failures", func(t *testing.T) {
		job := jobspkg.Job{ID: fake.UUID().V4()}
		input := financepkg.BankConnectionSyncCommand{ConnectionID: fake.UUID().V4()}
		for name, failure := range map[string]error{
			"terminal":       terminalFailure(),
			"infrastructure": errors.New("database unavailable " + fake.UUID().V4()),
		} {
			t.Run(name, func(t *testing.T) {
				service := newMockbankSyncJobService(t)
				service.EXPECT().RunBankConnectionSync(
					mock.Anything,
					makeRunBankConnectionSyncParams(job, input),
				).Return(financepkg.BankConnectionSyncResult{}, failure)
				assertClassification(t, runBankSyncJob(t.Context(), service, job, input), name == "terminal")
			})
		}
	})

	t.Run("FX refresh terminal and infrastructure failures", func(t *testing.T) {
		input := financepkg.FXRatesRefreshCommand{Provider: "provider-" + fake.Letter()}
		for name, failure := range map[string]error{
			"terminal":       terminalFailure(),
			"infrastructure": errors.New("provider unavailable " + fake.UUID().V4()),
		} {
			t.Run(name, func(t *testing.T) {
				service := newMockfxRefreshJobService(t)
				service.EXPECT().RefreshRequiredFXRates(
					mock.Anything,
					financepkg.RefreshFXRatesParams{Provider: input.Provider},
				).Return(financepkg.RefreshFXRatesResult{}, failure)
				assertClassification(t, runFXRefreshJob(t.Context(), service, input), name == "terminal")
			})
		}
	})
}

func TestFinanceRegistrationPostgres(t *testing.T) {
	fake := faker.New()

	openPrepared := func(t *testing.T) (*sql.DB, *persistence.Database, appdispatch.Config) {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sql.Open("pgx", dsn)
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
