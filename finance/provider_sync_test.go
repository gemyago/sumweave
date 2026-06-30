package finance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/golang-jwt/jwt/v5"
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
	p.secrets = append(p.secrets, params.Secret)
	if p.syncCalls >= len(p.syncResults) {
		return ProviderSyncResult{}, errors.New("no stub sync result")
	}
	result := p.syncResults[p.syncCalls]
	p.syncCalls++
	return result, nil
}

//nolint:gocyclo,cyclop // Integration-style provider coverage keeps related scenarios together.
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
		})
		require.NoError(t, err)
		return tenant
	}

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
					ExternalID:        "external-" + fake.UUID().V4(),
					Secret:            providerSecret,
					State:             domain.BankConnectionStateActive,
					RawPayloads: []ProviderRawPayload{{
						Scope:            domain.RawPayloadScopeConnection,
						ProviderObjectID: "link-" + fake.UUID().V4(),
						PayloadJSON:      []byte(`{"linked":true}`),
					}},
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
							RawPayloadJSON: []byte(`{"pending":true}`),
						}},
						RawPayloads: []ProviderRawPayload{{
							Scope:            domain.RawPayloadScopeTransaction,
							ProviderObjectID: "pending-" + fake.UUID().V4(),
							PayloadJSON:      []byte(`{"stage":"pending"}`),
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
							RawPayloadJSON: []byte(`{"pending":false}`),
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

			connection, err := service.LinkTokenBankConnection(
				t.Context(),
				LinkTokenBankConnectionParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
					Provider:    provider.name,
					Token:       providerToken,
				},
			)
			require.NoError(t, err)
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
			require.Len(t, scheduleWriter.schedules, 2)
			assert.False(t, scheduleWriter.schedules[1].Enabled)

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
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 1, firstSync.ImportedAccounts)
			assert.Equal(t, 1, firstSync.ImportedTransactions)

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

			connections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			require.Len(t, connections, 1)
			assert.Equal(
				t,
				domain.BankConnectionStateReauthRequired,
				connections[0].Connection.State,
			)
			require.NotNil(t, connections[0].Connection.Reauth)
			assert.Equal(t, "sca_expired", connections[0].Connection.Reauth.Reason)
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

			rawPayloads, err := store.ListRawPayloads(t.Context(), connection.ID)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(rawPayloads), 2)

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
			rawPayloads, err = store.ListRawPayloads(t.Context(), connection.ID)
			require.NoError(t, err)
			assert.Empty(t, rawPayloads)
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

			assert.Equal(t, []string{providerToken}, provider.linkedTokens)
			assert.Equal(t, []string{providerSecret, providerSecret}, provider.secrets)
			assert.NotContains(t, logs.String(), providerToken)
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
		connection, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    provider.name,
			Token:       "token-" + fake.UUID().V4(),
		})
		require.NoError(t, err)

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
		assert.Equal(t, now, storedConnection.LastSyncStartedAt.UTC())
		assert.Equal(t, "provider sync failed", storedConnection.LastSyncError)
		assert.Equal(t, "job-failed", storedConnection.LastSyncJobID)
		assert.Nil(t, storedConnection.LastSuccessfulSyncAt)
	})

	t.Run("surfaces schedule writer failures", func(t *testing.T) {
		failureFake := faker.New()
		failureStore := makeStore(t)
		failureProvider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "Connection " + failureFake.Company().Name(),
				ProviderReference: "provider-ref-" + failureFake.UUID().V4(),
				ExternalID:        "external-" + failureFake.UUID().V4(),
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
		failureConnection, err := failureService.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: failureOwnerUserID,
				TenantID:    failureTenant.ID,
				Provider:    failureProvider.Name(),
				Token:       "token-" + failureFake.UUID().V4(),
			},
		)
		require.NoError(t, err)
		_, err = failureService.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
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

	t.Run("supports redirect link start and finish flows", func(t *testing.T) {
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()
		provider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.com/auth",
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "Redirected " + fake.Company().Name(),
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
				ExternalID:        "external-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)), WithBankProviders(provider))
		tenant := makeTenant(t, service, ownerUserID)
		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example.test/callback",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.startResult.AuthorizationURL, start.AuthorizationURL)
		assert.Equal(t, provider.startResult.ProviderReference, start.ProviderReference)
		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-" + fake.UUID().V4(),
			Start:       start,
		})
		require.NoError(t, err)
		assert.Equal(t, "pko", connection.Provider)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, connection.ConnectorID)
		assert.Equal(t, provider.finishResult.ProviderReference, connection.ProviderReference)
	})

	t.Run("reuses existing pko connection on repeated redirect linking", func(t *testing.T) {
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()
		provider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "state-1-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.com/auth/1",
				ProviderReference: "provider-ref-1-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "Redirected " + fake.Company().Name(),
				ProviderReference: "provider-ref-1-" + fake.UUID().V4(),
				ExternalID:        "external-1-" + fake.UUID().V4(),
				Secret:            "secret-1-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)), WithBankProviders(provider))
		tenant := makeTenant(t, service, ownerUserID)

		startOne, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderPKO,
			RedirectURL: "https://app.example.test/callback/1",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.startResult.ProviderReference, startOne.ProviderReference)
		firstConnection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderPKO,
			State:       startOne.State,
			Code:        "code-1-" + fake.UUID().V4(),
		})
		require.NoError(t, err)

		provider.startResult = ProviderLinkStart{
			State:             "state-2-" + fake.UUID().V4(),
			AuthorizationURL:  "https://example.com/auth/2",
			ProviderReference: "provider-ref-2-" + fake.UUID().V4(),
		}
		provider.finishResult = ProviderLinkResult{
			DisplayName:       "Redirected again " + fake.Company().Name(),
			ProviderReference: "provider-ref-2-" + fake.UUID().V4(),
			ExternalID:        "external-2-" + fake.UUID().V4(),
			Secret:            "secret-2-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
		}

		startTwo, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderPKO,
			RedirectURL: "https://app.example.test/callback/2",
		})
		require.NoError(t, err)
		assert.Equal(t, provider.startResult.ProviderReference, startTwo.ProviderReference)
		secondConnection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    bankProviderPKO,
			State:       startTwo.State,
			Code:        "code-2-" + fake.UUID().V4(),
		})
		require.NoError(t, err)

		assert.Equal(t, firstConnection.ID, secondConnection.ID)
		assert.Equal(t, firstConnection.CreatedAt, secondConnection.CreatedAt)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, secondConnection.ConnectorID)
		assert.Equal(t, provider.finishResult.ProviderReference, secondConnection.ProviderReference)

		connections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, connections, 1)
		assert.Equal(t, secondConnection.ID, connections[0].Connection.ID)
	})

	t.Run("covers tenant-access-denied error branches for link and schedule flows", func(t *testing.T) {
		fake := faker.New()
		service := NewService(
			makeStore(t),
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(&stubBankProvider{name: "monobank"}),
		)
		_, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: "missing-user-" + fake.UUID().V4(),
			TenantID:    "missing-tenant-" + fake.UUID().V4(),
			Provider:    "provider-" + fake.Lorem().Word(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: "missing-user-" + fake.UUID().V4(),
			TenantID:    "missing-tenant-" + fake.UUID().V4(),
			Provider:    "provider-" + fake.Lorem().Word(),
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  "missing-user-" + fake.UUID().V4(),
			TenantID:     "missing-tenant-" + fake.UUID().V4(),
			ConnectionID: "missing-connection-" + fake.UUID().V4(),
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("covers missing provider and missing connection error branches", func(t *testing.T) {
		fake := faker.New()
		service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)))
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)
		_, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "missing-provider-" + fake.Lorem().Word(),
		})
		require.ErrorContains(t, err, "unsupported bank provider")
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "missing-provider-" + fake.Lorem().Word(),
			Token:       "token-" + fake.UUID().V4(),
		})
		require.ErrorContains(t, err, "unsupported bank provider")
		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
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

		t.Run("monobank token link", func(t *testing.T) {
			service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)))
			tenant := makeTenant(t, service, ownerUserID)

			_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    bankProviderMonobank,
				Token:       "token-" + fake.UUID().V4(),
			})
			require.ErrorIs(t, err, ErrBankProviderNotConfigured)
			require.ErrorContains(t, err, bankProviderMonobank)
		})

		t.Run("pko redirect link", func(t *testing.T) {
			service := NewService(makeStore(t), WithConnectionSecretCipher(makeCipher(t)))
			tenant := makeTenant(t, service, ownerUserID)

			_, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    bankProviderPKO,
				RedirectURL: "https://app.example.test/#/finance/connections",
			})
			require.ErrorIs(t, err, ErrBankProviderNotConfigured)
			require.ErrorContains(t, err, bankProviderPKO)
			require.ErrorContains(t, err, bankConnectorEnableBanking)
		})

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
			require.ErrorIs(t, err, ErrBankProviderNotConfigured)
			require.ErrorContains(t, err, bankProviderPKO)
			require.ErrorContains(t, err, bankConnectorEnableBanking)
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

	t.Run("productizes enable banking redirect flow and provider sync", func(t *testing.T) {
		fake := faker.New()
		stateValue := "state-" + fake.UUID().V4()
		providerSecret := "session-" + fake.UUID().V4()
		server := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case request.Method == http.MethodPost && request.URL.Path == "/auth":
					_, _ = writer.Write(
						[]byte(
							`{"authorizationUrl":"https://bank.example/auth","providerReference":"ref-123"}`,
						),
					)
				case request.Method == http.MethodPost && request.URL.Path == "/sessions":
					_, _ = writer.Write(
						[]byte(
							`{"externalId":"session-123","secret":"` + providerSecret + `","displayName":"PKO","state":"active"}`,
						),
					)
				case request.Method == http.MethodGet && request.URL.Path == "/accounts":
					_, _ = writer.Write(
						[]byte(
							`{"state":"reauth_required","reauthReason":"sca_expired","reauthRequiredAt":"2026-06-20T12:00:00Z","accounts":[{"id":"acc-1","name":"ROR","currency":"PLN","iban":"PL61109010140000071219812874"}]}`,
						),
					)
				case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/balances":
					_, _ = writer.Write(
						[]byte(
							`{"balances":[{"currentBalanceMinor":55123,"availableBalanceMinor":53123,"currency":"PLN"}]}`,
						),
					)
				case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/transactions":
					_, _ = writer.Write(
						[]byte(
							`{"transactions":[{"transactionId":"txn-1","status":"booked","amountMinor":-2500,"currency":"PLN","description":"Coffee","effectiveAt":"2026-06-20T10:00:00Z"}]}`,
						),
					)
				default:
					http.NotFound(writer, request)
				}
			}),
		)
		defer server.Close()

		provider := NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:       server.URL,
			StateProvider: func() (string, error) { return stateValue, nil },
		})
		store := makeStore(t)
		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example/callback",
		})
		require.NoError(t, err)
		assert.Equal(t, stateValue, start.State)
		assert.Equal(t, "https://bank.example/auth", start.AuthorizationURL)

		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       stateValue,
			Code:        "code-123",
			Start:       start,
		})
		require.NoError(t, err)
		assert.Equal(t, "pko", connection.Provider)
		assert.Equal(t, providerSecret, mustLoadSecret(t, service, connection.SecretID))

		rawPayloads, err := store.ListRawPayloads(t.Context(), connection.ID)
		require.NoError(t, err)
		require.Len(t, rawPayloads, 1)
		assert.NotContains(t, string(rawPayloads[0].PayloadJSON), providerSecret)
		assert.NotContains(t, string(rawPayloads[0].PayloadJSON), `"secret"`)

		result, err := provider.Sync(t.Context(), ProviderSyncParams{
			Secret: providerSecret,
		})
		require.NoError(t, err)
		require.Len(t, result.Accounts, 1)
		require.Len(t, result.Transactions, 1)
		assert.Equal(t, "acc-1", result.Accounts[0].ProviderAccountID)
		assert.Equal(t, "txn-1", result.Transactions[0].ProviderTransactionID)
		require.NotNil(t, result.Transactions[0].ProviderOriginal)
		require.NotNil(t, result.Reauth)
		assert.Equal(t, "sca_expired", result.Reauth.Reason)
		assert.NotEmpty(t, result.RawPayloads)

		errorServer := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
				_, _ = writer.Write([]byte(`{"message":"token ` + providerSecret + ` leaked"}`))
			}),
		)
		defer errorServer.Close()
		errorProvider := NewEnableBankingProvider(
			EnableBankingProviderConfig{BaseURL: errorServer.URL},
		)
		_, err = errorProvider.Sync(t.Context(), ProviderSyncParams{Secret: providerSecret})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), providerSecret)
	})

	t.Run("supports signed official enable banking flows and session-based sync", func(t *testing.T) {
		fake := faker.New()
		fixedNow := time.Date(2026, time.June, 23, 9, 0, 0, 0, time.UTC)
		stateValue := "state-" + fake.UUID().V4()

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		privateKeyPEM := pem.EncodeToMemory(
			&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER},
		)
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

		verifyJWT := func(t *testing.T, request *http.Request) {
			t.Helper()
			authorization := strings.TrimSpace(request.Header.Get("Authorization"))
			require.True(t, strings.HasPrefix(authorization, "Bearer "))
			tokenString := strings.TrimPrefix(authorization, "Bearer ")
			parser := jwt.NewParser(jwt.WithoutClaimsValidation())
			token, parseErr := parser.Parse(tokenString, func(_ *jwt.Token) (any, error) {
				return &privateKey.PublicKey, nil
			})
			require.NoError(t, parseErr)
			require.True(t, token.Valid)
			require.Equal(t, "app-123", token.Header["kid"])
			claims, ok := token.Claims.(jwt.MapClaims)
			require.True(t, ok)
			require.Equal(t, enableBankingJWTIssuer, claims["iss"])
			require.Equal(t, enableBankingJWTAudience, claims["aud"])
		}

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			verifyJWT(t, request)
			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/auth":
				var body map[string]any
				if decodeErr := json.NewDecoder(request.Body).Decode(&body); decodeErr != nil {
					t.Errorf("decode auth body: %v", decodeErr)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				access, ok := body["access"].(map[string]any)
				if !ok {
					t.Errorf("auth body access missing: %#v", body)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				aspsp, ok := body["aspsp"].(map[string]any)
				if !ok {
					t.Errorf("auth body aspsp missing: %#v", body)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				assert.NotEmpty(t, access["valid_until"])
				assert.Equal(t, EnableBankingDefaultCountry, aspsp["country"])
				assert.Equal(t, EnableBankingDefaultASPSPName, aspsp["name"])
				assert.Equal(t, EnableBankingDefaultPSUType, body["psu_type"])
				assert.Equal(t, stateValue, body["state"])
				assert.Equal(t, "https://backend.example.test/enable-banking/callback", body["redirect_url"])
				_, _ = writer.Write([]byte(`{"url":"https://bank.example.test/authorize","id":"auth-123"}`))
			case request.Method == http.MethodPost && request.URL.Path == "/sessions":
				var body map[string]any
				if decodeErr := json.NewDecoder(request.Body).Decode(&body); decodeErr != nil {
					t.Errorf("decode session body: %v", decodeErr)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				assert.Equal(t, "code-123", body["code"])
				_, _ = writer.Write([]byte(
					`{"id":"session-123","accounts":[{"uid":"acc-1","name":"ROR","currency":"PLN","iban":"PL123"}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/sessions/session-123":
				_, _ = writer.Write([]byte(
					`{"id":"session-123","state":"active","accounts":[{"uid":"acc-1","name":"ROR","currency":"PLN","iban":"PL123"}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/balances":
				_, _ = writer.Write([]byte(
					`{"balances":[{"type":"closingBooked","balance_amount":{"amount":"551.23","currency":"PLN"}},{"type":"interimAvailable","balance_amount":{"amount":"531.23","currency":"PLN"}}]}`,
				))
			case request.Method == http.MethodGet && request.URL.Path == "/accounts/acc-1/transactions":
				assert.Equal(
					t,
					url.Values{"date_from": []string{"2026-06-01"}, "date_to": []string{"2026-06-15"}},
					request.URL.Query(),
				)
				_, _ = writer.Write([]byte(
					`{"transactions":[{"id":"txn-1","status":"booked","booking_date":"2026-06-10","amount":{"amount":"25.00","currency":"PLN"},"credit_debit_indicator":"DBIT","remittance_information_unstructured":"Coffee"}]}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		provider := NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:        server.URL,
			AppID:          "app-123",
			PrivateKeyPath: privateKeyPath,
			StateProvider:  func() (string, error) { return stateValue, nil },
			Now:            func() time.Time { return fixedNow },
		})
		store := makeStore(t)
		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://backend.example.test/enable-banking/callback",
		})
		require.NoError(t, err)
		assert.Equal(t, stateValue, start.State)
		assert.Equal(t, "https://bank.example.test/authorize", start.AuthorizationURL)

		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       stateValue,
			Code:        "code-123",
		})
		require.NoError(t, err)
		assert.Equal(t, "pko", connection.Provider)
		assert.Equal(t, "session-123", connection.ExternalID)
		assert.Empty(t, mustLoadSecret(t, service, connection.SecretID))

		result, err := provider.Sync(t.Context(), ProviderSyncParams{
			ExternalID:  connection.ExternalID,
			WindowStart: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			WindowEnd:   time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.Len(t, result.Accounts, 1)
		require.Len(t, result.Transactions, 1)
		assert.Equal(t, int64(55123), *result.Accounts[0].CurrentBalanceMinor)
		assert.Equal(t, int64(53123), *result.Accounts[0].AvailableBalanceMinor)
		assert.Equal(t, int64(-2500), result.Transactions[0].AmountMinor)
		assert.Equal(t, "txn-1", result.Transactions[0].ProviderTransactionID)
		assert.NotEmpty(t, result.RawPayloads)
	})

	t.Run(
		"productizes monobank token sync with range chunking and rate limit sanitization",
		func(t *testing.T) {
			fake := faker.New()
			providerToken := "mono-" + fake.UUID().V4()
			var statementHits []string
			server := httptest.NewServer(
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					switch {
					case request.Method == http.MethodGet && request.URL.Path == "/personal/client-info":
						_, _ = writer.Write(
							[]byte(
								`{"name":"mono","accounts":[{"id":"0","type":"black","currencyCode":980,"balance":150500,"iban":"UA123"},{"id":"1","type":"white","currencyCode":840,"balance":50500,"iban":"UA456"}]}`,
							),
						)
					case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/personal/statement/"):
						statementHits = append(statementHits, request.URL.Path)
						_, _ = writer.Write(
							[]byte(
								`[{"id":"st-1","time":1718870400,"description":"Shop","amount":-5050,"currencyCode":980,"balance":145450,"hold":false}]`,
							),
						)
					default:
						http.NotFound(writer, request)
					}
				}),
			)
			defer server.Close()

			sleepCalls := 0
			provider := NewMonobankProvider(MonobankProviderConfig{
				BaseURL:              server.URL,
				SleepBetweenRequests: time.Millisecond,
				Sleep:                func(time.Duration) { sleepCalls++ },
			})

			linkResult, err := provider.LinkToken(
				t.Context(),
				ProviderTokenLinkParams{Token: providerToken},
			)
			require.NoError(t, err)
			assert.Equal(t, providerToken, linkResult.Secret)

			result, err := provider.Sync(t.Context(), ProviderSyncParams{
				Secret:      providerToken,
				WindowStart: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
				WindowEnd:   time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, err)
			require.Len(t, result.Accounts, 2)
			assert.Len(t, statementHits, 4)
			assert.Equal(t, 3, sleepCalls)
			require.Len(t, result.Transactions, 4)
			require.NotNil(t, result.Transactions[0].ProviderOriginal)
			assert.NotEmpty(t, result.RawPayloads)

			rateLimitServer := httptest.NewServer(
				http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
					writer.WriteHeader(http.StatusTooManyRequests)
					_, _ = writer.Write([]byte(`too many requests for ` + providerToken))
				}),
			)
			defer rateLimitServer.Close()
			rateLimited := NewMonobankProvider(MonobankProviderConfig{BaseURL: rateLimitServer.URL})
			_, err = rateLimited.Sync(t.Context(), ProviderSyncParams{Secret: providerToken})
			require.Error(t, err)
			assert.NotContains(t, err.Error(), providerToken)
		},
	)
}

func mustLoadSecret(t *testing.T, service *Service, secretID string) string {
	t.Helper()
	secret, err := service.decryptConnectionSecret(t.Context(), secretID)
	require.NoError(t, err)
	return secret
}
