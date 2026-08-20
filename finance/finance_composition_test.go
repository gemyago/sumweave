package finance

import (
	"crypto/sha256"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
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
		commandPublisher := NewMockSemanticCommandPublisher(t)
		cipherKey := sha256.Sum256([]byte("finance-composition-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(cipherKey[:], "finance-composition")
		require.NoError(t, err)
		serviceConfig := focusedServicesConfigFromConfig(&Config{
			Database:               database,
			Logger:                 slog.New(slog.DiscardHandler),
			Now:                    func() time.Time { return now },
			NewID:                  func() string { return "id-1" },
			HTTPClient:             http.DefaultClient,
			ConnectionSecretCipher: cipher,
			FXProviders:            []FXRatesProvider{NewStaticFXProvider("custom-fx", nil)},
			DefaultFXProvider:      "custom-fx",
			CommandPublisher:       commandPublisher,
		}, newMockbankSyncOrchestrator(t))
		services := newFocusedServices(store, persistence.NewTransactionTagStore(database), serviceConfig)

		require.Same(t, commandPublisher, services.FXService.commandPublisher)
		require.Same(t, commandPublisher, services.CSVImportService.commandPublisher)
		require.Same(t, commandPublisher, services.BankSyncService.commandPublisher)
		assert.Equal(t, "custom-fx", services.FXService.defaultFXProvider)
		assert.Contains(t, services.FXService.fxProviders, "custom-fx")
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
