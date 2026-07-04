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

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	internalmonobank "github.com/gemyago/signal-foundry/finance/internal/monobank"
	internalproviders "github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/gemyago/signal-foundry/finance/persistence"
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
		fxEnqueuer := &capturedFXSyncJobEnqueuer{}
		fxWriter := &capturedFXSyncScheduleWriter{}
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
		}, connectors)
		services := newFocusedServices(store, serviceConfig)

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
			ExternalID:        "acc-1",
			WindowStart:       windowStart,
			WindowEnd:         windowEnd,
		})
		require.NoError(t, err)
		require.Len(t, result.Accounts, 1)
		require.NotNil(t, result.Accounts[0].CurrentBalanceMinor)
		assert.Equal(t, int64(12345), *result.Accounts[0].CurrentBalanceMinor)
		require.Len(t, result.Transactions, 1)
		assert.Equal(t, "txn-1", result.Transactions[0].ProviderTransactionID)
		assert.Len(t, result.RawPayloads, 2)
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
}
