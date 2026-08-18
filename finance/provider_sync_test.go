package finance

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedBankSyncJobEnqueuer struct {
	request *BankConnectionSyncJobRequest
	jobRef  BankConnectionSyncJobRef
}

type capturedBankSyncScheduleWriter struct {
	schedules []BankConnectionSyncSchedule
	err       error
}

func (w *capturedBankSyncScheduleWriter) UpsertBankConnectionSyncSchedule(
	_ context.Context,
	schedule BankConnectionSyncSchedule,
) error {
	w.schedules = append(w.schedules, schedule)
	return w.err
}

func (e *capturedBankSyncJobEnqueuer) EnqueueBankConnectionSync(
	_ context.Context,
	request BankConnectionSyncJobRequest,
) (BankConnectionSyncJobRef, error) {
	e.request = &request
	if e.jobRef.JobType == "" {
		e.jobRef = BankConnectionSyncJobRef{ID: "job-sync-1", JobType: request.JobType}
	}
	return e.jobRef, nil
}

type stubBankProvider struct {
	name         string
	startResult  ProviderLinkStart
	finishResult ProviderLinkResult
	finishParams []ProviderFinishLinkParams
	linkResult   ProviderTokenLinkResult
	startErr     error
	finishErr    error
	linkErr      error
	syncResults  []ProviderSyncResult
	syncErr      error
	syncCalls    int
	syncParams   []ProviderSyncParams
	startCalls   int
	finishCalls  int
	linkedTokens []string
	secrets      []string
	mu           sync.Mutex
}

func (p *stubBankProvider) Name() string { return p.name }

func (p *stubBankProvider) StartLink(
	context.Context,
	ProviderStartLinkParams,
) (ProviderLinkStart, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startCalls++
	if p.startErr != nil {
		return ProviderLinkStart{}, p.startErr
	}
	if p.startResult.AuthorizationURL == "" && p.startResult.State == "" {
		return ProviderLinkStart{}, errors.New("start link unsupported")
	}
	return p.startResult, nil
}

func (p *stubBankProvider) FinishLink(
	_ context.Context,
	params ProviderFinishLinkParams,
) (ProviderLinkResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finishCalls++
	p.finishParams = append(p.finishParams, params)
	if p.finishErr != nil {
		return ProviderLinkResult{}, p.finishErr
	}
	if p.finishResult.Secret == "" && p.finishResult.ProviderReference == "" {
		return ProviderLinkResult{}, errors.New("finish link unsupported")
	}
	return p.finishResult, nil
}

func (p *stubBankProvider) LinkToken(
	_ context.Context,
	params ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.linkErr != nil {
		return ProviderTokenLinkResult{}, p.linkErr
	}
	p.linkedTokens = append(p.linkedTokens, params.Token)
	return p.linkResult, nil
}

func (p *stubBankProvider) Sync(
	_ context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.syncErr != nil {
		return ProviderSyncResult{}, p.syncErr
	}
	p.syncParams = append(p.syncParams, params)
	p.secrets = append(p.secrets, params.Secret)
	if p.syncCalls >= len(p.syncResults) {
		return ProviderSyncResult{}, errors.New("no stub sync result")
	}
	result := p.syncResults[p.syncCalls]
	p.syncCalls++
	return result, nil
}

