package finance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	providers "github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingProviderSyncStore struct {
	*persistence.Store

	getScheduleErr           error
	saveScheduleErr          error
	saveProviderAccountErr   error
	getSyncRunErr            error
	saveSnapshotErr          error
	listProviderAccountsErr  error
	saveRawPayloadErr        error
	saveBankConnectionErr    error
	saveSyncRunErr           error
	listBankConnectionsErr   error
	getScheduleForListErr    error
	saveConnectionSecretErr  error
	getConnectionSecretErr   error
	listAccountsErr          error
	getMatchByProviderIDErr  error
	getMatchByFingerprintErr error
	getTransactionErr        error
	saveTransactionErr       error
	saveTransactionMatchErr  error
	restorePendingStartErr   error
	getPendingStartErr       error
	deleteBankConnectionErr  error
	deleteScheduleErr        error
	deleteProviderAcctsErr   error
	deleteSnapshotsErr       error
	deleteRawPayloadsErr     error
	deleteSyncRunsErr        error
	deleteTxnMatchesErr      error
	deleteSecretErr          error
}

func (s *failingProviderSyncStore) WithTransaction(
	_ context.Context,
	fn func(*persistence.Store) error,
) error {
	return fn(s.Store)
}

func (s *failingProviderSyncStore) GetBankConnectionSchedule(
	ctx context.Context,
	connectionID string,
) (*domain.BankConnectionSchedule, error) {
	if s.getScheduleErr != nil {
		return nil, s.getScheduleErr
	}
	if s.getScheduleForListErr != nil {
		return nil, s.getScheduleForListErr
	}
	return s.Store.GetBankConnectionSchedule(ctx, connectionID)
}

func (s *failingProviderSyncStore) SaveBankConnectionSchedule(
	ctx context.Context,
	schedule domain.BankConnectionSchedule,
) (domain.BankConnectionSchedule, error) {
	if s.saveScheduleErr != nil {
		return domain.BankConnectionSchedule{}, s.saveScheduleErr
	}
	return s.Store.SaveBankConnectionSchedule(ctx, schedule)
}

func (s *failingProviderSyncStore) GetBankConnectionSyncRun(
	ctx context.Context,
	connectionID string,
	syncKey string,
) (*domain.BankConnectionSyncRun, error) {
	if s.getSyncRunErr != nil {
		return nil, s.getSyncRunErr
	}
	return s.Store.GetBankConnectionSyncRun(ctx, connectionID, syncKey)
}

func (s *failingProviderSyncStore) SaveBalanceSnapshot(
	ctx context.Context,
	snapshot domain.BalanceSnapshot,
) (domain.BalanceSnapshot, error) {
	if s.saveSnapshotErr != nil {
		return domain.BalanceSnapshot{}, s.saveSnapshotErr
	}
	return s.Store.SaveBalanceSnapshot(ctx, snapshot)
}

func (s *failingProviderSyncStore) ListConnectionProviderAccounts(
	ctx context.Context,
	connectionID string,
) ([]domain.ConnectionProviderAccount, error) {
	if s.listProviderAccountsErr != nil {
		return nil, s.listProviderAccountsErr
	}
	return s.Store.ListConnectionProviderAccounts(ctx, connectionID)
}

func (s *failingProviderSyncStore) SaveConnectionProviderAccount(
	ctx context.Context,
	account domain.ConnectionProviderAccount,
) (domain.ConnectionProviderAccount, error) {
	if s.saveProviderAccountErr != nil {
		return domain.ConnectionProviderAccount{}, s.saveProviderAccountErr
	}
	return s.Store.SaveConnectionProviderAccount(ctx, account)
}

func (s *failingProviderSyncStore) SaveRawPayload(
	ctx context.Context,
	payload domain.RawPayload,
) (domain.RawPayload, error) {
	if s.saveRawPayloadErr != nil {
		return domain.RawPayload{}, s.saveRawPayloadErr
	}
	return s.Store.SaveRawPayload(ctx, payload)
}

func (s *failingProviderSyncStore) SavePendingStart(
	ctx context.Context,
	start domain.PendingBankConnectionLinkStart,
) (domain.PendingBankConnectionLinkStart, error) {
	return persistence.NewProviderLinkPersistence(s.Store).SavePendingStart(ctx, start)
}

func (s *failingProviderSyncStore) ConsumePendingStart(
	ctx context.Context,
	request providers.ConsumePendingStartRequest,
) (*domain.PendingBankConnectionLinkStart, error) {
	return persistence.NewProviderLinkPersistence(s.Store).ConsumePendingStart(ctx, request)
}

func (s *failingProviderSyncStore) RestorePendingStart(
	ctx context.Context,
	request providers.RestorePendingStartRequest,
) error {
	if s.restorePendingStartErr != nil {
		return s.restorePendingStartErr
	}
	return persistence.NewProviderLinkPersistence(s.Store).RestorePendingStart(ctx, request)
}

func (s *failingProviderSyncStore) SaveBankConnection(
	ctx context.Context,
	connection domain.BankConnection,
) (domain.BankConnection, error) {
	if s.saveBankConnectionErr != nil {
		return domain.BankConnection{}, s.saveBankConnectionErr
	}
	return s.Store.SaveBankConnection(ctx, connection)
}

func (s *failingProviderSyncStore) SaveBankConnectionSyncRun(
	ctx context.Context,
	run domain.BankConnectionSyncRun,
) (domain.BankConnectionSyncRun, error) {
	if s.saveSyncRunErr != nil {
		return domain.BankConnectionSyncRun{}, s.saveSyncRunErr
	}
	return s.Store.SaveBankConnectionSyncRun(ctx, run)
}

func (s *failingProviderSyncStore) ClaimBankConnectionSyncRun(
	ctx context.Context,
	run domain.BankConnectionSyncRun,
) (bool, error) {
	if s.saveSyncRunErr != nil {
		return false, s.saveSyncRunErr
	}
	return s.Store.ClaimBankConnectionSyncRun(ctx, run)
}

