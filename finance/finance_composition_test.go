package finance

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	internalmonobank "github.com/gemyago/sumweave/finance/internal/monobank"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceComposition(t *testing.T) {
	fake := faker.New()

	t.Run("service options from config wire focused collaborators", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
		fxEnqueuer := &capturedFXRefreshJobEnqueuer{}
		fxWriter := &capturedFXRefreshScheduleWriter{}
		csvEnqueuer := &recordingCSVJobEnqueuer{}
		bankEnqueuer := &capturedBankSyncJobEnqueuer{}
		bankWriter := &capturedBankSyncScheduleWriter{}
		cipherKey := sha256.Sum256([]byte("finance-composition-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(cipherKey[:], "finance-composition")
		require.NoError(t, err)
		connectors := []internalproviders.Connector{internalmonobank.NewConnector(internalmonobank.Args{
			BaseURL:    "https://" + fake.Internet().Domain(),
			HTTPClient: http.DefaultClient,
			Logger:     slog.New(slog.DiscardHandler),
		})}

		serviceConfig := focusedServicesConfigFromConfig(&Config{
			Database:               database,
			Logger:                 slog.New(slog.DiscardHandler),
			Now:                    func() time.Time { return now },
			NewID:                  func() string { return "id-1" },
			HTTPClient:             http.DefaultClient,
			ConnectionSecretCipher: cipher,
			FXProviders:            []FXRatesProvider{NewStaticFXProvider("custom-fx", nil)},
			DefaultFXProvider:      "custom-fx",
			FXJobEnqueuer:          fxEnqueuer,
			FXScheduleWriter:       fxWriter,
			CSVImportJobEnqueuer:   csvEnqueuer,
			BankSyncJobEnqueuer:    bankEnqueuer,
			BankSyncScheduleWriter: bankWriter,
		}, connectors, newMockbankSyncOrchestrator(t))
		services := newFocusedServices(store, persistence.NewTransactionTagStore(database), serviceConfig)

		require.Same(t, fxEnqueuer, services.FXService.fxJobEnqueuer)
		require.Same(t, fxWriter, services.FXService.fxScheduleWriter)
		require.Same(t, csvEnqueuer, services.CSVImportService.csvImportJobEnqueuer)
		require.Same(t, bankEnqueuer, services.BankSyncService.bankSyncJobEnqueuer)
		require.Same(t, bankWriter, services.BankSyncService.bankSyncScheduleWriter)
		assert.Equal(t, "custom-fx", services.FXService.defaultFXProvider)
		assert.Contains(t, services.FXService.fxProviders, "custom-fx")
		assert.Contains(t, services.BankSyncService.bankProviders, "monobank")
	})

	t.Run("connector bank sync provider maps fetched observations", func(t *testing.T) {
		windowStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
		windowEnd := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.URL.Path == "/personal/client-info":
				assert.Equal(t, "mono-token", request.Header.Get("X-Token"))
				_, _ = writer.Write([]byte(
					`{"name":"Fixture","accounts":[{"id":"acc-1","type":"black","currencyCode":980,"balance":12345}]}`,
				))
			case strings.HasPrefix(request.URL.Path, "/personal/statement/acc-1/"):
				assert.Equal(t, "mono-token", request.Header.Get("X-Token"))
				_, _ = writer.Write([]byte(
					`[{"id":"txn-1","time":1717203600,"description":"coffee","currencyCode":980,"amount":-250}]`,
				))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		connector := internalmonobank.NewConnector(
			internalmonobank.Args{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				Logger:     slog.New(slog.DiscardHandler),
			},
			internalmonobank.WithSecretTokenResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
				return "mono-token", nil
			}),
		)
		provider, ok := newConnectorBankSyncProvider(connector)
		require.True(t, ok)

		result, err := provider.Sync(t.Context(), ProviderSyncParams{
			ProviderReference: "mono-ref-1",
			Secret:            "mono-token",
			WindowStart:       windowStart,
			WindowEnd:         windowEnd,
		})
		require.NoError(t, err)
		require.Len(t, result.Accounts, 1)
		require.NotNil(t, result.Accounts[0].CurrentBalanceMinor)
		assert.Equal(t, int64(12345), *result.Accounts[0].CurrentBalanceMinor)
		require.Len(t, result.Transactions, 1)
		assert.Equal(t, "txn-1", result.Transactions[0].ProviderTransactionID)
		assert.Len(t, result.Snapshots, 3)
	})

	t.Run("connector bank sync provider rejects nil connectors", func(t *testing.T) {
		_, ok := newConnectorBankSyncProvider(nil)
		assert.False(t, ok)
	})

	t.Run("connector bank sync provider rejects unsupported link entrypoints", func(t *testing.T) {
		provider := connectorBankSyncProvider{name: "monobank"}

		_, err := provider.StartLink(t.Context(), ProviderStartLinkParams{})
		require.ErrorIs(t, err, ErrUnsupportedBankLinkingMethod)

		_, err = provider.FinishLink(t.Context(), ProviderFinishLinkParams{})
		require.ErrorIs(t, err, ErrUnsupportedBankLinkingMethod)

		_, err = provider.LinkToken(t.Context(), ProviderTokenLinkParams{})
		require.ErrorIs(t, err, ErrUnsupportedBankLinkingMethod)
	})

	t.Run("New composes orchestrated synthetic first sync through SQLite", func(t *testing.T) {
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		now := time.Date(2026, time.August, 18, 11, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
		cipherKey := sha256.Sum256([]byte("finance-orchestrated-composition-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(cipherKey[:], "finance-orchestrated-composition")
		require.NoError(t, err)
		module, err := New(&Config{
			Database: database, Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
			NewID: uuid.NewString, HTTPClient: http.DefaultClient, ConnectionSecretCipher: cipher,
			Monobank: MonobankConfig{BaseURL: "https://" + fake.Internet().Domain()},
			EnableBanking: EnableBankingConfig{
				BaseURL: "https://" + fake.Internet().Domain(), AppID: "app-" + fake.UUID().V4(),
				PrivateKeyPath: "key-" + fake.UUID().V4() + ".pem",
				ASPSPs: []EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO, Name: "PKO", Country: "PL", PSUType: "personal", ValidDays: 90,
				}},
			},
		})
		require.NoError(t, err)
		ownerID := "owner-" + fake.UUID().V4()
		tenant, err := module.TenantService.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID: ownerID, Name: "tenant-" + fake.Company().Name(), DisplayCurrency: "PLN",
		})
		require.NoError(t, err)
		secretEnvelope, err := cipher.SealString("synthetic-secret-" + fake.UUID().V4())
		require.NoError(t, err)
		secret, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID: "secret-" + fake.UUID().V4(), Provider: string(domain.ProviderIDSynthetic),
			Reference: "secret-reference-" + fake.UUID().V4(), Envelope: secretEnvelope, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		providerReference := "synthetic-reference-" + fake.UUID().V4()
		_, err = persistence.NewSyntheticProviderStateStoreFromStore(store).SaveSyntheticProviderState(
			t.Context(), domain.SyntheticProviderState{
				ProviderReference: providerReference,
				Envelope: domain.SyntheticProviderStateEnvelope{
					Version: domain.SyntheticProviderStateVersion1,
					ConfiguredAccounts: []domain.SyntheticConfiguredAccount{{
						Key: "account-" + fake.UUID().V4(), Name: "Synthetic " + fake.Lorem().Word(), Currency: "PLN",
					}},
				},
				CreatedAt: now, UpdatedAt: now,
			},
		)
		require.NoError(t, err)
		connection, err := store.SaveBankConnection(t.Context(), domain.BankConnection{
			ID: "connection-" + fake.UUID().V4(), TenantID: tenant.ID, Provider: string(domain.ProviderIDSynthetic),
			ConnectorID: domain.ProviderConnectorIDSynthetic, ProviderReference: providerReference, SecretID: secret.ID,
			State: domain.BankConnectionStateActive, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		windowStart := now.AddDate(0, 0, -2)
		result, err := module.BankSyncService.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID, JobID: "job-" + fake.UUID().V4(), Reason: BankConnectionSyncReasonManual,
			WindowStart: &windowStart, WindowEnd: &now,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedAccounts)
		assert.Positive(t, result.ImportedTransactions)
		accounts, err := store.ListConnectionProviderAccounts(t.Context(), connection.ID)
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		transactions, err := store.ListTransactions(
			t.Context(), tenant.ID, accounts[0].FinanceAccountID, domain.TransactionSourceProvider, "", true,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, transactions)
	})
}
