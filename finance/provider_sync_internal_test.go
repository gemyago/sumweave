package finance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	providers "github.com/gemyago/sumweave/finance/internal/providers"
	internalsynthetic "github.com/gemyago/sumweave/finance/internal/synthetic"
	"github.com/gemyago/sumweave/finance/persistence"
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

func TestProviderSyncInternals(t *testing.T) {
	t.Run("optional sync windows reject invalid present timestamps", func(t *testing.T) {
		valid := time.Date(2026, time.July, 10, 8, 30, 0, 123, time.FixedZone("request", 5*60*60+30*60))
		require.NoError(t, validateBankConnectionSyncWindows(nil, nil))
		require.NoError(t, validateBankConnectionSyncWindows(&valid, &valid))

		zero := time.Time{}
		require.ErrorContains(t, validateBankConnectionSyncWindows(&zero, nil), "windowStart must be non-zero")
	})

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
				SeedDefaults:    true,
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
	) (domain.BankConnection, error) {
		secretID, err := service.encryptAndSaveConnectionSecret(
			ctx,
			providerName,
			result.ProviderReference,
			result.Secret,
		)
		if err != nil {
			return domain.BankConnection{}, err
		}
		syncStore, err := service.bankSyncStore()
		if err != nil {
			return domain.BankConnection{}, err
		}
		now := service.now().UTC()
		connection := domain.BankConnection{
			ID:                service.newID(),
			TenantID:          tenantID,
			Provider:          providerName,
			ProviderReference: result.ProviderReference,
			SecretID:          secretID,
			State:             result.State,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		saved, err := syncStore.SaveBankConnection(ctx, connection)
		if err != nil {
			return domain.BankConnection{}, err
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
				return domain.BankConnection{}, rawErr
			}
		}
		return saved, nil
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
		assert.Nil(t, timePtrOrNil(time.Time{}))
		now := time.Date(2026, time.June, 23, 10, 0, 0, 0, time.FixedZone("CET", 2*60*60))
		resolved := timePtrOrNil(now)
		require.NotNil(t, resolved)
		assert.Equal(t, now, *resolved)
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

	t.Run("refreshes existing provider-ID fallback linked account names on sync", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		service := NewService(store, WithConnectionSecretCipher(makeCipher(t)))
		now := time.Date(2026, time.July, 7, 18, 45, 0, 0, time.UTC)
		service.now = func() time.Time { return now }

		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		financeAccountID := "finance-account-" + fake.UUID().V4()
		readableName := "provider-name-" + fake.Lorem().Word()

		_, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                connectionID,
			TenantID:          tenantID,
			Provider:          string(domain.ProviderIDPKO),
			ProviderReference: "provider-reference-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)
		_, err = store.SaveAccount(t.Context(), domain.Account{
			ID:       financeAccountID,
			TenantID: tenantID,
			Name:     providerAccountID,
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(domain.ProviderIDPKO),
				ProviderAccountID: providerAccountID,
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		})
		require.NoError(t, err)
		_, err = store.SaveConnectionProviderAccount(t.Context(), domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connectionID,
			ProviderAccountID: providerAccountID,
			FinanceAccountID:  financeAccountID,
			Name:              providerAccountID,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)

		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connectionID,
			Result: ProviderSyncResult{Accounts: []ProviderNormalizedAccount{{
				ProviderAccountID: providerAccountID,
				Name:              readableName,
				Currency:          "EUR",
			}}},
		})
		require.NoError(t, err)

		loaded, err := store.GetAccount(t.Context(), financeAccountID)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, readableName, loaded.Name)
		assert.Equal(t, "EUR", loaded.Currency)
	})

	t.Run("isolates identical provider account IDs across same-provider connections", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		evidenceStore := persistence.NewProviderEvidenceStoreFromStore(store)
		now := time.Date(2026, time.August, 10, 14, 30, 0, 0, time.FixedZone("test", 2*60*60))
		service := NewBankSyncService(
			store,
			WithBankSyncServiceNow(func() time.Time { return now }),
			WithBankSyncServiceEvidenceWriter(evidenceStore),
		)
		tenantID := "tenant-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		makeConnection := func(reference string) domain.BankConnection {
			return domain.BankConnection{
				ID:                "connection-" + fake.UUID().V4(),
				TenantID:          tenantID,
				Provider:          string(domain.ProviderIDPKO),
				ConnectorID:       domain.ProviderConnectorIDEnableBanking,
				ProviderReference: reference,
				State:             domain.BankConnectionStateActive,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
		}
		firstConnection, err := store.SaveBankConnection(t.Context(), makeConnection("reference-"+fake.UUID().V4()))
		require.NoError(t, err)
		secondConnection, err := store.SaveBankConnection(t.Context(), makeConnection("reference-"+fake.UUID().V4()))
		require.NoError(t, err)
		makeResult := func(providerTransactionID string, balance int64) ProviderSyncResult {
			return ProviderSyncResult{
				SyncKey: "sync-" + fake.UUID().V4(),
				Accounts: []ProviderNormalizedAccount{{
					ProviderAccountID:   providerAccountID,
					Name:                "Account " + fake.Lorem().Word(),
					Currency:            "PLN",
					CurrentBalanceMinor: &balance,
				}},
				Transactions: []ProviderNormalizedTransaction{{
					ProviderAccountID:     providerAccountID,
					ProviderTransactionID: providerTransactionID,
					Status:                domain.TransactionStatusBooked,
					AmountMinor:           balance,
					Currency:              "PLN",
					Description:           "Transaction " + fake.Lorem().Word(),
					EffectiveAt:           now,
					Fingerprint:           "fingerprint-" + fake.UUID().V4(),
					RawPayloadJSON:        []byte(`{"transaction":"` + providerTransactionID + `"}`),
				}},
				RawPayloads: []ProviderRawPayload{{
					Scope:            domain.RawPayloadScopeAccount,
					ProviderObjectID: providerAccountID,
					PayloadJSON:      []byte(`{"account":"` + providerAccountID + `"}`),
				}},
			}
		}
		firstTransactionID := "transaction-" + fake.UUID().V4()
		secondTransactionID := "transaction-" + fake.UUID().V4()
		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: firstConnection.ID,
			Result:       makeResult(firstTransactionID, 100),
		})
		require.NoError(t, err)
		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: secondConnection.ID,
			Result:       makeResult(secondTransactionID, 200),
		})
		require.NoError(t, err)

		firstMappings, err := store.ListConnectionProviderAccounts(t.Context(), firstConnection.ID)
		require.NoError(t, err)
		secondMappings, err := store.ListConnectionProviderAccounts(t.Context(), secondConnection.ID)
		require.NoError(t, err)
		require.Len(t, firstMappings, 1)
		require.Len(t, secondMappings, 1)
		assert.Equal(t, providerAccountID, firstMappings[0].ProviderAccountID)
		assert.Equal(t, providerAccountID, secondMappings[0].ProviderAccountID)
		assert.NotEqual(t, firstMappings[0].FinanceAccountID, secondMappings[0].FinanceAccountID)

		firstSnapshots, err := store.ListBalanceSnapshots(t.Context(), firstConnection.ID)
		require.NoError(t, err)
		secondSnapshots, err := store.ListBalanceSnapshots(t.Context(), secondConnection.ID)
		require.NoError(t, err)
		require.Len(t, firstSnapshots, 1)
		require.Len(t, secondSnapshots, 1)
		assert.Equal(t, firstConnection.ID, firstSnapshots[0].ConnectionID)
		assert.Equal(t, firstMappings[0].FinanceAccountID, firstSnapshots[0].FinanceAccountID)
		assert.Equal(t, secondConnection.ID, secondSnapshots[0].ConnectionID)
		assert.Equal(t, secondMappings[0].FinanceAccountID, secondSnapshots[0].FinanceAccountID)

		firstMatch, err := store.GetProviderTransactionMatchByProviderID(
			t.Context(), firstConnection.ID, providerAccountID, firstTransactionID,
		)
		require.NoError(t, err)
		secondMatch, err := store.GetProviderTransactionMatchByProviderID(
			t.Context(), secondConnection.ID, providerAccountID, secondTransactionID,
		)
		require.NoError(t, err)
		assert.Equal(t, firstConnection.ID, firstMatch.ConnectionID)
		assert.Equal(t, secondConnection.ID, secondMatch.ConnectionID)
		assert.NotEqual(t, firstMatch.TransactionID, secondMatch.TransactionID)

		firstAccountEvidence, err := evidenceStore.ListAccountProviderEvidence(
			t.Context(), tenantID, firstMappings[0].FinanceAccountID,
		)
		require.NoError(t, err)
		secondAccountEvidence, err := evidenceStore.ListAccountProviderEvidence(
			t.Context(), tenantID, secondMappings[0].FinanceAccountID,
		)
		require.NoError(t, err)
		require.Len(t, firstAccountEvidence, 1)
		require.Len(t, secondAccountEvidence, 1)
		assert.Equal(t, firstConnection.ID, firstAccountEvidence[0].ConnectionID)
		assert.Equal(t, firstMappings[0].FinanceAccountID, firstAccountEvidence[0].FinanceAccountID)
		assert.Equal(t, secondConnection.ID, secondAccountEvidence[0].ConnectionID)
		assert.Equal(t, secondMappings[0].FinanceAccountID, secondAccountEvidence[0].FinanceAccountID)

		firstTransactionEvidence, err := evidenceStore.ListTransactionProviderEvidence(
			t.Context(), tenantID, firstMatch.TransactionID,
		)
		require.NoError(t, err)
		secondTransactionEvidence, err := evidenceStore.ListTransactionProviderEvidence(
			t.Context(), tenantID, secondMatch.TransactionID,
		)
		require.NoError(t, err)
		require.Len(t, firstTransactionEvidence, 1)
		require.Len(t, secondTransactionEvidence, 1)
		assert.Equal(t, firstConnection.ID, firstTransactionEvidence[0].ConnectionID)
		assert.Equal(t, firstMatch.TransactionID, firstTransactionEvidence[0].FinanceTransactionID)
		assert.Equal(t, secondConnection.ID, secondTransactionEvidence[0].ConnectionID)
		assert.Equal(t, secondMatch.TransactionID, secondTransactionEvidence[0].FinanceTransactionID)
	})

	t.Run("preserves custom linked finance account names during provider metadata refresh", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		service := NewService(store, WithConnectionSecretCipher(makeCipher(t)))
		now := time.Date(2026, time.July, 7, 18, 50, 0, 0, time.UTC)
		service.now = func() time.Time { return now }

		tenantID := "tenant-" + fake.UUID().V4()
		connectionID := "connection-" + fake.UUID().V4()
		providerAccountID := "provider-account-" + fake.UUID().V4()
		financeAccountID := "finance-account-" + fake.UUID().V4()
		customName := "custom-name-" + fake.Lorem().Word()

		_, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID:                connectionID,
			TenantID:          tenantID,
			Provider:          string(domain.ProviderIDPKO),
			ProviderReference: "provider-reference-" + fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)
		_, err = store.SaveAccount(t.Context(), domain.Account{
			ID:       financeAccountID,
			TenantID: tenantID,
			Name:     customName,
			Currency: "USD",
			Kind:     domain.AccountKindLinked,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          string(domain.ProviderIDPKO),
				ProviderAccountID: providerAccountID,
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		})
		require.NoError(t, err)
		_, err = store.SaveConnectionProviderAccount(t.Context(), domain.ConnectionProviderAccount{
			ID:                "provider-account-row-" + fake.UUID().V4(),
			ConnectionID:      connectionID,
			ProviderAccountID: providerAccountID,
			FinanceAccountID:  financeAccountID,
			Name:              "old-provider-name-" + fake.Lorem().Word(),
			Currency:          "USD",
			CreatedAt:         now.Add(-time.Hour),
			UpdatedAt:         now.Add(-time.Hour),
		})
		require.NoError(t, err)

		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connectionID,
			Result: ProviderSyncResult{Accounts: []ProviderNormalizedAccount{{
				ProviderAccountID: providerAccountID,
				Name:              "new-provider-name-" + fake.Lorem().Word(),
				Currency:          "EUR",
			}}},
		})
		require.NoError(t, err)

		loaded, err := store.GetAccount(t.Context(), financeAccountID)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, customName, loaded.Name)
		assert.Equal(t, "EUR", loaded.Currency)
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

	t.Run("syncs synthetic linked connections through connector-backed bank sync path", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		syntheticStateStore := persistence.NewSyntheticProviderStateStoreFromStore(store)
		now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
		connector := internalsynthetic.NewConnector(
			syntheticStateStore,
			internalsynthetic.WithConnectorNow(func() time.Time { return now }),
		)
		provider, ok := newConnectorBankSyncProvider(connector)
		require.True(t, ok)

		service := NewService(
			store,
			WithConnectionSecretCipher(makeCipher(t)),
			WithBankProviders(provider),
			WithNow(func() time.Time { return now }),
		)
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		syncStore, err := service.bankSyncStore()
		require.NoError(t, err)

		linker := internalsynthetic.NewLinker(internalsynthetic.LinkerDeps{
			RequireTenantMember: service.requireTenantMember,
			SaveConnectionSecret: func(ctx context.Context, providerName, reference, secret string) (string, error) {
				return service.encryptAndSaveConnectionSecret(ctx, providerName, reference, secret)
			},
			DeleteConnectionSecret: store.DeleteConnectionSecret,
			SaveBankConnection:     syncStore.SaveBankConnection,
			DeleteBankConnectionOwnedMetadata: func(ctx context.Context, connection domain.BankConnection) error {
				return service.deleteBankConnectionOwnedMetadata(ctx, connection)
			},
			SaveSyntheticProviderState: syntheticStateStore.SaveSyntheticProviderState,
			Now:                        func() time.Time { return now },
			NewID:                      service.newID,
		})

		connection, err := linker.LinkConfiguredBankConnection(
			t.Context(),
			internalsynthetic.LinkConfiguredBankConnectionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Provider:    string(domain.ProviderIDSynthetic),
				Accounts: []internalsynthetic.ConfiguredAccount{{
					Name:     "Checking-" + fake.Lorem().Word(),
					Currency: "USD",
				}},
			},
		)
		require.NoError(t, err)

		windowStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
		windowEnd := time.Date(2026, time.June, 4, 0, 0, 0, 0, time.UTC)
		result, err := service.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID,
			JobID:        "job-" + fake.UUID().V4(),
			Reason:       BankConnectionSyncReasonManual,
			WindowStart:  &windowStart,
			WindowEnd:    &windowEnd,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedAccounts)
		assert.Positive(t, result.ImportedTransactions)

		accounts, err := store.ListConnectionProviderAccounts(t.Context(), connection.ID)
		require.NoError(t, err)
		require.Len(t, accounts, 1)
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
			syncErr: errors.New("sync failed"),
		}
		serviceWithProvider, tenant, _ := makeService(t, provider)
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			serviceWithProvider,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		_, err = serviceWithProvider.RunBankConnectionSync(
			t.Context(),
			RunBankConnectionSyncParams{ConnectionID: connection.ID, JobID: "job-1"},
		)
		require.Error(t, err)
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
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		connectionID := connection.ID

		_, err = service.PauseBankConnectionSchedule(
			t.Context(),
			PauseBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connectionID,
			},
		)
		require.Error(t, err)
		_, err = service.ResumeBankConnectionSchedule(
			t.Context(),
			ResumeBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connectionID,
				NextRunAt:    time.Now().UTC(),
			},
		)
		require.Error(t, err)
		_, err = service.TriggerBankConnectionSync(
			t.Context(),
			TriggerBankConnectionSyncParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connectionID,
				Reason:       BankConnectionSyncReasonManual,
			},
		)
		require.Error(t, err)
	})

	t.Run("surfaces sync application failures and missing cipher branches", func(t *testing.T) {
		fake := faker.New()
		provider := &stubBankProvider{name: "monobank"}
		service, tenant, _ := makeService(t, provider)
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		connectionID := connection.ID

		_, err = service.ApplyProviderSyncResult(t.Context(), ApplyProviderSyncResultParams{
			ConnectionID: connectionID,
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
			RunBankConnectionSyncParams{ConnectionID: connectionID, JobID: "job-4"},
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
		assert.Nil(t, timePtrOrNil(time.Time{}))
		require.NotNil(t, timePtrOrNil(time.Now()))
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
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		connectionID := connection.ID
		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connectionID,
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
				ConnectionID: connectionID,
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
				ConnectionID: connectionID,
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

	t.Run("keeps same-provider linked connections separate and lists their schedules", func(t *testing.T) {
		fake := faker.New()
		service, tenant, ownerUserID := makeService(t, nil)

		monobankConnection, err := service.saveLinkedBankConnection(
			t.Context(),
			tenant.ID,
			bankProviderMonobank,
			domain.ProviderConnectorIDMonobank,
			ProviderLinkResult{
				DisplayName:       "Monobank " + fake.Company().Name(),
				ProviderReference: "mono-ref-" + fake.UUID().V4(),
				Secret:            "mono-secret-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
				RawPayloads: []ProviderRawPayload{{
					Scope:            domain.RawPayloadScopeConnection,
					ProviderObjectID: "mono-payload-" + fake.UUID().V4(),
					PayloadJSON:      []byte(`{"provider":"monobank"}`),
				}},
			},
		)
		require.NoError(t, err)

		firstPKOConnection, err := service.saveLinkedBankConnection(
			t.Context(),
			tenant.ID,
			bankProviderPKO,
			domain.ProviderConnectorIDEnableBanking,
			ProviderLinkResult{
				DisplayName:       "PKO " + fake.Company().Name(),
				ProviderReference: "pko-ref-1-" + fake.UUID().V4(),
				Secret:            "pko-secret-1-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)

		secondPKOConnection, err := service.saveLinkedBankConnection(
			t.Context(),
			tenant.ID,
			bankProviderPKO,
			domain.ProviderConnectorIDEnableBanking,
			ProviderLinkResult{
				DisplayName:       "PKO again " + fake.Company().Name(),
				ProviderReference: "pko-ref-2-" + fake.UUID().V4(),
				Secret:            "pko-secret-2-" + fake.UUID().V4(),
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		assert.NotEqual(t, firstPKOConnection.ID, secondPKOConnection.ID)
		assert.NotEqual(t, firstPKOConnection.SecretID, secondPKOConnection.SecretID)
		assert.NotEqual(t, firstPKOConnection.ProviderReference, secondPKOConnection.ProviderReference)

		connections, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, connections, 3)

		_, err = service.UpsertBankConnectionSchedule(t.Context(), UpsertBankConnectionScheduleParams{
			ActorUserID:  ownerUserID,
			TenantID:     tenant.ID,
			ConnectionID: secondPKOConnection.ID,
			Interval:     time.Hour,
			NextRunAt:    time.Now().UTC().Add(time.Hour),
		})
		require.NoError(t, err)

		views, err := service.ListBankConnections(t.Context(), ListBankConnectionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, views, 3)
		viewsByID := map[string]BankConnectionView{}
		for _, view := range views {
			viewsByID[view.Connection.ID] = view
		}
		assert.Nil(t, viewsByID[monobankConnection.ID].Schedule)
		require.NotNil(t, viewsByID[secondPKOConnection.ID].Schedule)
		assert.Equal(t, secondPKOConnection.ID, viewsByID[secondPKOConnection.ID].Schedule.ConnectionID)
	})

	t.Run("keeps helper ids safe for nil values", func(t *testing.T) {
		assert.Empty(t, accountID(nil))
		assert.Empty(t, matchID(nil))
		assert.NotNil(t, defaultLogger())
		assert.Contains(t, fmt.Sprintf("%T", defaultLogger()), "Logger")

		serviceWithNonSyncStore := NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
		})
		_, err := serviceWithNonSyncStore.ListBankConnections(
			t.Context(),
			ListBankConnectionsParams{ActorUserID: "user-1", TenantID: "tenant-1"},
		)
		require.ErrorContains(t, err, "bank sync store is required")

		serviceWithNonSecretStore := NewService(stubStore{}, WithConnectionSecretCipher(makeCipher(t)))
		_, err = serviceWithNonSecretStore.connectionSecretsStore()
		require.ErrorContains(t, err, "connection secret store is required")
	})

	t.Run("covers scheduled metadata helper branches", func(t *testing.T) {
		fake := faker.New()
		provider := &stubBankProvider{
			name: "enable-banking",
			syncResults: []ProviderSyncResult{{
				SyncKey: "sync-" + fake.UUID().V4(),
			}},
		}
		service, tenant, ownerUserID := makeService(t, provider)
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
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
				SeedDefaults:    true,
			},
		)
		require.NoError(t, err)
		connection, err := saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{
				DisplayName:       "display",
				ProviderReference: "ref",
				Secret:            "secret",
				State:             domain.BankConnectionStateActive,
			},
		)
		require.NoError(t, err)
		connectionID := connection.ID

		store.getScheduleErr = errors.New("schedule read failed")
		_, err = service.UpsertBankConnectionSchedule(
			t.Context(),
			UpsertBankConnectionScheduleParams{
				ActorUserID:  ownerUserID,
				TenantID:     tenant.ID,
				ConnectionID: connectionID,
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
		_, err = saveLinkedConnectionForTest(
			t.Context(),
			serviceWithoutCipher,
			tenant.ID,
			provider.name,
			ProviderLinkResult{},
		)
		require.Error(t, err)

		store.saveBankConnectionErr = errors.New("save linked connection failed")
		_, err = saveLinkedConnectionForTest(
			t.Context(),
			service,
			tenant.ID,
			provider.name,
			ProviderLinkResult{Secret: "secret"},
		)
		require.Error(t, err)
		store.saveBankConnectionErr = nil
		store.saveRawPayloadErr = errors.New("save linked raw failed")
		_, err = saveLinkedConnectionForTest(
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
			&providerAccount,
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
