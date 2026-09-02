package financeapp

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/appdispatch"
	apphttpclient "github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	financepkg "github.com/gemyago/sumweave/finance"
	"github.com/gemyago/sumweave/finance/credentials"
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
		logger := slog.New(slog.DiscardHandler)
		assert.Same(t, logger, resolveFinanceLogger(logger))
		baseURL := "https://" + fake.Internet().Domain()
		assert.Equal(t, baseURL, resolveMonobankBaseURL(" "+baseURL+" "))
		configuredCipher, err := makeFinanceCipher(fake.UUID().V4())
		require.NoError(t, err)
		_, err = configuredCipher.SealString(fake.UUID().V4())
		require.NoError(t, err)
		_, err = NewDatabase(nil, "sqlite:"+fake.UUID().V4(), logger)
		require.Error(t, err)
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

	t.Run("registers typed workload adapters against non-nil services", func(t *testing.T) {
		registry := jobspkg.NewRegistry()
		fxService := newMockfxRefreshJobService(t)
		csvService := newMockcsvImportJobService(t)
		bankService := newMockbankSyncJobService(t)
		require.NoError(t, registerFinanceJobHandlers(registry, fxService, csvService, bankService))
		assert.Len(t, registry.Handlers(), 4)
		require.NoError(t, registerFinanceJobHandlers(registry, fxService, csvService, bankService))
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

	t.Run("publishes semantic commands through the appdispatch adapter", func(t *testing.T) {
		publisher := newMockappdispatchPublisher(t)
		command := financepkg.SemanticCommand{
			Topic:          "finance.command." + fake.UUID().V4(),
			Payload:        []byte(fake.UUID().V4()),
			IdempotencyKey: fake.UUID().V4(),
		}
		publisher.EXPECT().PublishRequest(
			mock.Anything,
			appdispatch.PublicationRequest{
				Topic: command.Topic, Payload: command.Payload, IdempotencyKey: command.IdempotencyKey,
			},
		).Return(appdispatch.PublicationReference{MessageID: fake.UUID().V4()}, nil).Once()
		adapter := appdispatchSemanticCommandPublisher{publisher: publisher}
		reference, err := adapter.PublishSemanticCommand(t.Context(), command)
		require.NoError(t, err)
		assert.NotEmpty(t, reference.MessageID)

		tx := &sql.Tx{}
		publisher.EXPECT().PublishRequestInTx(
			mock.Anything,
			tx,
			appdispatch.PublicationRequest{
				Topic: command.Topic, Payload: command.Payload, IdempotencyKey: command.IdempotencyKey,
			},
		).Return(appdispatch.PublicationReference{MessageID: fake.UUID().V4()}, nil).Once()
		reference, err = adapter.PublishScheduledSemanticCommand(t.Context(), tx, command)
		require.NoError(t, err)
		assert.NotEmpty(t, reference.MessageID)

		publishErr := errors.New(fake.Lorem().Sentence(3))
		publisher.EXPECT().PublishRequest(mock.Anything, mock.Anything).
			Return(appdispatch.PublicationReference{}, publishErr).Once()
		_, err = adapter.PublishSemanticCommand(t.Context(), command)
		require.ErrorIs(t, err, publishErr)
		publisher.EXPECT().PublishRequestInTx(mock.Anything, tx, mock.Anything).
			Return(appdispatch.PublicationReference{}, publishErr).Once()
		_, err = adapter.PublishScheduledSemanticCommand(t.Context(), tx, command)
		require.ErrorIs(t, err, publishErr)
	})
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