func TestFinanceProviderSync(t *testing.T) {
	makeStore := func(t *testing.T) *persistence.Store {
		t.Helper()

		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		return store
	}

	makeCipher := func(t *testing.T) *credentials.AESGCMCipher {
		t.Helper()
		cipher, err := credentials.NewAESGCMCipher(
			[]byte("0123456789abcdef0123456789abcdef"),
			"test-key",
		)
		require.NoError(t, err)
		return cipher
	}

	makeTenant := func(t *testing.T, service *Service, actorUserID string) domain.Tenant {
		t.Helper()

		fake := faker.New()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		return tenant
	}

	saveLinkedConnectionForTest := func(
		t *testing.T,
		service *Service,
		tenantID string,
		providerName string,
		connectorID domain.ProviderConnectorID,
		result ProviderLinkResult,
	) domain.BankConnection {
		t.Helper()

		secretID, err := service.encryptAndSaveConnectionSecret(
			t.Context(),
			providerName,
			result.ProviderReference,
			result.Secret,
		)
		require.NoError(t, err)

		syncStore, err := service.bankSyncStore()
		require.NoError(t, err)

		now := service.now().UTC()
		connection, err := syncStore.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                service.newID(),
			TenantID:          tenantID,
			Provider:          providerName,
			ConnectorID:       connectorID,
			DisplayName:       strings.TrimSpace(result.DisplayName),
			ProviderReference: strings.TrimSpace(result.ProviderReference),
			SecretID:          secretID,
			State:             result.State,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		require.NoError(t, err)

		return connection
	}

	t.Run("covers bank-synced foreign currency reporting with historical FX coverage", func(t *testing.T) {
		fake := faker.New()
		effectiveAt := time.Date(2026, time.June, 9, 10, 0, 0, 0, time.UTC)

		type foreignCurrencySyncFixture struct {
			service    *Service
			store      *persistence.Store
			tenant     domain.Tenant
			ownerID    string
			connection domain.BankConnection
		}
		makeForeignCurrencySyncFixture := func(
			t *testing.T,
			rates []domain.FXRate,
		) foreignCurrencySyncFixture {
			t.Helper()

			store := makeStore(t)
			providerAccountID := "account-" + fake.UUID().V4()
			providerTransactionID := "transaction-" + fake.UUID().V4()
			provider := &stubBankProvider{
				name: "provider-" + fake.Lorem().Word(),
				syncResults: []ProviderSyncResult{
					{
						SyncKey: "sync-" + fake.UUID().V4(),
						Accounts: []ProviderNormalizedAccount{{
							ProviderAccountID: providerAccountID,
							Name:              "account-" + fake.Lorem().Word(),
							Currency:          "EUR",
						}},
						Transactions: []ProviderNormalizedTransaction{{
							ProviderAccountID:     providerAccountID,
							ProviderTransactionID: providerTransactionID,
							Status:                domain.TransactionStatusBooked,
							AmountMinor:           -100_00,
							Currency:              "EUR",
							Description:           "expense-" + fake.Lorem().Word(),
							EffectiveAt:           effectiveAt,
							Fingerprint:           "fingerprint-" + fake.UUID().V4(),
						}},
					},
					{
						SyncKey: "sync-" + fake.UUID().V4(),
						Transactions: []ProviderNormalizedTransaction{{
							ProviderAccountID:     providerAccountID,
							ProviderTransactionID: providerTransactionID,
							Status:                domain.TransactionStatusBooked,
							AmountMinor:           -100_00,
							Currency:              "EUR",
							Description:           "expense-" + fake.Lorem().Word(),
							EffectiveAt:           effectiveAt,
							Fingerprint:           "fingerprint-" + fake.UUID().V4(),
						}},
					},
				},
			}
			fxProvider := NewStaticFXProvider(FXProviderFrankfurter, rates)
			service := NewService(
				store,
				WithNow(func() time.Time { return effectiveAt.Add(24 * time.Hour) }),
				WithConnectionSecretCipher(makeCipher(t)),
				WithBankProviders(provider),
				WithFXProviders(fxProvider),
				WithDefaultFXProvider(fxProvider.Name()),
			)
			ownerID := "owner-" + fake.UUID().V4()
			tenant := makeTenant(t, service, ownerID)
			connection := saveLinkedConnectionForTest(
				t,
				service,
				tenant.ID,
				provider.name,
				domain.ProviderConnectorIDSynthetic,
				ProviderLinkResult{
					DisplayName:       "connection-" + fake.Lorem().Word(),
					ProviderReference: "reference-" + fake.UUID().V4(),
					Secret:            "secret-" + fake.UUID().V4(),
					State:             domain.BankConnectionStateActive,
				},
			)
			return foreignCurrencySyncFixture{
				service: service, store: store, tenant: tenant, ownerID: ownerID, connection: connection,
			}
		}

		t.Run("does not backfill or persist historical FX after bank sync", func(t *testing.T) {
			fixture := makeForeignCurrencySyncFixture(t, []domain.FXRate{{
				Provider:      FXProviderFrankfurter,
				BaseCurrency:  "EUR",
				QuoteCurrency: "PLN",
				RateDate:      effectiveAt.AddDate(0, 0, -1),
				Rate:          4.25,
			}})

			first, err := fixture.service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
				ConnectionID: fixture.connection.ID,
				JobID:        "job-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			assert.Equal(t, 1, first.ImportedTransactions)
			dashboard, err := fixture.service.GetDashboard(t.Context(), DashboardParams{
				ActorUserID: fixture.ownerID,
				TenantID:    fixture.tenant.ID,
			})
			require.NoError(t, err)
			assert.Zero(t, dashboard.Settled.ExpenseMinor)
			assert.False(t, dashboard.Settled.Complete)
			require.Len(t, dashboard.MissingFX, 2)
			assert.Equal(t, []DashboardCurrencyTotal{{
				Currency: "EUR", ExpenseMinor: 100_00, NetMinor: -100_00,
			}}, dashboard.NativeSettledTotals)

			second, err := fixture.service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
				ConnectionID: fixture.connection.ID,
				JobID:        "job-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			assert.Equal(t, 1, second.UpdatedTransactions)
			storedRates, err := fixture.store.ListFXRates(t.Context(), persistence.ListFXRatesParams{
				Provider: FXProviderFrankfurter, BaseCurrency: "EUR", QuoteCurrency: "PLN",
			})
			require.NoError(t, err)
			assert.Empty(t, storedRates)
		})

		t.Run("keeps the dashboard incomplete when no historical rate is available", func(t *testing.T) {
			fixture := makeForeignCurrencySyncFixture(t, nil)
			_, err := fixture.service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
				ConnectionID: fixture.connection.ID,
				JobID:        "job-" + fake.UUID().V4(),
			})
			require.NoError(t, err)
			dashboard, err := fixture.service.GetDashboard(t.Context(), DashboardParams{
				ActorUserID: fixture.ownerID,
				TenantID:    fixture.tenant.ID,
			})
			require.NoError(t, err)
			assert.Zero(t, dashboard.Settled.ExpenseMinor)
			assert.False(t, dashboard.Settled.Complete)
			require.Len(t, dashboard.MissingFX, 2)
			assert.Equal(t, DashboardMissingFXSourceTransaction, dashboard.MissingFX[0].Source)
			assert.Equal(t, []DashboardCurrencyTotal{{
				Currency: "EUR", ExpenseMinor: 100_00, NetMinor: -100_00,
			}}, dashboard.NativeSettledTotals)
		})
	})

	t.Run(
		"manages bank connection lifecycle sync persistence schedules and redacted logging",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			cipher := makeCipher(t)
			var logs bytes.Buffer
			providerToken := "token-" + fake.UUID().V4()
			providerSecret := "secret-" + fake.UUID().V4()
			providerAccountID := "provider-account-" + fake.UUID().V4()
			pendingFingerprint := "fp-" + fake.UUID().V4()
			provider := &stubBankProvider{
				name: "monobank",
				linkResult: ProviderTokenLinkResult{
					DisplayName:       "Connection " + fake.Company().Name(),
					ProviderReference: "provider-ref-" + fake.UUID().V4(),
					Secret:            providerSecret,
					State:             domain.BankConnectionStateActive,
				},
				syncResults: []ProviderSyncResult{
					{
						SyncKey: "sync-" + fake.UUID().V4(),
						Accounts: []ProviderNormalizedAccount{{
							ProviderAccountID: providerAccountID,
							Name:              "Main " + fake.Lorem().Word(),
							Currency:          "PLN",
							IBAN:              "PL61109010140000071219812874",
							AvailableBalanceMinor: func() *int64 {
								value := int64(401_23)
								return &value
							}(),
							CurrentBalanceMinor: func() *int64 {
								value := int64(501_23)
								return &value
							}(),
						}},
						Transactions: []ProviderNormalizedTransaction{{
							ProviderAccountID: providerAccountID,
							Status:            domain.TransactionStatusPending,
							AmountMinor:       -1200,
							Currency:          "PLN",
							Description:       "Pending " + fake.Lorem().Word(),
							EffectiveAt: time.Date(
								2026,
								time.June,
								20,
								10,
								0,
								0,
								0,
								time.UTC,
							),
							Fingerprint: pendingFingerprint,
							ProviderOriginal: &domain.ProviderTransactionOriginal{
								AmountMinor: -1200,
								Currency:    "PLN",
								Description: "Pending original " + fake.Lorem().Word(),
							},
						}},
						ScheduledRun: &ProviderScheduledRunMetadata{
							ScheduledAt: time.Date(2026, time.June, 20, 10, 1, 0, 0, time.UTC),
							NextRunAt: func() *time.Time {
								value := time.Date(2026, time.June, 20, 11, 1, 0, 0, time.UTC)
								return &value
							}(),
						},
					},
					{
						SyncKey: "sync-" + fake.UUID().V4(),
						Transactions: []ProviderNormalizedTransaction{{
							ProviderAccountID:     providerAccountID,
							ProviderTransactionID: "provider-txn-" + fake.UUID().V4(),
							Status:                domain.TransactionStatusBooked,
							AmountMinor:           -1200,
							Currency:              "PLN",
							Description:           "Booked " + fake.Lorem().Word(),
							EffectiveAt: time.Date(
								2026,
								time.June,
								21,
								10,
								0,
								0,
								0,
								time.UTC,
							),
							Fingerprint: pendingFingerprint,
							ProviderOriginal: &domain.ProviderTransactionOriginal{
								AmountMinor: -1200,
								Currency:    "PLN",
								Description: "Booked original " + fake.Lorem().Word(),
							},
						}},
						Reauth: &domain.ConnectionReauthMetadata{
							RequiredAt: func() *time.Time {
								value := time.Date(2026, time.June, 22, 10, 0, 0, 0, time.UTC)
								return &value
							}(),
							Reason: "sca_expired",
						},
					},
				},
			}
			enqueuer := &capturedBankSyncJobEnqueuer{}
			scheduleWriter := &capturedBankSyncScheduleWriter{}
			service := NewService(
				store,
				WithConnectionSecretCipher(cipher),
				WithBankProviders(provider),
				WithBankSyncJobEnqueuer(enqueuer),
				WithBankConnectionSyncScheduleWriter(scheduleWriter),
				WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
			)

			ownerUserID := "user-owner-" + fake.UUID().V4()
			tenant := makeTenant(t, service, ownerUserID)

			connection := saveLinkedConnectionForTest(
				t,
				service,
				tenant.ID,
				provider.name,
				domain.ProviderConnectorIDMonobank,
				ProviderLinkResult{
					DisplayName:       provider.linkResult.DisplayName,
					ProviderReference: provider.linkResult.ProviderReference,
					Secret:            providerSecret,
					State:             provider.linkResult.State,
				},
			)
			assert.Equal(t, domain.BankConnectionStateActive, connection.State)
			assert.Equal(t, domain.ProviderConnectorIDMonobank, connection.ConnectorID)
			assert.Equal(t, provider.linkResult.ProviderReference, connection.ProviderReference)

			sqlDB, err := store.DB().DB()
			require.NoError(t, err)
			var ciphertext string
			err = sqlDB.QueryRowContext(
				t.Context(),
				"SELECT ciphertext FROM finance_connection_secrets WHERE id = ?",
				connection.SecretID,
			).Scan(&ciphertext)
			require.NoError(t, err)
			assert.NotContains(t, ciphertext, providerToken)
			assert.NotContains(t, ciphertext, providerSecret)

			schedule, err := service.UpsertBankConnectionSchedule(
				t.Context(),
				UpsertBankConnectionScheduleParams{
					ActorUserID:  ownerUserID,
					TenantID:     tenant.ID,
					ConnectionID: connection.ID,
					Interval:     2 * time.Hour,
					NextRunAt:    time.Date(2026, time.June, 20, 10, 30, 0, 0, time.UTC),
				},
			)
			require.NoError(t, err)
			assert.True(t, schedule.Enabled)
			require.Len(t, scheduleWriter.schedules, 1)
			assert.True(t, scheduleWriter.schedules[0].Enabled)

			paused, err := service.PauseBankConnectionSchedule(
				t.Context(),
				PauseBankConnectionScheduleParams{
					ActorUserID:  ownerUserID,
					TenantID:     tenant.ID,
					ConnectionID: connection.ID,
				},
			)
			require.NoError(t, err)
			assert.False(t, paused.Enabled)
			assert.Nil(t, paused.NextRunAt)
			require.Len(t, scheduleWriter.schedules, 2)
			assert.False(t, scheduleWriter.schedules[1].Enabled)
			assert.Nil(t, scheduleWriter.schedules[1].NextRunAt)

			resumed, err := service.ResumeBankConnectionSchedule(
				t.Context(),
				ResumeBankConnectionScheduleParams{
					ActorUserID:  ownerUserID,
					TenantID:     tenant.ID,
					ConnectionID: connection.ID,
					NextRunAt:    time.Date(2026, time.June, 20, 11, 0, 0, 0, time.UTC),
				},
			)
			require.NoError(t, err)
			assert.True(t, resumed.Enabled)
			require.Len(t, scheduleWriter.schedules, 3)
			assert.True(t, scheduleWriter.schedules[2].Enabled)

			scheduledAt := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
			nextScheduledAt := scheduledAt.Add(2 * time.Hour)
			scheduledJobID := "job-scheduled-" + fake.UUID().V4()
			_, err = service.RecordBankConnectionSyncScheduled(
				t.Context(),
				RecordBankConnectionSyncScheduledParams{
					ConnectionID: connection.ID, JobID: scheduledJobID,
					ScheduledAt: time.Time{}, NextRunAt: nextScheduledAt,
				},
			)
			require.ErrorContains(t, err, "scheduled at must be a non-zero timestamp")
			recorded, err := service.RecordBankConnectionSyncScheduled(
				t.Context(),
				RecordBankConnectionSyncScheduledParams{
					ConnectionID: connection.ID, JobID: scheduledJobID,
					ScheduledAt: scheduledAt, NextRunAt: nextScheduledAt,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, &scheduledAt, recorded.LastScheduledAt)
			assert.Equal(t, &nextScheduledAt, recorded.NextRunAt)
			assert.Equal(t, scheduledJobID, recorded.LastJobID)

			jobRef, err := service.TriggerBankConnectionSync(
				t.Context(),
				TriggerBankConnectionSyncParams{
					ActorUserID:  ownerUserID,
					TenantID:     tenant.ID,
					ConnectionID: connection.ID,
					Reason:       BankConnectionSyncReasonManual,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, BankConnectionSyncJobType, jobRef.JobType)
			require.NotNil(t, enqueuer.request)
			assert.Equal(t, connection.ID, enqueuer.request.Input.ConnectionID)

			firstSync, err := service.RunBankConnectionSync(
				t.Context(),
				RunBankConnectionSyncParams{
					ConnectionID: connection.ID,
					JobID:        jobRef.ID,
					Reason:       BankConnectionSyncReasonScheduled,
					ScheduledAt:  &scheduledAt, ScheduledNextRunAt: &nextScheduledAt,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 1, firstSync.ImportedAccounts)
			assert.Equal(t, 1, firstSync.ImportedTransactions)
			require.Len(t, provider.syncParams, 1)
			assert.Equal(t, connection.ProviderReference, provider.syncParams[0].ProviderReference)

			duplicateSync, err := service.ApplyProviderSyncResult(
				t.Context(),
				ApplyProviderSyncResultParams{
					ConnectionID: connection.ID,
					JobID:        "job-duplicate",
					Result:       provider.syncResults[0],
				},
			)
			require.NoError(t, err)
			assert.Zero(t, duplicateSync.ImportedTransactions)

			secondSync, err := service.RunBankConnectionSync(
				t.Context(),
				RunBankConnectionSyncParams{
					ConnectionID: connection.ID,
					JobID:        "job-sync-2",
					Reason:       BankConnectionSyncReasonScheduled,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 1, secondSync.UpdatedTransactions)
			require.Len(t, provider.syncParams, 2)
			assert.Equal(t, connection.ProviderReference, provider.syncParams[1].ProviderReference)

			connections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, connections, 1)
			assert.Equal(t, domain.BankConnectionStateActive, connections[0].Connection.State)
			assert.Nil(t, connections[0].Connection.Reauth)
			require.NotNil(t, connections[0].Schedule)
			assert.Equal(t, "job-sync-2", connections[0].Schedule.LastJobID)

			accounts, err := store.ListConnectionProviderAccounts(t.Context(), connection.ID)
			require.NoError(t, err)
			require.Len(t, accounts, 1)
			assert.Equal(t, providerAccountID, accounts[0].ProviderAccountID)

			snapshots, err := store.ListBalanceSnapshots(t.Context(), connection.ID)
			require.NoError(t, err)
			require.Len(t, snapshots, 1)
			assert.Equal(t, int64(501_23), snapshots[0].CurrentBalanceMinor)

			transactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, transactions, 1)
			assert.Equal(t, domain.TransactionStatusBooked, transactions[0].Status)
			require.NotNil(t, transactions[0].ProviderOriginal)

			accountsBeforeDelete, err := service.ListAccounts(t.Context(), ListAccountsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, accountsBeforeDelete, 1)

			require.NoError(t, service.DeleteBankConnection(t.Context(), DeleteBankConnectionParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
			}))
			require.Len(t, scheduleWriter.schedules, 4)
			assert.False(t, scheduleWriter.schedules[3].Enabled)
			assert.Nil(t, scheduleWriter.schedules[3].NextRunAt)

			remainingConnections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			assert.Empty(t, remainingConnections)
			_, err = store.GetBankConnection(t.Context(), connection.ID)
			require.ErrorIs(t, err, persistence.ErrBankConnectionNotFound)
			_, err = store.GetBankConnectionSchedule(t.Context(), connection.ID)
			require.ErrorIs(t, err, persistence.ErrBankConnectionScheduleNotFound)
			accounts, err = store.ListConnectionProviderAccounts(t.Context(), connection.ID)
			require.NoError(t, err)
			assert.Empty(t, accounts)
			snapshots, err = store.ListBalanceSnapshots(t.Context(), connection.ID)
			require.NoError(t, err)
			assert.Empty(t, snapshots)
			_, err = store.GetConnectionSecret(t.Context(), connection.SecretID)
			require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)

			transactions, err = service.ListTransactions(t.Context(), ListTransactionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, transactions, 1)
			accountsAfterDelete, err := service.ListAccounts(t.Context(), ListAccountsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, accountsAfterDelete, 1)

			assert.Equal(t, []string{providerSecret, providerSecret}, provider.secrets)
			assert.NotContains(t, logs.String(), providerSecret)
		},
	)

	t.Run("tolerates missing schedules and persists failure lifecycle", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
			syncErr: errors.New("provider sync failed"),
		}
		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
			WithNow(func() time.Time { return now }),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)
		connection := saveLinkedConnectionForTest(
			t,
			service,
			tenant.ID,
			provider.name,
			domain.ProviderConnectorIDMonobank,
			ProviderLinkResult{
				DisplayName:       provider.linkResult.DisplayName,
				ProviderReference: provider.linkResult.ProviderReference,
				Secret:            provider.linkResult.Secret,
				State:             provider.linkResult.State,
			},
		)

		views, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, views, 1)
		assert.Nil(t, views[0].Schedule)

		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-failed",
			Reason:       BankConnectionSyncReasonManual,
		})
		require.Error(t, err)

		storedConnection, err := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		require.NotNil(t, storedConnection.LastSyncStartedAt)
		assert.Equal(t, now, *storedConnection.LastSyncStartedAt)
		assert.Contains(t, storedConnection.LastSyncError, "provider sync failed")
		assert.Equal(t, "job-failed", storedConnection.LastSyncJobID)
		assert.Nil(t, storedConnection.LastSuccessfulSyncAt)

		scheduledAt := time.Date(2026, time.June, 20, 14, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
		nextRunAt := scheduledAt.Add(time.Hour)
		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID: ownerUserID, TenantID: tenant.ID, ConnectionID: connection.ID,
			Interval: time.Hour, NextRunAt: scheduledAt,
		})
		require.NoError(t, err)
		failedJobID := "job-failed-scheduled-" + fake.UUID().V4()
		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID, JobID: failedJobID, Reason: BankConnectionSyncReasonScheduled,
			ScheduledAt: &scheduledAt, ScheduledNextRunAt: &nextRunAt,
		})
		require.Error(t, err)
		views, err = service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID, TenantID: tenant.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, views[0].Schedule)
		require.NotNil(t, views[0].Schedule.LastScheduledAt)
		require.NotNil(t, views[0].Schedule.NextRunAt)
		assert.True(t, scheduledAt.Equal(*views[0].Schedule.LastScheduledAt))
		assert.True(t, nextRunAt.Equal(*views[0].Schedule.NextRunAt))
		assert.Equal(
			t,
			scheduledAt.Format(time.RFC3339Nano),
			views[0].Schedule.LastScheduledAt.Format(time.RFC3339Nano),
		)
		assert.Equal(
			t,
			nextRunAt.Format(time.RFC3339Nano),
			views[0].Schedule.NextRunAt.Format(time.RFC3339Nano),
		)
		assert.Equal(t, &now, views[0].Schedule.LastStartedAt)
		assert.Equal(t, &now, views[0].Schedule.LastCompletedAt)
		assert.Equal(t, failedJobID, views[0].Schedule.LastJobID)
	})

	t.Run("uses checkpoint-based default sync windows when no window was requested", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		currentTime := time.Date(2026, time.July, 6, 12, 0, 0, 0, time.UTC)
		provider := &stubBankProvider{
			name: bankConnectorEnableBanking,
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
			syncResults: []ProviderSyncResult{{}, {}},
		}
		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
			WithNow(func() time.Time { return currentTime }),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)
		connection := saveLinkedConnectionForTest(
			t,
			service,
			tenant.ID,
			bankProviderPKO,
			domain.ProviderConnectorIDEnableBanking,
			ProviderLinkResult{
				DisplayName:       provider.linkResult.DisplayName,
				ProviderReference: provider.linkResult.ProviderReference,
				Secret:            provider.linkResult.Secret,
				State:             provider.linkResult.State,
			},
		)

		_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-fresh-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
		})
		require.NoError(t, err)
		require.Len(t, provider.syncParams, 1)
		assert.Equal(t, currentTime.AddDate(-3, 0, 0), provider.syncParams[0].WindowStart)
		assert.Equal(t, currentTime, provider.syncParams[0].WindowEnd)

		currentTime = currentTime.Add(48 * time.Hour)
		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-refresh-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
		})
		require.NoError(t, err)
		require.Len(t, provider.syncParams, 2)
		assert.Equal(t, currentTime.AddDate(0, 0, -30), provider.syncParams[1].WindowStart)
		assert.Equal(t, currentTime, provider.syncParams[1].WindowEnd)
	})

	t.Run("surfaces schedule writer failures", func(t *testing.T) {
		failureFake := faker.New()
		failureStore := makeStore(t)
		failureProvider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "Connection " + failureFake.Company().Name(),
				ProviderReference: "provider-ref-" + failureFake.UUID().V4(),
				Secret:            "secret-" + failureFake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		failureService := NewService(
			failureStore,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(failureProvider),
			WithBankConnectionSyncScheduleWriter(
				&capturedBankSyncScheduleWriter{err: errors.New("schedule write failed")},
			),
		)
		failureOwnerUserID := "user-owner-" + failureFake.UUID().V4()
		failureTenant := makeTenant(t, failureService, failureOwnerUserID)
		failureConnection := saveLinkedConnectionForTest(
			t,
			failureService,
			failureTenant.ID,
			failureProvider.Name(),
			domain.ProviderConnectorIDMonobank,
			ProviderLinkResult{
				DisplayName:       failureProvider.linkResult.DisplayName,
				ProviderReference: failureProvider.linkResult.ProviderReference,
				Secret:            failureProvider.linkResult.Secret,
				State:             failureProvider.linkResult.State,
			},
		)
		_, err := failureService.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  failureOwnerUserID,
			TenantID:     failureTenant.ID,
			ConnectionID: failureConnection.ID,
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC(),
		})
		require.ErrorContains(t, err, "write bank connection sync schedule")
	})

	t.Run("covers schedule writer helper branches", func(t *testing.T) {
		service := NewService(makeStore(t))
		require.NoError(t, service.writeBankConnectionSyncSchedule(t.Context(), BankConnectionSyncSchedule{}))
		writer := &capturedBankSyncScheduleWriter{}
		service = NewService(makeStore(t), WithBankConnectionSyncScheduleWriter(writer))
		require.NoError(t, service.writeBankConnectionSyncSchedule(
			t.Context(),
			BankConnectionSyncSchedule{ConnectionID: "connection-1"},
		))
		require.Len(t, writer.schedules, 1)
		assert.Equal(t, "finance.bank_connection_sync:connection-1", bankConnectionSyncScheduleID("connection-1"))
	})

	t.Run("covers tenant-access-denied error branches for schedule flows", func(t *testing.T) {
		fake := faker.New()
		service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)))
		_, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  "missing-user-" + fake.UUID().V4(),
			TenantID:     "missing-tenant-" + fake.UUID().V4(),
			ConnectionID: "missing-connection-" + fake.UUID().V4(),
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("covers missing connection error branches", func(t *testing.T) {
		fake := faker.New()
		service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)))
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)
		_, err := service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: "missing-connection-" + fake.UUID().V4(),
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC(),
		})
		require.ErrorIs(t, err, ErrBankConnectionNotFound)
	})

	t.Run("returns explicit configuration errors for unconfigured bank providers", func(t *testing.T) {
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()

		t.Run("pko sync", func(t *testing.T) {
			store := makeStore(t)
			service := NewService(store, WithConnectionSecretCipher(makeCipher(t)))
			tenant := makeTenant(t, service, ownerUserID)
			connection, saveErr := store.SaveBankConnection(t.Context(), domain.BankConnection{
				ID:                "connection-" + fake.UUID().V4(),
				TenantID:          tenant.ID,
				Provider:          bankProviderPKO,
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				SecretID:          "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				CreatedAt:         time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			})
			require.NoError(t, saveErr)

			_, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
				ConnectionID: connection.ID,
				JobID:        "job-" + fake.UUID().V4(),
			})
			require.ErrorIs(t, err, persistence.ErrConnectionSecretNotFound)
		})
	})

	t.Run("keeps duplicate apply atomic enough under concurrent runs", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		service := NewService(store, WithConnectionSecretCipher(makeCipher(t)))
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)
		providerAccountID := "provider-account-" + fake.UUID().V4()
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                "connection-" + fake.UUID().V4(),
			TenantID:          tenant.ID,
			Provider:          "provider-" + fake.Lorem().Word(),
			DisplayName:       "display",
			ProviderReference: "ref",
			SecretID:          "secret",
			State:             domain.BankConnectionStateActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		})
		require.NoError(t, err)
		result := ProviderSyncResult{
			SyncKey: "sync-" + fake.UUID().V4(),
			Accounts: []ProviderNormalizedAccount{{
				ProviderAccountID: providerAccountID,
				Name:              "main",
				Currency:          "PLN",
			}},
			Transactions: []ProviderNormalizedTransaction{{
				ProviderAccountID: providerAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       -1200,
				Currency:          "PLN",
				Description:       "txn-" + fake.Lorem().Word(),
				EffectiveAt:       time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Fingerprint:       "fp-" + fake.UUID().V4(),
			}},
		}

		var wait sync.WaitGroup
		wait.Add(2)
		errs := make(chan error, 2)
		for range 2 {
			go func() {
				defer wait.Done()
				_, applyErr := service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
					ConnectionID: connection.ID,
					JobID:        "job-dup",
					Result:       result,
				})
				errs <- applyErr
			}()
		}
		wait.Wait()
		close(errs)
		for applyErr := range errs {
			require.NoError(t, applyErr)
		}

		transactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 1)
	})
}
