package finance

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestFinance(t *testing.T) {
	t.Run("New returns finance instance with functional bank connection service", func(t *testing.T) {
		fake := faker.New()
		monobankToken := "mono-token-" + fake.UUID().V4()
		monobankName := "mono-" + fake.UUID().V4()
		monobankAccountID := "account-" + fake.UUID().V4()
		now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
		statementBody := fmt.Sprintf(
			`[{"id":"transaction-%s","time":%d,"description":"coffee","currencyCode":980,"amount":-250}]`,
			fake.UUID().V4(), now.Unix(),
		)
		monobankServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assert.Equal(t, monobankToken, request.Header.Get("X-Token"))
			switch {
			case request.URL.Path == "/personal/client-info":
				_, _ = writer.Write([]byte(
					`{"name":"` + monobankName + `","accounts":[{"id":"` +
						monobankAccountID + `","type":"black","currencyCode":980,"balance":12345}]}`,
				))
			case strings.HasPrefix(request.URL.Path, "/personal/statement/"+monobankAccountID+"/"):
				_, _ = writer.Write([]byte(statementBody))
			default:
				http.NotFound(writer, request)
			}
		}))
		defer monobankServer.Close()

		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		service := NewService(store)
		actorUserID := "user-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "UAH",
			SeedDefaults:    true,
		})
		require.NoError(t, err)

		key := sha256.Sum256([]byte("finance-new-" + fake.UUID().V4()))
		cipher, err := credentials.NewAESGCMCipher(key[:], "finance-new")
		require.NoError(t, err)

		financeModule, err := New(&Config{
			Database:               database,
			Logger:                 slog.New(slog.DiscardHandler),
			Now:                    func() time.Time { return now },
			NewID:                  uuid.NewString,
			HTTPClient:             monobankServer.Client(),
			ConnectionSecretCipher: cipher,
			Monobank: MonobankConfig{
				BaseURL: monobankServer.URL,
			},
			EnableBanking: EnableBankingConfig{
				BaseURL:        "https://" + fake.Internet().Domain(),
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: "enable-banking-private-key-" + fake.UUID().V4() + ".pem",
				ASPSPs: []EnableBankingASPSP{{
					ProviderID: domain.ProviderIDPKO,
					Name:       "bank-" + fake.Company().Name(),
					Country:    "PL",
					PSUType:    "personal",
					ValidDays:  90,
				}},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, financeModule.TenantService)
		require.NotNil(t, financeModule.CatalogService)
		require.NotNil(t, financeModule.LedgerService)
		require.NotNil(t, financeModule.ReportingService)
		require.NotNil(t, financeModule.FXService)
		require.NotNil(t, financeModule.CSVImportService)
		require.NotNil(t, financeModule.BankConnectionService)
		require.NotNil(t, financeModule.SyntheticLinkStateService)
		require.NotNil(t, financeModule.BankSyncService)
		require.NotNil(t, financeModule.ProviderSnapshotService)
		require.NotNil(t, financeModule.TransferDetailService)

		connection, err := financeModule.BankConnectionService.LinkTokenBankConnection(
			t.Context(),
			LinkTokenBankConnectionParams{
				ActorUserID: actorUserID,
				TenantID:    tenant.ID,
				Provider:    string(domain.ProviderIDMonobank),
				Token:       monobankToken,
			},
		)
		require.NoError(t, err)

		assert.Equal(t, string(domain.ProviderIDMonobank), connection.Provider)
		assert.Equal(t, domain.ProviderConnectorIDMonobank, connection.ConnectorID)
		assert.Empty(t, connection.ProviderReference)

		windowStart := now.Add(-24 * time.Hour)
		result, err := financeModule.BankSyncService.RunBankConnectionSync(t.Context(), RunBankConnectionSyncParams{
			ConnectionID: connection.ID, JobID: "job-" + fake.UUID().V4(), Reason: BankConnectionSyncReasonManual,
			WindowStart: &windowStart, WindowEnd: &now,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedAccounts)
		assert.Equal(t, 1, result.ImportedTransactions)
	})

	t.Run("New returns config validation error", func(t *testing.T) {
		_, err := New(nil)
		require.ErrorContains(t, err, "config is required")
	})
}