func (s *failingProviderSyncStore) ListBankConnections(
	ctx context.Context,
	tenantID string,
) ([]domain.BankConnection, error) {
	if s.listBankConnectionsErr != nil {
		return nil, s.listBankConnectionsErr
	}
	return s.Store.ListBankConnections(ctx, tenantID)
}

func (s *failingProviderSyncStore) SaveConnectionSecret(
	ctx context.Context,
	secret domain.ConnectionSecret,
) (domain.ConnectionSecret, error) {
	if s.saveConnectionSecretErr != nil {
		return domain.ConnectionSecret{}, s.saveConnectionSecretErr
	}
	return s.Store.SaveConnectionSecret(ctx, secret)
}

func (s *failingProviderSyncStore) GetConnectionSecret(
	ctx context.Context,
	secretID string,
) (*domain.ConnectionSecret, error) {
	if s.getConnectionSecretErr != nil {
		return nil, s.getConnectionSecretErr
	}
	return s.Store.GetConnectionSecret(ctx, secretID)
}

func (s *failingProviderSyncStore) DeleteConnectionSecret(
	ctx context.Context,
	secretID string,
) error {
	if s.deleteSecretErr != nil {
		return s.deleteSecretErr
	}
	return s.Store.DeleteConnectionSecret(ctx, secretID)
}

func (s *failingProviderSyncStore) ListAccounts(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Account, error) {
	if s.listAccountsErr != nil {
		return nil, s.listAccountsErr
	}
	return s.Store.ListAccounts(ctx, tenantID, includeHidden)
}

