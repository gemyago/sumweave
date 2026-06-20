package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableBankingAccountsAndTransactionsCommands(t *testing.T) {
	fake := faker.New()

	type accountResult struct {
		UID         string           `json:"uid"`
		Name        string           `json:"name,omitempty"`
		IBAN        string           `json:"iban,omitempty"`
		Currency    string           `json:"currency,omitempty"`
		Details     map[string]any   `json:"details,omitempty"`
		Balances    []map[string]any `json:"balances,omitempty"`
		Raw         map[string]any   `json:"raw"`
		DetailsRaw  map[string]any   `json:"detailsRaw,omitempty"`
		BalancesRaw []map[string]any `json:"balancesRaw,omitempty"`
	}

	type accountsCommandResult struct {
		Provider  string          `json:"provider"`
		Operation string          `json:"operation"`
		FetchedAt string          `json:"fetchedAt"`
		SessionID string          `json:"sessionId"`
		Accounts  []accountResult `json:"accounts"`
		Raw       map[string]any  `json:"raw"`
	}

	type transactionResult struct {
		TransactionID         string         `json:"transactionId,omitempty"`
		Status                string         `json:"status,omitempty"`
		BookingDate           string         `json:"bookingDate,omitempty"`
		ValueDate             string         `json:"valueDate,omitempty"`
		Amount                string         `json:"amount,omitempty"`
		Currency              string         `json:"currency,omitempty"`
		CreditDebitIndicator  string         `json:"creditDebitIndicator,omitempty"`
		RemittanceInformation string         `json:"remittanceInformation,omitempty"`
		Raw                   map[string]any `json:"raw"`
	}

	type transactionsCommandResult struct {
		Provider         string              `json:"provider"`
		Operation        string              `json:"operation"`
		FetchedAt        string              `json:"fetchedAt"`
		SessionID        string              `json:"sessionId"`
		AccountID        string              `json:"accountId"`
		From             string              `json:"from"`
		To               string              `json:"to"`
		Strategy         string              `json:"strategy,omitempty"`
		Status           string              `json:"status"`
		TransactionCount int                 `json:"transactionCount"`
		Transactions     []transactionResult `json:"transactions"`
		Raw              map[string]any      `json:"raw"`
	}

	makeRootCmd := func(t *testing.T, deps financePOCCommandDeps) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		rootCmd := newRootCmd()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.AddCommand(newFinancePOCCmd(deps))
		return rootCmd, stdout, stderr
	}

	writePrivateKeyFile := func(t *testing.T) string {
		t.Helper()
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
		privateKeyPath := filepath.Join(t.TempDir(), "key-"+fake.Lorem().Word()+".pem")
		require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))
		return privateKeyPath
	}

	writeSessionFile := func(t *testing.T, sessionID string) string {
		t.Helper()
		sessionFilePath := filepath.Join(t.TempDir(), "session-"+fake.Lorem().Word()+".json")
		sessionFile := enableBankingSessionFile{
			Provider:           enableBankingCommandName,
			CreatedAt:          time.Date(2026, time.June, 18, 15, 0, 0, 0, time.UTC).Format(time.RFC3339),
			Country:            "PL",
			ASPSPName:          "PKO Bank Polski",
			PSUType:            "personal",
			SessionID:          sessionID,
			AccessValidForDays: 90,
			Raw:                map[string]any{"session": map[string]any{"id": sessionID}},
		}
		payload, err := json.MarshalIndent(sessionFile, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(sessionFilePath, append(payload, '\n'), 0o600))
		return sessionFilePath
	}

	t.Run("accounts loads session file and fetches optional details and balances", func(t *testing.T) {
		fetchedAt := time.Date(2026, time.June, 18, 16, 0, 0, 0, time.UTC)
		privateKeyPath := writePrivateKeyFile(t)
		sessionID := "session-" + fake.Lorem().Word()
		sessionFilePath := writeSessionFile(t, sessionID)
		outputFilePath := filepath.Join(t.TempDir(), fake.Lorem().Word(), "accounts.json")

		requestedPaths := make([]string, 0, 5)
		sessionPayload := fmt.Sprintf(
			`{"session":{"id":%q},"accounts":[{"uid":"acc-primary","name":"Main account","iban":"PL27114020040000300201355387","currency":"PLN"},{"uid":"acc-savings","name":"Savings","currency":"EUR"}]}`,
			sessionID,
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPaths = append(requestedPaths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/sessions/" + sessionID:
				_, _ = w.Write([]byte(sessionPayload))
			case "/accounts/acc-primary/details":
				_, _ = w.Write([]byte(
					`{"account":{"uid":"acc-primary","owner_name":"Jane Example","product":"ROR"}}`,
				))
			case "/accounts/acc-savings/details":
				_, _ = w.Write([]byte(
					`{"account":{"uid":"acc-savings","owner_name":"Jane Example","product":"Savings"}}`,
				))
			case "/accounts/acc-primary/balances":
				_, _ = w.Write([]byte(
					`{"balances":[{"type":"expected","balance_amount":{"amount":"101.25","currency":"PLN"}}]}`,
				))
			case "/accounts/acc-savings/balances":
				_, _ = w.Write([]byte(
					`{"balances":[{"type":"interimAvailable","balance_amount":{"amount":"202.50","currency":"EUR"}}]}`,
				))
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{Now: func() time.Time { return fetchedAt }})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "accounts",
			"--session-file", sessionFilePath,
			"--include-details",
			"--include-balances",
			"--json",
			"--out", outputFilePath,
			"--base-url", server.URL,
			"--app-id", "app-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got accountsCommandResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, enableBankingCommandName, got.Provider)
		assert.Equal(t, "accounts", got.Operation)
		assert.Equal(t, fetchedAt.Format(time.RFC3339), got.FetchedAt)
		assert.Equal(t, sessionID, got.SessionID)
		require.Len(t, got.Accounts, 2)
		assert.Equal(t, "acc-primary", got.Accounts[0].UID)
		assert.Equal(t, "Jane Example", got.Accounts[0].Details["ownerName"])
		require.Len(t, got.Accounts[0].Balances, 1)
		assert.Equal(t, "PLN", got.Accounts[0].Balances[0]["currency"])
		assert.Equal(t, "expected", got.Accounts[0].Balances[0]["type"])
		assert.Contains(t, got.Raw, "session")

		writtenPayload, err := os.ReadFile(outputFilePath)
		require.NoError(t, err)
		assert.JSONEq(t, stdout.String(), string(writtenPayload))
		assert.Contains(t, stderr.String(), "retrieved 2 accounts")
		assert.Equal(t, []string{
			"/sessions/" + sessionID,
			"/accounts/acc-primary/details",
			"/accounts/acc-primary/balances",
			"/accounts/acc-savings/details",
			"/accounts/acc-savings/balances",
		}, requestedPaths)
	})

	t.Run("transactions load session file, paginate, and keep stdout json clean", func(t *testing.T) {
		fetchedAt := time.Date(2026, time.June, 18, 17, 0, 0, 0, time.UTC)
		privateKeyPath := writePrivateKeyFile(t)
		sessionID := "session-" + fake.Lorem().Word()
		sessionFilePath := writeSessionFile(t, sessionID)
		accountID := "acc-" + fake.Lorem().Word()
		outputFilePath := filepath.Join(t.TempDir(), fake.Lorem().Word(), "transactions.json")

		queries := make([]url.Values, 0, 2)
		sessionPayload := fmt.Sprintf(`{"session":{"id":%q}}`, sessionID)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/sessions/" + sessionID:
				_, _ = w.Write([]byte(sessionPayload))
			case "/accounts/" + accountID + "/transactions":
				queries = append(queries, r.URL.Query())
				continuationKey := r.URL.Query().Get("continuation_key")
				if continuationKey == "" {
					_, _ = w.Write([]byte(
						`{"transactions":[{"transaction_id":"txn-1","status":"BOOKED","booking_date":"2026-06-01","value_date":"2026-06-01","credit_debit_indicator":"credit","amount":{"amount":"11.11","currency":"PLN"},"remittance_information_unstructured":"salary"}],"continuation_key":"page-2"}`,
					))
					return
				}
				assert.Equal(t, "page-2", continuationKey)
				_, _ = w.Write([]byte(
					`{"transactions":[{"transaction_id":"txn-2","status":"BOOKED","booking_date":"2026-06-02","value_date":"2026-06-02","credit_debit_indicator":"debit","amount":{"amount":"-5.20","currency":"PLN"},"remittance_information_unstructured":"coffee"}]}`,
				))
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
		}))
		defer server.Close()

		rootCmd, stdout, stderr := makeRootCmd(t, financePOCCommandDeps{Now: func() time.Time { return fetchedAt }})
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "transactions",
			"--session-file", sessionFilePath,
			"--account-id", accountID,
			"--from", "2026-06-01",
			"--to", "2026-06-30",
			"--strategy", "all",
			"--status", "booked",
			"--json",
			"--out", outputFilePath,
			"--base-url", server.URL,
			"--app-id", "app-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))

		var got transactionsCommandResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, enableBankingCommandName, got.Provider)
		assert.Equal(t, "transactions", got.Operation)
		assert.Equal(t, sessionID, got.SessionID)
		assert.Equal(t, accountID, got.AccountID)
		assert.Equal(t, "2026-06-01", got.From)
		assert.Equal(t, "2026-06-30", got.To)
		assert.Equal(t, "all", got.Strategy)
		assert.Equal(t, "booked", got.Status)
		assert.Equal(t, 2, got.TransactionCount)
		require.Len(t, got.Transactions, 2)
		assert.Equal(t, "txn-1", got.Transactions[0].TransactionID)
		assert.Equal(t, "booked", got.Transactions[0].Status)
		assert.Equal(t, "11.11", got.Transactions[0].Amount)
		assert.Equal(t, "PLN", got.Transactions[0].Currency)
		assert.Contains(t, stderr.String(), "retrieved 2 transactions")
		assert.NotContains(t, stdout.String(), "retrieved 2 transactions")

		writtenPayload, err := os.ReadFile(outputFilePath)
		require.NoError(t, err)
		assert.JSONEq(t, stdout.String(), string(writtenPayload))

		require.Len(t, queries, 2)
		assert.Equal(t, "2026-06-01", queries[0].Get("date_from"))
		assert.Equal(t, "2026-06-30", queries[0].Get("date_to"))
		assert.Equal(t, "all", queries[0].Get("strategy"))
		assert.Equal(t, "booked", queries[0].Get("status"))
		assert.Empty(t, queries[0].Get("continuation_key"))
		assert.Equal(t, "page-2", queries[1].Get("continuation_key"))
	})

	t.Run("session file errors are surfaced before remote calls", func(t *testing.T) {
		privateKeyPath := writePrivateKeyFile(t)
		rootCmd, _, _ := makeRootCmd(t, financePOCCommandDeps{})
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
		rootCmd.SetArgs([]string{
			"finance-poc", "enable-banking", "accounts",
			"--session-file", filepath.Join(t.TempDir(), "missing-"+fake.Lorem().Word()+".json"),
			"--json",
			"--base-url", "https://example.test",
			"--app-id", "app-" + fake.Lorem().Word(),
			"--private-key-path", privateKeyPath,
		})

		err := rootCmd.ExecuteContext(context.Background())
		require.Error(t, err)
		require.ErrorContains(t, err, "read enable-banking session file")
	})

	t.Run("helper coverage for validation and normalization", func(t *testing.T) {
		t.Run("status helpers cover both pending and invalid values", func(t *testing.T) {
			statusValue, err := resolveEnableBankingTransactionStatus(enableBankingStatusPending)
			require.NoError(t, err)
			assert.Equal(t, enableBankingStatusPending, statusValue)
			assert.Equal(t, enableBankingStatusBoth, normalizeEnableBankingRequestedStatus(""))

			_, err = resolveEnableBankingTransactionStatus("invalid-" + fake.Lorem().Word())
			require.Error(t, err)
			require.ErrorContains(t, err, "status must be booked, pending, or both")
		})

		t.Run("transaction params and query validate date ordering and optional fields", func(t *testing.T) {
			_, _, _, err := validateEnableBankingTransactionsParams(enableBankingTransactionsParams{
				From:   "2026-06-10",
				To:     "2026-06-01",
				Status: enableBankingStatusBooked,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "must be on or after")

			query := makeEnableBankingTransactionsQuery(
				time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
				"",
				"",
				"",
			)
			assert.Equal(t, "2026-06-01", query.Get("date_from"))
			assert.Empty(t, query.Get("strategy"))
			assert.Empty(t, query.Get("status"))
		})

		t.Run("session confirmation validates missing session id and missing credentials", func(t *testing.T) {
			sessionFilePath := filepath.Join(t.TempDir(), "session.json")
			payload, err := json.Marshal(enableBankingSessionFile{})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(sessionFilePath, payload, 0o600))

			_, _, err = loadEnableBankingConfirmedSession(
				t.Context(),
				financePOCProviderRequest{},
				sessionFilePath,
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "session file missing session ID")

			sessionFilePath = writeSessionFile(t, "session-"+fake.Lorem().Word())
			_, _, err = loadEnableBankingConfirmedSession(
				t.Context(),
				financePOCProviderRequest{},
				sessionFilePath,
				time.Now,
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "app ID is required")
		})

		t.Run("normalizers handle wrapper-less details, empty balances, and alt transaction keys", func(t *testing.T) {
			details := normalizeEnableBankingAccountDetails(map[string]any{
				"ownerName": "Jane Example",
				"bic":       "BPKOPLPW",
			})
			assert.Equal(t, "Jane Example", details["ownerName"])
			assert.Equal(t, "BPKOPLPW", details["bic"])
			assert.Nil(t, normalizeEnableBankingAccountDetails(map[string]any{}))

			assert.Nil(t, extractEnableBankingBalances(map[string]any{"balances": "bad"}))
			assert.Nil(t, normalizeEnableBankingBalances(nil))
			assert.Nil(t, extractEnableBankingTransactionItems(map[string]any{"transactions": "bad"}))

			transaction := normalizeEnableBankingTransaction(map[string]any{
				"id":                                "txn-alt",
				"status":                            "PENDING",
				"bookingDate":                       "2026-06-03",
				"valueDate":                         "2026-06-04",
				"creditDebitIndicator":              "credit",
				"remittanceInformationUnstructured": "bonus",
				"amount": map[string]any{
					"amount":   "7.00",
					"currency": "EUR",
				},
			})
			assert.Equal(t, "txn-alt", transaction.TransactionID)
			assert.Equal(t, enableBankingStatusPending, transaction.Status)
			assert.Equal(t, "EUR", transaction.Currency)
		})
	})
}
