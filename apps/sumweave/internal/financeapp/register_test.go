package financeapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
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
			dsn := fmt.Sprintf(
				"file:finance-schedule-%s?mode=memory&cache=shared",
				fake.UUID().V4(),
			)
			db, err := sqlconn.Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			store, err := jobspkg.NewStore(db, dsn, jobspkg.StoreOpts{TablePrefix: "finance_"})
			require.NoError(t, err)
			require.NoError(t, store.AutoMigrate())
			registry := jobspkg.NewRegistry()
			require.NoError(t, registerFinanceJobHandlers(registry, nil, nil, nil))
			_, err = registry.Handler(financepkg.FXRatesRefreshCommandTopic)
			require.ErrorIs(t, err, jobspkg.ErrHandlerNotRegistered)
		},
	)
}

func TestFinanceJobRegistrationAdapters(t *testing.T) {
	fake := faker.New()
	makeStore := func(t *testing.T) (*jobspkg.Store, *sql.DB, string) {
		t.Helper()
		dsn := fmt.Sprintf("file:finance-job-adapter-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := jobspkg.NewStore(db, dsn, jobspkg.StoreOpts{TablePrefix: "finance_adapter_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store, db, dsn
	}

	t.Run("publishes each finance workload directly without a job row", func(t *testing.T) {
		store, db, dsn := makeStore(t)
		config := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "finance_adapter_"}
		require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		adapter := appdispatchSemanticCommandPublisher{publisher: publisher}
		now := time.Now()
		commands := []financepkg.SemanticCommand{
			{
				Topic:   financepkg.TransactionCSVImportCommandTopic,
				Payload: []byte(`{"importId":"` + fake.UUID().V4() + `"}`),
			},
			{
				Topic:   financepkg.AccountCSVImportCommandTopic,
				Payload: []byte(`{"importId":"` + fake.UUID().V4() + `"}`),
			},
			{
				Topic: financepkg.BankConnectionSyncCommandTopic,
				Payload: []byte(
					`{"connectionId":"` + fake.UUID().
						V4() +
						`","scheduledAt":"` + now.Format(
						time.RFC3339Nano,
					) + `"}`,
				),
			},
			{
				Topic:   financepkg.FXRatesRefreshCommandTopic,
				Payload: []byte(`{"provider":"` + fake.Letter() + `"}`),
			},
		}
		for _, command := range commands {
			reference, publishErr := adapter.PublishSemanticCommand(t.Context(), command)
			require.NoError(t, publishErr)
			assert.NotEmpty(t, reference.MessageID)
		}
		jobs, err := store.List(t.Context(), jobspkg.ListParams{})
		require.NoError(t, err)
		assert.Empty(t, jobs.Items)

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

func TestBankScheduleDispatch(t *testing.T) {
	fake := faker.New()

	makeScheduleService := func(t *testing.T, migrateDispatch bool) (
		*financepkg.BankConnectionScheduleService,
		*persistence.BankConnectionScheduleStore,
		*sql.DB,
		appdispatch.Config,
	) {
		t.Helper()
		dsn := fmt.Sprintf("file:bank-schedule-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		config := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "bank_schedule_"}
		if migrateDispatch {
			require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
		}
		database, err := persistence.NewDatabase(db, dsn)
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		store := persistence.NewBankConnectionScheduleStore(database)
		now := time.Now()
		return financepkg.NewBankConnectionScheduleService(
			store,
			financepkg.WithBankConnectionScheduleServiceNow(func() time.Time { return now }),
			financepkg.WithBankConnectionScheduleServicePublisher(
				appdispatchSemanticCommandPublisher{publisher: publisher},
			),
		), store, db, config
	}

	makeSchedule := func(now time.Time) domain.BankConnectionSchedule {
		dueAt := now.Add(-time.Hour)
		return domain.BankConnectionSchedule{
			ConnectionID: fake.UUID().V4(), Interval: time.Hour, NextRunAt: &dueAt,
			Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("commits occurrence advance and dispatch reference together without rerun duplication", func(t *testing.T) {
		service, store, db, config := makeScheduleService(t, true)
		now := time.Now()
		schedule := makeSchedule(now)
		require.NoError(t, store.Save(t.Context(), schedule))

		first, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)
		second, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)

		assert.Equal(t, 1, first)
		assert.Zero(t, second)
		actual, err := store.Get(t.Context(), schedule.ConnectionID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.After(*schedule.NextRunAt))
		assert.NotEmpty(t, actual.LastJobID)
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM "`+config.MessagesTable()+`" WHERE topic = ?`,
			financepkg.BankConnectionSyncCommandTopic,
		).Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("rolls back bank occurrence state when dispatch cannot be persisted", func(t *testing.T) {
		service, store, _, _ := makeScheduleService(t, false)
		now := time.Now()
		schedule := makeSchedule(now)
		require.NoError(t, store.Save(t.Context(), schedule))

		_, err := service.EnqueueDue(t.Context())

		require.Error(t, err)
		actual, getErr := store.Get(t.Context(), schedule.ConnectionID)
		require.NoError(t, getErr)
		assert.True(t, actual.NextRunAt.Equal(*schedule.NextRunAt))
		assert.Nil(t, actual.LastScheduledAt)
		assert.Empty(t, actual.LastJobID)
	})
}

func TestFXScheduleDispatch(t *testing.T) {
	fake := faker.New()

	makeScheduleService := func(t *testing.T, migrateDispatch bool) (
		*financepkg.FXRefreshScheduleService,
		*persistence.FXRefreshScheduleStore,
		*sql.DB,
		appdispatch.Config,
	) {
		t.Helper()
		dsn := fmt.Sprintf("file:fx-schedule-%s?mode=memory&cache=shared", fake.UUID().V4())
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		config := appdispatch.Config{DatabaseDSN: dsn, TablePrefix: "fx_schedule_"}
		if migrateDispatch {
			require.NoError(t, appdispatch.AutoMigrate(t.Context(), config, db))
		}
		database, err := persistence.NewDatabase(db, dsn)
		require.NoError(t, err)
		require.NoError(t, persistence.NewMigrator(database).Migrate(t.Context()))
		publisher, err := appdispatch.NewPublisher(config, db, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, publisher.Close()) })
		now := time.Now()
		return financepkg.NewFXRefreshScheduleService(
			persistence.NewFXRefreshScheduleStore(database),
			financepkg.WithFXRefreshScheduleServiceNow(func() time.Time { return now }),
			financepkg.WithFXRefreshScheduleServicePublisher(appdispatchSemanticCommandPublisher{publisher: publisher}),
		), persistence.NewFXRefreshScheduleStore(database), db, config
	}

	makeSchedule := func(now time.Time) domain.FXRefreshSchedule {
		dueAt := now.Add(-time.Hour)
		return domain.FXRefreshSchedule{
			ScheduleID: fake.UUID().V4(), Provider: "provider-" + fake.Letter(),
			Interval: time.Hour, NextRunAt: &dueAt, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
	}

	t.Run("commits the FX occurrence and dispatch reference together without rerun duplication", func(t *testing.T) {
		service, store, db, config := makeScheduleService(t, true)
		now := time.Now()
		schedule := makeSchedule(now)
		require.NoError(t, store.Save(t.Context(), schedule))

		first, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)
		second, err := service.EnqueueDue(t.Context())
		require.NoError(t, err)

		assert.Equal(t, 1, first)
		assert.Zero(t, second)
		actual, err := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, err)
		assert.True(t, actual.NextRunAt.After(*schedule.NextRunAt))
		assert.NotEmpty(t, actual.LastJobID)
		var count int
		require.NoError(t, db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM "`+config.MessagesTable()+`" WHERE topic = ?`,
			financepkg.FXRatesRefreshCommandTopic,
		).Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("rolls back the FX occurrence when dispatch cannot be persisted", func(t *testing.T) {
		service, store, _, _ := makeScheduleService(t, false)
		now := time.Now()
		schedule := makeSchedule(now)
		require.NoError(t, store.Save(t.Context(), schedule))

		_, err := service.EnqueueDue(t.Context())

		require.Error(t, err)
		actual, getErr := store.Get(t.Context(), schedule.ScheduleID)
		require.NoError(t, getErr)
		assert.True(t, actual.NextRunAt.Equal(*schedule.NextRunAt))
		assert.Nil(t, actual.LastScheduledAt)
		assert.Empty(t, actual.LastJobID)
	})
}