func (s *failingProviderSyncStore) DeleteBankConnection(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteBankConnectionErr != nil {
		return s.deleteBankConnectionErr
	}
	return s.Store.DeleteBankConnection(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteBankConnectionSchedule(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteScheduleErr != nil {
		return s.deleteScheduleErr
	}
	return s.Store.DeleteBankConnectionSchedule(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteConnectionProviderAccounts(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteProviderAcctsErr != nil {
		return s.deleteProviderAcctsErr
	}
	return s.Store.DeleteConnectionProviderAccounts(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteBalanceSnapshots(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteSnapshotsErr != nil {
		return s.deleteSnapshotsErr
	}
	return s.Store.DeleteBalanceSnapshots(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteRawPayloads(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteRawPayloadsErr != nil {
		return s.deleteRawPayloadsErr
	}
	return s.Store.DeleteRawPayloads(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteBankConnectionSyncRuns(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteSyncRunsErr != nil {
		return s.deleteSyncRunsErr
	}
	return s.Store.DeleteBankConnectionSyncRuns(ctx, connectionID)
}

func (s *failingProviderSyncStore) DeleteProviderTransactionMatches(
	ctx context.Context,
	connectionID string,
) error {
	if s.deleteTxnMatchesErr != nil {
		return s.deleteTxnMatchesErr
	}
	return s.Store.DeleteProviderTransactionMatches(ctx, connectionID)
}

func (s *failingProviderSyncStore) GetProviderTransactionMatchByProviderID(
	ctx context.Context,
	connectionID string,
	providerAccountID string,
	providerTransactionID string,
) (*domain.ProviderTransactionMatch, error) {
	if s.getMatchByProviderIDErr != nil {
		return nil, s.getMatchByProviderIDErr
	}
	return s.Store.GetProviderTransactionMatchByProviderID(
		ctx,
		connectionID,
		providerAccountID,
		providerTransactionID,
	)
}

func (s *failingProviderSyncStore) GetProviderTransactionMatchByFingerprint(
	ctx context.Context,
	connectionID string,
	providerAccountID string,
	fingerprint string,
) (*domain.ProviderTransactionMatch, error) {
	if s.getMatchByFingerprintErr != nil {
		return nil, s.getMatchByFingerprintErr
	}
	return s.Store.GetProviderTransactionMatchByFingerprint(
		ctx,
		connectionID,
		providerAccountID,
		fingerprint,
	)
}

func (s *failingProviderSyncStore) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.Transaction, error) {
	if s.getTransactionErr != nil {
		return nil, s.getTransactionErr
	}
	return s.Store.GetTransaction(ctx, transactionID)
}

func (s *failingProviderSyncStore) SaveTransaction(
	ctx context.Context,
	transaction domain.Transaction,
) (domain.Transaction, error) {
	if s.saveTransactionErr != nil {
		return domain.Transaction{}, s.saveTransactionErr
	}
	return s.Store.SaveTransaction(ctx, transaction)
}

func (s *failingProviderSyncStore) SaveProviderTransactionMatch(
	ctx context.Context,
	match domain.ProviderTransactionMatch,
) (domain.ProviderTransactionMatch, error) {
	if s.saveTransactionMatchErr != nil {
		return domain.ProviderTransactionMatch{}, s.saveTransactionMatchErr
	}
	return s.Store.SaveProviderTransactionMatch(ctx, match)
}

func (s *failingProviderSyncStore) RestorePendingBankConnectionLinkStart(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	provider string,
	state string,
	restoredAt time.Time,
) error {
	if s.restorePendingStartErr != nil {
		return s.restorePendingStartErr
	}
	return s.Store.RestorePendingBankConnectionLinkStart(
		ctx,
		tenantID,
		actorUserID,
		provider,
		state,
		restoredAt,
	)
}

func (s *failingProviderSyncStore) GetPendingBankConnectionLinkStartByState(
	ctx context.Context,
	provider string,
	state string,
) (*domain.PendingBankConnectionLinkStart, error) {
	if s.getPendingStartErr != nil {
		return nil, s.getPendingStartErr
	}
	return s.Store.GetPendingBankConnectionLinkStartByState(ctx, provider, state)
}

type redirectBankProvider struct {
	name         string
	startResult  ProviderLinkStart
	startErr     error
	finishResult ProviderLinkResult
	finishErr    error
}

func (p *redirectBankProvider) Name() string { return p.name }

func (p *redirectBankProvider) StartLink(
	context.Context,
	ProviderStartLinkParams,
) (ProviderLinkStart, error) {
	if p.startErr != nil {
		return ProviderLinkStart{}, p.startErr
	}
	return p.startResult, nil
}

func (p *redirectBankProvider) FinishLink(
	context.Context,
	ProviderFinishLinkParams,
) (ProviderLinkResult, error) {
	if p.finishErr != nil {
		return ProviderLinkResult{}, p.finishErr
	}
	return p.finishResult, nil
}

func (p *redirectBankProvider) LinkToken(
	context.Context,
	ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	return ProviderTokenLinkResult{}, errors.New("unsupported")
}

func (p *redirectBankProvider) Sync(
	context.Context,
	ProviderSyncParams,
) (ProviderSyncResult, error) {
	return ProviderSyncResult{}, errors.New("unsupported")
}

func TestProviderSyncInternals(t *testing.T) {
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
	makeService := func(t *testing.T, provider BankConnectionProvider, opts ...ServiceOption) (*Service, domain.Tenant, string) {
		t.Helper()
		store := makeStore(t)
		serviceOpts := []ServiceOption{WithConnectionSecretCipher(makeCipher(t))}
		if provider != nil {
			serviceOpts = append(serviceOpts, WithBankProviders(provider))
		}
		serviceOpts = append(serviceOpts, opts...)
		service := NewService(store, serviceOpts...)
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(
			t.Context(),
			CreateTenantParams{
				ActorUserID:     ownerUserID,
				Name:            "tenant-" + fake.Company().Name(),
				DisplayCurrency: "USD",
			},
		)
		require.NoError(t, err)
		return service, tenant, ownerUserID
	}
	saveLinkedConnectionForTest := func(
		ctx context.Context,
		service *Service,
		tenantID string,
		providerName string,
		result ProviderLinkResult,
	) error {
		secretID, err := service.encryptAndSaveConnectionSecret(
			ctx,
			providerName,
			result.ProviderReference,
			result.Secret,
		)
		if err != nil {
			return err
		}
		syncStore, err := service.bankSyncStore()
		if err != nil {
			return err
		}
		now := service.now().UTC()
		connection := domain.BankConnection{
			ID:                service.newID(),
			TenantID:          tenantID,
			Provider:          providerName,
			ProviderReference: result.ProviderReference,
			ExternalID:        result.ExternalID,
			SecretID:          secretID,
			State:             result.State,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		saved, err := syncStore.SaveBankConnection(ctx, connection)
		if err != nil {
			return err
		}
		for _, payload := range result.RawPayloads {
			_, rawErr := syncStore.SaveRawPayload(ctx, domain.RawPayload{
				ID:               service.newID(),
				ConnectionID:     saved.ID,
				Scope:            payload.Scope,
				ProviderObjectID: payload.ProviderObjectID,
				PayloadJSON:      payload.PayloadJSON,
				CapturedAt:       now,
			})
			if rawErr != nil {
				return rawErr
			}
		}
		_ = saved
		return nil
	}

	t.Run("covers delete helper error branches", func(t *testing.T) {
		serviceWithoutSyncStore := &Service{}
		err := serviceWithoutSyncStore.deleteBankConnectionOwnedMetadata(
			t.Context(),
			domain.BankConnection{ID: "connection-1", SecretID: "secret-1"},
		)
		require.ErrorContains(t, err, "bank sync store is required")

		store := makeStore(t)
		failingStore := &failingProviderSyncStore{
			Store:               store,
			deleteTxnMatchesErr: errors.New("delete transaction matches failed"),
		}
		service := NewService(failingStore, WithConnectionSecretCipher(makeCipher(t)))
		connection := domain.BankConnection{ID: "connection-2", SecretID: "secret-2"}
		err = service.deleteBankConnectionOwnedMetadata(t.Context(), connection)
		require.ErrorContains(t, err, "delete bank connection")
		require.ErrorContains(t, err, "delete transaction matches failed")
	})

	t.Run("covers provider sync helper functions", func(t *testing.T) {
		assert.Empty(t, accountID(nil))
		assert.Equal(t, "account-1", accountID(&domain.ConnectionProviderAccount{ID: "account-1"}))
		assert.Empty(t, matchID(nil))
		assert.Equal(t, "match-1", matchID(&domain.ProviderTransactionMatch{ID: "match-1"}))
		assert.Nil(t, timePtrUTC(time.Time{}))
		now := time.Date(2026, time.June, 23, 10, 0, 0, 0, time.FixedZone("CET", 2*60*60))
		resolved := timePtrUTC(now)
		require.NotNil(t, resolved)
		assert.Equal(t, now.UTC(), *resolved)
		alreadyApplied, err := (&Service{}).syncRunAlreadyApplied(t.Context(), nil, "connection-1", "")
		require.NoError(t, err)
		assert.False(t, alreadyApplied)
		claimed, err := (&Service{}).claimSyncRun(t.Context(), nil, "connection-1", "", "job-1", now)
		require.NoError(t, err)
		assert.True(t, claimed)
		claimErrStore := &failingProviderSyncStore{Store: makeStore(t), saveSyncRunErr: errors.New("claim failed")}
		claimed, err = (&Service{newID: func() string { return "sync-run-1" }}).claimSyncRun(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: claimErrStore},
			"connection-1",
			"sync-key-1",
			"job-1",
			now,
		)
		require.ErrorContains(t, err, "apply provider sync result")
		assert.False(t, claimed)
	})

	t.Run("surfaces provider and membership configuration errors", func(t *testing.T) {
		fake := faker.New()
		service, tenant, ownerUserID := makeService(t, nil)
		_, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "missing-" + fake.Lorem().Word(),
			Token:       "token-" + fake.UUID().V4(),
		})
		require.Error(t, err)

		_, err = service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: "outsider-" + fake.UUID().V4(),
			TenantID:    tenant.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("covers direct option helpers and provider failures", func(t *testing.T) {
		service := &Service{}
		WithBankProviders(nil)(service)
		WithLogger(nil)(service)
		WithBankSyncJobEnqueuer(&capturedBankSyncJobEnqueuer{})(service)
		assert.NotNil(t, service.bankProviders)
		assert.NotNil(t, service.bankSyncJobEnqueuer)

		provider := &stubBankProvider{
			name:    "monobank",
			linkErr: errors.New("link failed"),
		}
		serviceWithProvider, tenant, ownerUserID := makeService(t, provider)
		_, err := serviceWithProvider.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    provider.name,
				Token:       "token",
			},
		)
		require.Error(t, err)

		monobankRef, refErr := serviceWithProvider.bankProviderForLink(bankProviderMonobank, bankLinkMethodToken)
		require.NoError(t, refErr)
		assert.Equal(t, bankProviderMonobank, monobankRef.bankID)
		assert.Equal(t, bankProviderMonobank, monobankRef.Name())

		pkoProvider := &stubBankProvider{name: bankConnectorEnableBanking}
		serviceWithRedirectProvider, _, _ := makeService(t, nil, WithBankProviders(pkoProvider))
		pkoRef, refErr := serviceWithRedirectProvider.bankProviderForLink(bankProviderPKO, bankLinkMethodRedirect)
		require.NoError(t, refErr)
		assert.Equal(t, bankProviderPKO, pkoRef.bankID)
		assert.Equal(t, bankConnectorEnableBanking, pkoRef.Name())

		rawPayloads := []ProviderRawPayload{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: " payload-id ",
			PayloadJSON:      []byte(`{"linked":true}`),
		}}
		assert.Equal(t, []domain.ProviderRawPayloadObservation{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: "payload-id",
			PayloadJSON:      []byte(`{"linked":true}`),
		}}, pendingStartRawPayloadObservations(rawPayloads))
		assert.Equal(t, []ProviderRawPayload{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: "payload-id",
			PayloadJSON:      []byte(`{"linked":true}`),
		}}, pendingStartRawPayloads([]domain.ProviderRawPayloadObservation{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: " payload-id ",
			PayloadJSON:      []byte(`{"linked":true}`),
		}}))

		serviceWithInvalidStore := &Service{store: stubStore{}}
		_, err = serviceWithInvalidStore.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: bankProviderPKO,
				State:    "state",
			},
		)
		require.ErrorContains(t, err, "bank sync store is required")

		serviceWithPendingStartFailure := &Service{
			store: &failingProviderSyncStore{
				Store:              makeStore(t),
				getPendingStartErr: errors.New("pending start failed"),
			},
		}
		_, err = serviceWithPendingStartFailure.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: bankProviderPKO,
				State:    "state",
			},
		)
		require.ErrorContains(t, err, "pending start failed")
	})

	t.Run("surfaces schedule and enqueue validation failures", func(t *testing.T) {
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		}
		service, tenant, ownerUserID := makeService(t, provider)
		connection, err := service.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    provider.name,
				Token:       "token",
			},
		)
		require.NoError(t, err)

		_, err = service.PauseBankConnectionSchedule(
			t.Context(),
			PauseBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
			},
		)
		require.Error(t, err)
		_, err = service.ResumeBankConnectionSchedule(
			t.Context(),
			ResumeBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.Error(t, err)
		_, err = service.TriggerBankConnectionSync(
			t.Context(),
			TriggerBankConnectionSyncParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Reason:       BankConnectionSyncReasonManual,
			},
		)
		require.Error(t, err)
	})

	t.Run("surfaces sync application failures and missing cipher branches", func(t *testing.T) {
		fake := faker.New()
		provider := &stubBankProvider{name: "monobank"}
		service, tenant, ownerUserID := makeService(t, provider)
		connection, err := service.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    provider.name,
				Token:       "token",
			},
		)
		require.NoError(t, err)

		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connection.ID,
			JobID:        "job-1",
			Result: ProviderSyncResult{
				SyncKey: "sync-1",
				Transactions: []ProviderNormalizedTransaction{{
					ProviderAccountID: "missing-account-" + fake.UUID().V4(),
					Status:            domain.TransactionStatusBooked,
					AmountMinor:       1,
					Currency:          "USD",
					Description:       "desc",
					EffectiveAt:       time.Now().UTC(),
					Fingerprint:       "fp-1",
				}},
			},
		})
		require.Error(t, err)

		_, err = service.RunBankConnectionSync(
			t.Context(),
			RunBankConnectionSyncParams{ConnectionID: "missing", JobID: "job-2"},
		)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)

		stored, saveErr := service.store.(bankSyncStore).SaveBankConnection(
			t.Context(),
			domain.BankConnection{
				ID:                "missing-provider-connection",
				TenantID:          tenant.ID,
				Provider:          "missing-provider",
				DisplayName:       "display",
				ProviderReference: "ref",
				SecretID:          connection.SecretID,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			},
		)
		require.NoError(t, saveErr)
		_, err = service.RunBankConnectionSync(
			t.Context(),
			RunBankConnectionSyncParams{ConnectionID: stored.ID, JobID: "job-3"},
		)
		require.Error(t, err)

		provider.syncErr = errors.New("sync failed")
		_, err = service.RunBankConnectionSync(
			t.Context(),
			RunBankConnectionSyncParams{ConnectionID: connection.ID, JobID: "job-4"},
		)
		require.Error(t, err)
		provider.syncErr = nil

		store := makeStore(t)
		serviceWithoutCipher := NewService(store)
		_, err = serviceWithoutCipher.encryptAndSaveConnectionSecret(
			t.Context(),
			"provider",
			"ref",
			"plain-secret",
		)
		require.Error(t, err)
		_, err = serviceWithoutCipher.decryptConnectionSecret(t.Context(), "secret-id")
		require.Error(t, err)

		badCipherService := NewService(store, WithConnectionSecretCipher(makeCipher(t)))
		_, err = badCipherService.decryptConnectionSecret(t.Context(), "missing-secret")
		require.Error(t, err)
	})

	t.Run("covers helpers and successful schedule update path", func(t *testing.T) {
		assert.Equal(t, "a", firstNonEmpty("", "a", "b"))
		assert.Empty(t, firstNonEmpty("", "   "))
		assert.Nil(t, timePtrUTC(time.Time{}))
		require.NotNil(t, timePtrUTC(time.Now()))
		assert.NotEmpty(t, providerFingerprint("a", 1, "b"))

		fake := faker.New()
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
			syncResults: []ProviderSyncResult{{
				SyncKey: "sync-" + fake.UUID().V4(),
				Accounts: []ProviderNormalizedAccount{{
					ProviderAccountID: "provider-account-" + fake.UUID().V4(),
					Name:              "main",
					Currency:          "USD",
					CurrentBalanceMinor: func() *int64 {
						value := int64(100)
						return &value
					}(),
				}},
			}},
		}
		enqueuer := &capturedBankSyncJobEnqueuer{}
		service, tenant, ownerUserID := makeService(t, provider, WithBankSyncJobEnqueuer(enqueuer))
		connection, err := service.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    provider.name,
				Token:       "token",
			},
		)
		require.NoError(t, err)
		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Interval:     time.Hour,
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.NoError(t, err)
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
		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Interval:     2 * time.Hour,
				NextRunAt:    time.Now().UTC().Add(time.Hour),
			},
		)
		require.NoError(t, err)
	})

	t.Run("keeps bank provider lookups explicit", func(t *testing.T) {
		service, _, _ := makeService(t, &stubBankProvider{name: "monobank"})
		_, err := service.bankProvider("missing-provider")
		require.Error(t, err)
	})

	t.Run("enforces explicit bank provider linking contract", func(t *testing.T) {
		fake := faker.New()
		monobankProvider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "Monobank " + fake.Company().Name(),
				ProviderReference: "mono-ref-" + fake.UUID().V4(),
				ExternalID:        "mono-external-" + fake.UUID().V4(),
				Secret:            "mono-secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		enableBankingProvider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "pko-state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.test/auth",
				ProviderReference: "pko-ref-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "pko-session-" + fake.UUID().V4(),
				ExternalID:        "pko-external-" + fake.UUID().V4(),
				Secret:            "pko-secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		service, tenant, ownerUserID := makeService(t, nil, WithBankProviders(monobankProvider, enableBankingProvider))

		monobankToken := "mono-token-" + fake.UUID().V4()
		connection, err := service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "monobank",
			Token:       monobankToken,
		})
		require.NoError(t, err)
		assert.Equal(t, "monobank", connection.Provider)
		assert.Equal(t, domain.ProviderConnectorIDMonobank, connection.ConnectorID)
		assert.Equal(t, []string{monobankToken}, monobankProvider.linkedTokens)

		pkoToken := "pko-token-" + fake.UUID().V4()
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			Token:       pkoToken,
		})
		require.ErrorContains(t, err, "token linking unsupported for bank provider: pko")
		assert.NotContains(t, err.Error(), pkoToken)
		assert.Empty(t, enableBankingProvider.linkedTokens)

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example.test/#/finance/connections",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, enableBankingProvider.startCalls)
		assert.Zero(t, monobankProvider.startCalls)
		assert.Equal(t, enableBankingProvider.startResult.ProviderReference, start.ProviderReference)

		redirectConnection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "pko-code-" + fake.UUID().V4(),
			Start:       start,
		})
		require.NoError(t, err)
		assert.Equal(t, "pko", redirectConnection.Provider)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, redirectConnection.ConnectorID)
		assert.Equal(t, 1, enableBankingProvider.finishCalls)

		monobankRedirectURL := "https://app.example.test/callback?secret=" + fake.UUID().V4()
		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "monobank",
			RedirectURL: monobankRedirectURL,
		})
		require.ErrorContains(t, err, "redirect linking unsupported for bank provider: monobank")
		assert.NotContains(t, err.Error(), monobankRedirectURL)
		assert.Zero(t, monobankProvider.startCalls)

		unsupportedProvider := "unsupported-" + fake.UUID().V4()
		unsupportedToken := "unsupported-token-" + fake.UUID().V4()
		_, err = service.LinkTokenBankConnection(t.Context(), LinkTokenBankConnectionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    unsupportedProvider,
			Token:       unsupportedToken,
		})
		require.ErrorContains(t, err, "unsupported bank provider")
		assert.NotContains(t, err.Error(), unsupportedToken)
		assert.NotContains(t, err.Error(), "enable-banking")
	})

	t.Run("routes pko sync through enable banking connector", func(t *testing.T) {
		fake := faker.New()
		enableBankingProvider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "pko-state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.test/auth",
				ProviderReference: "pko-ref-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "pko-session-" + fake.UUID().V4(),
				ExternalID:        "pko-external-" + fake.UUID().V4(),
				Secret:            "pko-secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
			syncResults: []ProviderSyncResult{{
				SyncKey: "sync-" + fake.UUID().V4(),
				Accounts: []ProviderNormalizedAccount{{
					ProviderAccountID: "provider-account-" + fake.UUID().V4(),
					Name:              "PKO Main " + fake.Lorem().Word(),
					Currency:          "PLN",
				}},
			}},
		}
		service, tenant, ownerUserID := makeService(t, nil, WithBankProviders(enableBankingProvider))

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example.test/#/finance/connections",
		})
		require.NoError(t, err)

		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-" + fake.UUID().V4(),
		})
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, connection.ConnectorID)

		_, err = service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
			WindowStart:  time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			WindowEnd:    time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		require.Len(t, enableBankingProvider.secrets, 1)
	})

	t.Run("resolves persisted redirect starts by scoped state and consumes them once", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.June, 22, 14, 0, 0, 0, time.UTC)
		currentNow := now
		provider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "pko-state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.test/auth/" + fake.UUID().V4(),
				ProviderReference: "pko-ref-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "pko-session-" + fake.UUID().V4(),
				ExternalID:        "pko-external-" + fake.UUID().V4(),
				Secret:            "pko-secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		service, tenant, ownerUserID := makeService(
			t,
			nil,
			WithNow(func() time.Time { return currentNow }),
			WithBankProviders(provider),
		)

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   "member-" + fake.Internet().Email(),
		})
		require.NoError(t, err)
		memberUserID := "user-member-" + fake.UUID().V4()
		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: memberUserID,
			Code:        invite.Code,
		})
		require.NoError(t, err)

		otherTenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-other-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example.test/#/finance/connections",
		})
		require.NoError(t, err)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: memberUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-member-" + fake.UUID().V4(),
			Start: ProviderLinkStart{
				State:             start.State,
				ProviderReference: "tampered-member-" + fake.UUID().V4(),
			},
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Zero(t, provider.finishCalls)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    otherTenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-tenant-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Zero(t, provider.finishCalls)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       "missing-state-" + fake.UUID().V4(),
			Code:        "code-state-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		assert.Zero(t, provider.finishCalls)

		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-owner-" + fake.UUID().V4(),
			Start: ProviderLinkStart{
				State:             start.State,
				ProviderReference: "tampered-owner-" + fake.UUID().V4(),
			},
		})
		require.NoError(t, err)
		require.Len(t, provider.finishParams, 1)
		assert.Equal(t, provider.startResult.ProviderReference, provider.finishParams[0].Start.ProviderReference)
		assert.NotEqual(t, provider.finishResult.Secret, connection.SecretID)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        "code-duplicate-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		require.Len(t, provider.finishParams, 1)

		provider.startResult.State = "state-expired-" + fake.UUID().V4()
		provider.startResult.ProviderReference = "ref-expired-" + fake.UUID().V4()

		startExpired, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://app.example.test/#/finance/connections?expired=1",
		})
		require.NoError(t, err)

		currentNow = currentNow.Add(pendingBankConnectionLinkStartTTL + time.Minute)
		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       startExpired.State,
			Code:        "code-expired-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
		require.Len(t, provider.finishParams, 1)
	})

	t.Run("loads pending redirect starts by provider state for callback handoff", func(t *testing.T) {
		fake := faker.New()
		provider := &stubBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "pko-state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.test/auth/" + fake.UUID().V4(),
				ProviderReference: "pko-ref-" + fake.UUID().V4(),
			},
		}
		service, tenant, ownerUserID := makeService(t, nil, WithBankProviders(provider))

		_, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID:        ownerUserID,
			TenantID:           tenant.ID,
			Provider:           "pko",
			RedirectURL:        "https://backend.example.test/enable-banking/callback",
			BrowserCallbackURL: "http://localhost:5173/#/finance/connections",
		})
		require.NoError(t, err)

		pendingStart, err := service.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: "pko",
				State:    provider.startResult.State,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:5173/#/finance/connections", pendingStart.CallbackURL)

		_, err = service.GetPendingBankConnectionLinkStartByState(
			t.Context(),
			GetPendingBankConnectionLinkStartByStateParams{
				Provider: "pko",
				State:    "missing-state-" + fake.UUID().V4(),
			},
		)
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)
	})

	t.Run("keeps helper ids safe for nil values", func(t *testing.T) {
		assert.Empty(t, accountID(nil))
		assert.Empty(t, matchID(nil))
		assert.NotNil(t, defaultLogger())
		assert.Contains(t, fmt.Sprintf("%T", defaultLogger()), "Logger")
	})

	t.Run("covers redirect link and scheduled metadata helper branches", func(t *testing.T) {
		fake := faker.New()
		provider := &redirectBankProvider{
			name: "enable-banking",
			startResult: ProviderLinkStart{
				State:             "state-" + fake.UUID().V4(),
				AuthorizationURL:  "https://example.com/auth",
				ProviderReference: "ref-" + fake.UUID().V4(),
			},
			finishResult: ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref-" + fake.UUID().V4(),
				ExternalID:        "external-" + fake.UUID().V4(),
				Secret:            "secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		}
		service, tenant, ownerUserID := makeService(t, provider)
		var err error
		finishCode := "finish-" + fake.UUID().V4()
		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "missing-" + fake.UUID().V4(),
			RedirectURL: "https://example.com/callback",
		})
		require.Error(t, err)
		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "missing-" + fake.UUID().V4(),
		})
		require.Error(t, err)

		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: "outsider-" + fake.UUID().V4(),
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://example.com/callback",
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		provider.startErr = errors.New("start failed")
		_, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://example.com/callback",
		})
		require.Error(t, err)
		provider.startErr = nil

		start, err := service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://example.com/callback",
		})
		require.NoError(t, err)

		provider.finishErr = errors.New("finish failed")
		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        finishCode,
			Start:       start,
		})
		require.Error(t, err)
		provider.finishErr = nil

		connection, err := service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        finishCode,
			Start:       start,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, connection.ID)

		_, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        finishCode,
			Start:       start,
		})
		require.ErrorIs(t, err, ErrPendingBankConnectionLinkStartNotFound)

		storeRef := service.store.(*persistence.Store)
		failingService := NewService(
			&failingProviderSyncStore{
				Store:                  storeRef,
				restorePendingStartErr: errors.New("restore pending start failed"),
			},
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		provider.startResult.State = "state-restore-failure-" + fake.UUID().V4()
		provider.startResult.ProviderReference = "ref-restore-failure-" + fake.UUID().V4()

		startRestoreFailure, err := failingService.StartBankConnectionLink(
			t.Context(),
			StartBankConnectionLinkParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    "pko",
				RedirectURL: "https://example.com/callback?restore-failure=1",
			},
		)
		require.NoError(t, err)
		provider.finishErr = errors.New("finish failed again")
		_, err = failingService.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       startRestoreFailure.State,
			Code:        finishCode,
			Start:       startRestoreFailure,
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "finish bank connection link")
		require.ErrorContains(t, err, "restore pending bank connection link start")
		provider.finishErr = nil

		provider.startResult.State = "state-retry-" + fake.UUID().V4()
		provider.startResult.ProviderReference = "ref-retry-" + fake.UUID().V4()

		start, err = service.StartBankConnectionLink(t.Context(), StartBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			RedirectURL: "https://example.com/callback?retry=1",
		})
		require.NoError(t, err)

		connection, err = service.FinishBankConnectionLink(t.Context(), FinishBankConnectionLinkParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Provider:    "pko",
			State:       start.State,
			Code:        finishCode,
			Start:       start,
		})
		require.NoError(t, err)

		syncStore, err := service.bankSyncStore()
		require.NoError(t, err)
		metadata, ok, err := service.makeScheduledRunMetadata(
			t.Context(),
			syncStore,
			connection,
			RunBankConnectionSyncParams{Reason: BankConnectionSyncReasonManual},
			time.Now().UTC(),
		)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, metadata)

		metadata, ok, err = service.makeScheduledRunMetadata(
			t.Context(),
			syncStore,
			connection,
			RunBankConnectionSyncParams{Reason: BankConnectionSyncReasonScheduled},
			time.Now().UTC(),
		)
		require.NoError(t, err)
		assert.True(t, ok)
		require.NotNil(t, metadata)

		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: connection.ID,
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC().Add(time.Hour),
		})
		require.NoError(t, err)

		scheduleFailingService := NewService(
			service.store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		failingSyncStore, err := scheduleFailingService.bankSyncStore()
		require.NoError(t, err)
		loadedConnection, err := failingSyncStore.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, err)
		scheduleStoreRef := service.store.(*persistence.Store)
		failingStore := &failingProviderSyncStore{
			Store:           scheduleStoreRef,
			saveScheduleErr: errors.New("schedule save failed"),
		}
		scheduleFailingService = NewService(
			failingStore,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		failingSyncStore, err = scheduleFailingService.bankSyncStore()
		require.NoError(t, err)
		err = scheduleFailingService.markBankConnectionSyncStarted(
			t.Context(),
			failingSyncStore,
			loadedConnection,
			RunBankConnectionSyncParams{JobID: "job-1"},
			time.Now().UTC(),
			metadata,
		)
		require.Error(t, err)
		err = scheduleFailingService.recordBankConnectionSyncFailure(
			t.Context(),
			failingSyncStore,
			loadedConnection,
			RunBankConnectionSyncParams{JobID: "job-2"},
			time.Now().UTC(),
			metadata,
			errors.New("sync failed"),
		)
		require.Error(t, err)

		store := &failingProviderSyncStore{Store: makeStore(t), getScheduleErr: errors.New("schedule read failed")}
		serviceWithFailingStore := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		_, _, err = serviceWithFailingStore.makeScheduledRunMetadata(
			t.Context(),
			&bankSyncStoreRef{bankSyncStore: store},
			connection,
			RunBankConnectionSyncParams{Reason: BankConnectionSyncReasonScheduled},
			time.Now().UTC(),
		)
		require.Error(t, err)
	})

	t.Run("covers provider sync error branches with failing store seams", func(t *testing.T) {
		fake := faker.New()
		baseStore := makeStore(t)
		store := &failingProviderSyncStore{Store: baseStore}
		provider := &stubBankProvider{
			name: "monobank",
			linkResult: ProviderTokenLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		}
		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(
			t.Context(),
			CreateTenantParams{
				ActorUserID:     ownerUserID,
				Name:            "tenant-" + fake.Company().Name(),
				DisplayCurrency: "USD",
			},
		)
		require.NoError(t, err)
		connection, err := service.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    provider.name,
				Token:       "token",
			},
		)
		require.NoError(t, err)

		store.getScheduleErr = errors.New("schedule read failed")
		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Interval:     time.Hour,
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.Error(t, err)
		store.getScheduleErr = nil

		_, err = service.PauseBankConnectionSchedule(
			t.Context(),
			PauseBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: "missing",
			},
		)
		require.Error(t, err)
		_, err = service.ResumeBankConnectionSchedule(
			t.Context(),
			ResumeBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: "missing",
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.Error(t, err)
		_, err = service.TriggerBankConnectionSync(
			t.Context(),
			TriggerBankConnectionSyncParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: "missing",
				Reason:       BankConnectionSyncReasonManual,
			},
		)
		require.Error(t, err)

		store.getSyncRunErr = errors.New("sync run failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				Result:       ProviderSyncResult{SyncKey: "sync-key"},
			},
		)
		require.Error(t, err)
		store.getSyncRunErr = nil

		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{ConnectionID: "missing", Result: ProviderSyncResult{}},
		)
		require.ErrorIs(t, err, ErrBankConnectionNotFound)

		store.saveConnectionSecretErr = errors.New("secret save failed")
		_, err = service.encryptAndSaveConnectionSecret(t.Context(), provider.name, "ref", "secret")
		require.Error(t, err)
		store.saveConnectionSecretErr = nil
		store.getConnectionSecretErr = errors.New("secret read failed")
		_, err = service.decryptConnectionSecret(t.Context(), connection.SecretID)
		require.Error(t, err)
		store.getConnectionSecretErr = nil

		store.saveProviderAccountErr = errors.New("save provider account failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				Result: ProviderSyncResult{
					Accounts: []ProviderNormalizedAccount{
						{ProviderAccountID: "provider-account-0", Name: "main", Currency: "USD"},
					},
				},
			},
		)
		require.Error(t, err)
		store.saveProviderAccountErr = nil

		store.saveSnapshotErr = errors.New("snapshot failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				Result: ProviderSyncResult{
					Accounts: []ProviderNormalizedAccount{
						{
							ProviderAccountID: "provider-account-1",
							Name:              "main",
							Currency:          "USD",
							CurrentBalanceMinor: func() *int64 {
								value := int64(1)
								return &value
							}(),
						},
					},
				},
			},
		)
		require.Error(t, err)
		store.saveSnapshotErr = nil

		store.saveRawPayloadErr = errors.New("raw payload failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				Result: ProviderSyncResult{
					RawPayloads: []ProviderRawPayload{
						{
							Scope:            domain.RawPayloadScopeConnection,
							ProviderObjectID: "obj",
							PayloadJSON:      []byte(`{"ok":true}`),
						},
					},
				},
			},
		)
		require.Error(t, err)
		store.saveRawPayloadErr = nil

		store.saveBankConnectionErr = errors.New("save connection failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				Result:       ProviderSyncResult{},
			},
		)
		require.Error(t, err)
		store.saveBankConnectionErr = nil

		store.getScheduleErr = errors.New("schedule read failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				JobID:        "job-schedule-read-fail",
				Result:       ProviderSyncResult{},
			},
		)
		require.ErrorContains(t, err, "get bank connection schedule")
		store.getScheduleErr = nil

		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connection.ID,
				Interval:     time.Hour,
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		store.saveScheduleErr = errors.New("schedule save failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				JobID:        "job-1",
				Result:       ProviderSyncResult{},
			},
		)
		require.Error(t, err)
		store.saveScheduleErr = nil

		store.saveSyncRunErr = errors.New("sync run save failed")
		_, err = service.ApplyProviderSyncResult(
			t.Context(),
			ApplyProviderSyncResultParams{
				ConnectionID: connection.ID,
				JobID:        "job-2",
				Result:       ProviderSyncResult{SyncKey: "sync-save-fail"},
			},
		)
		require.Error(t, err)
		store.saveSyncRunErr = nil

		store.listBankConnectionsErr = errors.New("list bank connections failed")
		_, err = service.ListBankConnections(
			t.Context(),
			ListBankConnectionsParams{ActorUserID: ownerUserID, TenantID: tenant.ID},
		)
		require.Error(t, err)
		store.listBankConnectionsErr = nil
		store.getScheduleForListErr = errors.New("list schedule failed")
		_, err = service.ListBankConnections(
			t.Context(),
			ListBankConnectionsParams{ActorUserID: ownerUserID, TenantID: tenant.ID},
		)
		require.Error(t, err)
		store.getScheduleForListErr = nil

		serviceWithoutCipher := NewService(store)
		err = saveLinkedConnectionForTest(
			t.Context(),
			serviceWithoutCipher,
			tenant.ID,
			provider.name,
			ProviderLinkResult{},
		)
		require.Error(t, err)

		store.saveBankConnectionErr = errors.New("save linked connection failed")
		err = saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{Secret: "secret"},
		)
		require.Error(t, err)
		store.saveBankConnectionErr = nil
		store.saveRawPayloadErr = errors.New("save linked raw failed")
		err = saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				Secret: "secret",
				RawPayloads: []ProviderRawPayload{
					{
						Scope:            domain.RawPayloadScopeConnection,
						ProviderObjectID: "obj",
						PayloadJSON:      []byte(`{"ok":true}`),
					},
				},
			},
		)
		require.Error(t, err)
		store.saveRawPayloadErr = nil

		badCipher, cipherErr := credentials.NewAESGCMCipher(
			[]byte("0123456789abcdef0123456789abcdef"),
			"bad-key",
		)
		require.NoError(t, cipherErr)
		sealed, err := badCipher.SealString("secret")
		require.NoError(t, err)
		_, err = baseStore.SaveConnectionSecret(
			t.Context(),
			domain.ConnectionSecret{
				ID:        "bad-secret",
				Provider:  provider.name,
				Reference: "ref",
				Envelope:  sealed,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		)
		require.NoError(t, err)
		differentCipher, differentCipherErr := credentials.NewAESGCMCipher(
			[]byte("abcdef0123456789abcdef0123456789"),
			"different-key",
		)
		require.NoError(t, differentCipherErr)
		serviceWithDifferentCipher := NewService(
			baseStore,
			WithConnectionSecretCipher(differentCipher),
		)
		_, err = serviceWithDifferentCipher.decryptConnectionSecret(t.Context(), "bad-secret")
		require.Error(t, err)

		store.listAccountsErr = errors.New("accounts failed")
		_, err = service.upsertProviderAccount(
			t.Context(),
			connection,
			ProviderNormalizedAccount{
				ProviderAccountID: "provider-account-list-fail",
				Name:              "main",
				Currency:          "USD",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.listAccountsErr = nil

		providerAccount, err := service.upsertProviderAccount(
			t.Context(),
			connection,
			ProviderNormalizedAccount{
				ProviderAccountID: "provider-account-2",
				Name:              "main",
				Currency:          "USD",
			},
			time.Now().UTC(),
		)
		require.NoError(t, err)
		store.listProviderAccountsErr = errors.New("list provider accounts failed")
		_, err = service.upsertProviderAccount(
			t.Context(),
			connection,
			ProviderNormalizedAccount{
				ProviderAccountID: "provider-account-3",
				Name:              "main",
				Currency:          "USD",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.listProviderAccountsErr = nil

		linkedAccount, err := service.findOrCreateFinanceAccountForProviderAccount(
			t.Context(),
			connection,
			ProviderNormalizedAccount{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Name:              "main",
				Currency:          "USD",
			},
			time.Now().UTC(),
		)
		require.NoError(t, err)
		assert.Equal(t, providerAccount.FinanceAccountID, linkedAccount.ID)
		store.getMatchByFingerprintErr = errors.New("match failed")
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       1,
				Currency:          "USD",
				Description:       "desc",
				EffectiveAt:       time.Now().UTC(),
				Fingerprint:       "fp",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.getMatchByFingerprintErr = nil
		store.getMatchByProviderIDErr = errors.New("provider match failed")
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID:     providerAccount.ProviderAccountID,
				ProviderTransactionID: "provider-txn-1",
				Status:                domain.TransactionStatusBooked,
				AmountMinor:           1,
				Currency:              "USD",
				Description:           "desc",
				EffectiveAt:           time.Now().UTC(),
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.getMatchByProviderIDErr = nil

		store.saveTransactionErr = errors.New("save transaction failed")
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       1,
				Currency:          "USD",
				Description:       "desc",
				EffectiveAt:       time.Now().UTC(),
				Fingerprint:       "fp-2",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.saveTransactionErr = nil
		store.getTransactionErr = errors.New("get transaction failed")
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       1,
				Currency:          "USD",
				Description:       "desc",
				EffectiveAt:       time.Now().UTC(),
				Fingerprint:       "fp-4",
			},
			time.Now().UTC(),
		)
		require.NoError(t, err)
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       1,
				Currency:          "USD",
				Description:       "desc",
				EffectiveAt:       time.Now().UTC(),
				Fingerprint:       "fp-4",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
		store.getTransactionErr = nil

		store.saveTransactionMatchErr = errors.New("save match failed")
		_, err = service.applyProviderTransaction(
			t.Context(),
			connection,
			providerAccount,
			ProviderNormalizedTransaction{
				ProviderAccountID: providerAccount.ProviderAccountID,
				Status:            domain.TransactionStatusBooked,
				AmountMinor:       1,
				Currency:          "USD",
				Description:       "desc",
				EffectiveAt:       time.Now().UTC(),
				Fingerprint:       "fp-3",
			},
			time.Now().UTC(),
		)
		require.Error(t, err)
	})
}
