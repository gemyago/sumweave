package providers_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/enablebanking"
	"github.com/gemyago/sumweave/finance/internal/monobank"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWindowSyncExecutorRealConnectorComposition(t *testing.T) {
	t.Run("fetches monobank and pko through WithConnectors", func(t *testing.T) {
		fake := faker.New()
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 10, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, time.June, 12, 18, 30, 0, 0, time.UTC),
		}
		capturedAt := time.Date(2026, time.June, 29, 14, 0, 0, 0, time.UTC)

		makeConnection := func(
			providerID domain.ProviderID,
			connectorID domain.ProviderConnectorID,
		) domain.ProviderConnectionRef {
			return domain.ProviderConnectionRef{
				ConnectionID:      "connection-" + fake.UUID().V4(),
				ProviderID:        providerID,
				ConnectorID:       connectorID,
				ProviderReference: "provider-ref-" + fake.UUID().V4(),
			}
		}
		makeSecret := func(providerID domain.ProviderID) domain.ConnectionSecret {
			return domain.ConnectionSecret{
				ID:        "secret-" + fake.UUID().V4(),
				Provider:  string(providerID),
				Reference: "reference-" + fake.UUID().V4(),
				Envelope: credentials.Envelope{
					KeyVersion: "v1",
					Algorithm:  credentials.AlgorithmAESGCM,
					Nonce:      "nonce-" + fake.UUID().V4(),
					Ciphertext: "ciphertext-" + fake.UUID().V4(),
				},
			}
		}

		monobankToken := "token-" + fake.UUID().V4()
		monobankSecret := makeSecret(domain.ProviderIDMonobank)
		monobankConnection := makeConnection(domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank)
		monobankAccountID := "mono-account-" + fake.UUID().V4()
		monobankTransactionID := "mono-txn-" + fake.UUID().V4()
		monobankServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, monobankToken, r.Header.Get("X-Token"))

			switch r.URL.Path {
			case "/personal/client-info":
				_, _ = fmt.Fprintf(
					w,
					`{"name":"mono-%s","accounts":[{"id":"%s","type":"black","currencyCode":980,"balance":12345,"maskedPan":["4444********1111"],"iban":"UA123456789012345678901234567"}]}`,
					fake.Person().FirstName(),
					monobankAccountID,
				)
			case fmt.Sprintf(
				"/personal/statement/%s/%d/%d",
				monobankAccountID,
				requestedWindow.Start.UTC().Unix(),
				requestedWindow.End.UTC().Unix(),
			):
				_, _ = fmt.Fprintf(
					w,
					`[{"id":"%s","time":%d,"description":"groceries-%s","amount":-5050,"currencyCode":980,"balance":7295}]`,
					monobankTransactionID,
					requestedWindow.Start.Add(2*time.Hour).Unix(),
					fake.Lorem().Word(),
				)
			default:
				http.NotFound(w, r)
			}
		}))
		defer monobankServer.Close()

		enableBankingConnection := makeConnection(domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking)
		enableBankingAccountID := "pko-account-" + fake.UUID().V4()
		enableBankingTransactionID := "pko-txn-" + fake.UUID().V4()
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
		enableBankingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NotEmpty(t, r.Header.Get("Authorization"))

			switch r.URL.Path {
			case "/sessions/" + enableBankingConnection.ProviderReference:
				_, _ = fmt.Fprintf(
					w,
					`{"session_id":"%s","accounts":["%s"],"accounts_data":[{"uid":"%s","identification_hash":"hash","identification_hashes":[]}]}`,
					enableBankingConnection.ProviderReference,
					enableBankingAccountID,
					enableBankingAccountID,
				)
			case "/accounts/" + enableBankingAccountID + "/details":
				_, _ = w.Write([]byte(
					`{"name":"PKO Main","currency":"pln","account_id":{"iban":"PL11111111111111111111111111"}}`,
				))
			case "/accounts/" + enableBankingAccountID + "/balances":
				_, _ = w.Write([]byte(
					`{"balances":[{"type":"closingBooked","balance_amount":{"amount":"1234.56","currency":"pln"}},{"type":"available","balance_amount":{"amount":"1200.01","currency":"pln"}}]}`,
				))
			case "/accounts/" + enableBankingAccountID + "/transactions":
				_, _ = fmt.Fprintf(
					w,
					`{"transactions":[{"entry_reference":"%s","status":"BOOKED","transaction_amount":{"amount":"50.50","currency":"pln"},"credit_debit_indicator":"DBIT","remittance_information":["coffee-%s"],"booking_date":"2026-06-11"}]}`,
					enableBankingTransactionID,
					fake.Lorem().Word(),
				)
			default:
				http.NotFound(w, r)
			}
		}))
		defer enableBankingServer.Close()

		monobankConnector := monobank.NewConnector(
			monobank.Args{BaseURL: monobankServer.URL, HTTPClient: monobankServer.Client()},
			monobank.WithNow(func() time.Time { return capturedAt }),
			monobank.WithSecretTokenResolver(func(_ context.Context, actual domain.ConnectionSecret) (string, error) {
				assert.Equal(t, monobankSecret, actual)
				return monobankToken, nil
			}),
		)
		enableBankingConnector := enablebanking.NewConnector(
			enablebanking.Args{
				BaseURL:        enableBankingServer.URL,
				HTTPClient:     enableBankingServer.Client(),
				Logger:         slog.New(slog.DiscardHandler),
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			},
			enablebanking.WithNow(func() time.Time { return capturedAt }),
		)

		store := NewMockWindowSyncStore(t)
		loadConnections := make([]domain.ProviderConnectionRef, 0, 2)
		loadWindows := make([]domain.ProviderSyncWindow, 0, 2)
		appliedDiffPlans := make([]providers.ProviderDiffPlan, 0, 2)
		appliedApplyPlans := make([]providers.ApplyPlan, 0, 2)
		monobankLoad := store.EXPECT().LoadExistingWindow(
			mock.Anything,
			monobankConnection,
			requestedWindow,
			mock.Anything,
		)
		monobankLoad.RunAndReturn(
			func(_ context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow, _ []providers.ProviderTransactionIdentity) (providers.ExistingWindowSnapshot, error) {
				loadConnections = append(loadConnections, connection)
				loadWindows = append(loadWindows, window)
				return providers.ExistingWindowSnapshot{}, nil
			},
		)
		monobankLoad.Once()

		enableBankingLoad := store.EXPECT().LoadExistingWindow(
			mock.Anything,
			enableBankingConnection,
			requestedWindow,
			mock.Anything,
		)
		enableBankingLoad.RunAndReturn(
			func(_ context.Context, connection domain.ProviderConnectionRef, window domain.ProviderSyncWindow, _ []providers.ProviderTransactionIdentity) (providers.ExistingWindowSnapshot, error) {
				loadConnections = append(loadConnections, connection)
				loadWindows = append(loadWindows, window)
				return providers.ExistingWindowSnapshot{}, nil
			},
		)
		enableBankingLoad.Once()

		applySync := store.EXPECT().ApplySync(mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		applySync.RunAndReturn(
			func(
				_ context.Context,
				diffPlan providers.ProviderDiffPlan,
				applyPlan providers.ApplyPlan,
				_ ...domain.ProviderSyncState,
			) (domain.ProviderSyncStats, error) {
				appliedDiffPlans = append(appliedDiffPlans, diffPlan)
				appliedApplyPlans = append(appliedApplyPlans, applyPlan)
				return applyPlan.Stats, nil
			},
		)
		applySync.Twice()

		executor, err := providers.NewWindowSyncExecutor(
			providers.WithConnectors(monobankConnector, enableBankingConnector),
			providers.WithWindowSyncStore(store),
			providers.WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
		)
		require.NoError(t, err)

		monobankResult, err := executor.Execute(t.Context(), providers.WindowSyncRequest{
			Connection:      monobankConnection,
			Secret:          monobankSecret,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)

		enableBankingResult, err := executor.Execute(t.Context(), providers.WindowSyncRequest{
			Connection:      enableBankingConnection,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)

		assert.Equal(t, monobankConnection, monobankResult.Batch.Connection)
		assert.Equal(t, requestedWindow, monobankResult.Batch.RequestedWindow)
		require.Len(t, monobankResult.Batch.Accounts, 1)
		require.Len(t, monobankResult.Batch.Transactions, 1)
		assert.Equal(t, monobankAccountID, monobankResult.Batch.Accounts[0].ProviderAccountID)
		assert.Equal(t, monobankTransactionID, monobankResult.Batch.Transactions[0].ProviderTransactionID)
		assert.Equal(
			t,
			domain.ProviderSyncStats{ObservedAccounts: 1, ObservedTransactions: 1, CreatedTransactions: 1},
			monobankResult.Stats,
		)

		assert.Equal(t, enableBankingConnection, enableBankingResult.Batch.Connection)
		assert.Equal(t, requestedWindow, enableBankingResult.Batch.RequestedWindow)
		require.Len(t, enableBankingResult.Batch.Accounts, 1)
		require.Len(t, enableBankingResult.Batch.Transactions, 1)
		assert.Equal(t, enableBankingAccountID, enableBankingResult.Batch.Accounts[0].ProviderAccountID)
		assert.Equal(t, enableBankingTransactionID, enableBankingResult.Batch.Transactions[0].ProviderTransactionID)
		assert.Equal(
			t,
			domain.ProviderSyncStats{ObservedAccounts: 1, ObservedTransactions: 1, CreatedTransactions: 1},
			enableBankingResult.Stats,
		)

		assert.Equal(t, []domain.ProviderConnectionRef{monobankConnection, enableBankingConnection}, loadConnections)
		assert.Equal(t, []domain.ProviderSyncWindow{requestedWindow, requestedWindow}, loadWindows)
		require.Len(t, appliedDiffPlans, 2)
		require.Len(t, appliedApplyPlans, 2)
		assert.Equal(t, requestedWindow, appliedDiffPlans[0].RequestedWindow)
		assert.Equal(t, requestedWindow, appliedDiffPlans[1].RequestedWindow)
		assert.Equal(t, monobankConnection, appliedDiffPlans[0].Connection)
		assert.Equal(t, enableBankingConnection, appliedDiffPlans[1].Connection)
	})
}
