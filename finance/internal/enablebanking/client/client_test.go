package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestClient(baseURL string, opts ...func(*Args)) *Client {
	args := Args{
		BaseURL: baseURL,
		Logger:  slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		Now:     func() time.Time { return time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC) },
	}
	for _, opt := range opts {
		opt(&args)
	}
	return NewClient(args)
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func withSignedAuth(t *testing.T) func(*Args) {
	t.Helper()
	fake := faker.New()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPath := filepath.Join(t.TempDir(), "enable-banking-"+fake.UUID().V4()+".pem")
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	require.NoError(t, os.WriteFile(privateKeyPath, privateKeyPEM, 0o600))

	return func(args *Args) {
		args.AppID = "app-" + fake.UUID().V4()
		args.PrivateKeyPath = privateKeyPath
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type errorReadCloser struct{ err error }

func (r errorReadCloser) Read(_ []byte) (int, error) { return 0, r.err }

func (errorReadCloser) Close() error { return nil }

func TestClient_InternalHelpers(t *testing.T) {
	t.Run("typed request helper covers encode build transport and read failures", func(t *testing.T) {
		client := makeTestClient("https://provider.example.test")
		badBody := make(chan int)
		_, err := sendJSON[chan int, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[chan int]{
				Method: http.MethodPost,
				Path:   "/auth",
				Body:   &badBody,
			},
		)
		require.ErrorContains(t, err, "enable banking request encode")

		client = makeTestClient(":bad")
		_, err = sendJSON[struct{}, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: "/auth"},
		)
		require.ErrorContains(t, err, "enable banking request build")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport failed")
			})}
		})
		_, err = sendJSON[struct{}, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: authPath},
		)
		require.ErrorContains(t, err, "transport failed")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       errorReadCloser{err: errors.New("read failed")},
				}, nil
			})}
		})
		_, err = sendJSON[struct{}, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: authPath},
		)
		require.ErrorContains(t, err, "enable banking response read: read failed")
	})

	t.Run("typed request helper covers decode status auth and response body capture", func(t *testing.T) {
		responseBody := `{"message":"bad body","error":"BAD_BODY"}`
		client := makeTestClient("https://provider.example.test", func(args *Args) {
			args.HTTPClient = &http.Client{
				Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
					assert.Equal(t, "Bearer token-123", request.Header.Get("Authorization"))
					switch request.URL.Path {
					case "/ok":
						assert.Equal(t, "application/json", request.Header.Get("Accept"))
						assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(`{"aspsps":[{"id":"aspsp-1"}]}`)),
						}, nil
					case "/bad-json":
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("{")),
						}, nil
					default:
						return &http.Response{
							StatusCode: http.StatusBadRequest,
							Body:       io.NopCloser(strings.NewReader(responseBody)),
						}, nil
					}
				}),
			}
		})

		ctx := WithBearerToken(t.Context(), "token-123")
		result, err := sendJSON[CreateSessionRequest, ListASPSPsResponse](
			ctx,
			client,
			sendJSONParams[CreateSessionRequest]{
				Method: http.MethodPost,
				Path:   "/ok",
				Body:   &CreateSessionRequest{Code: "code-123"},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Value)
		assert.JSONEq(t, `{"aspsps":[{"id":"aspsp-1"}]}`, string(result.Body))
		require.Len(t, result.Value.ASPSPs, 1)
		assert.Equal(t, "aspsp-1", result.Value.ASPSPs[0].ID)

		_, err = sendJSON[struct{}, ListASPSPsResponse](
			ctx,
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: "/bad-json"},
		)
		require.ErrorContains(t, err, "enable banking response decode")

		_, err = sendJSON[struct{}, ListASPSPsResponse](
			ctx,
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: "/bad-status"},
		)
		require.Error(t, err)
		var responseErr *ResponseError
		require.ErrorAs(t, err, &responseErr)
		assert.Equal(t, "bad-status", responseErr.Operation)
		assert.Equal(t, http.StatusBadRequest, responseErr.StatusCode)
		assert.Equal(t, "BAD_BODY", responseErr.Code)
		assert.Equal(t, "bad body", responseErr.Message)
	})

	t.Run("constructor and normalization helpers cover defaults", func(t *testing.T) {
		client := NewClient(Args{})
		require.NotNil(t, client.httpClient)
		assert.Equal(t, defaultBaseURL, client.baseURL)

		assert.Equal(t, "message", firstNonEmpty("", "message"))
		assert.Equal(t, int64(-1234), decimalToMinor("-12.34"))
		assert.Equal(t, int64(-1234), signedTransactionAmountMinor(AccountTransaction{
			TransactionAmount:    &TransactionAmount{Amount: "12.34"},
			CreditDebitIndicator: "DBIT",
		}))
		assert.Equal(t, "one", firstSliceValue([]string{" ", "one", "two"}))

		normalizedAccount := normalizeAccount(Account{
			UID:       "uid-1",
			Currency:  "pln",
			AccountID: &AccountIdentification{IBAN: " PL123 "},
		})
		assert.Equal(t, "uid-1", normalizedAccount.ID)
		assert.Equal(t, "PLN", normalizedAccount.Currency)
		assert.Equal(t, "PL123", normalizedAccount.IBAN)

		normalizedSession := normalizeSessionResponse(&SessionResponse{
			SessionID:  "session-1",
			ASPSP:      &ASPSP{Name: "Nordea"},
			AccountIDs: []string{"account-1"},
		})
		assert.Equal(t, "session-1", normalizedSession.ExternalID)
		assert.Equal(t, "Nordea", normalizedSession.DisplayName)
		require.Len(t, normalizedSession.Accounts, 1)
		assert.Equal(t, "account-1", normalizedSession.Accounts[0].UID)

		normalizedDetails := normalizeAccountDetailsResponse(&GetAccountDetailsResponse{
			Name:            "Main account",
			Currency:        "eur",
			AccountID:       &AccountIdentification{IBAN: "FI123"},
			AccountServicer: &FinancialInstitution{BICFI: "NDEAFIHH"},
		})
		assert.Equal(t, "Main account", normalizedDetails.OwnerName)
		assert.Equal(t, "FI123", normalizedDetails.IBAN)
		assert.Equal(t, "NDEAFIHH", normalizedDetails.BIC)
		assert.Equal(t, "EUR", normalizedDetails.Currency)

		normalizedBalances := normalizeBalances(&GetAccountBalancesResponse{
			Balances: []AccountBalance{{
				BalanceType:   "CLAV",
				BalanceAmount: &BalanceAmount{Currency: "eur"},
			}},
		})
		assert.Equal(t, "CLAV", normalizedBalances.Balances[0].Type)
		assert.Equal(t, "EUR", normalizedBalances.Balances[0].BalanceAmount.Currency)

		normalizedTransactions := normalizeTransactions(&GetAccountTransactionsResponse{
			Transactions: []AccountTransaction{{
				EntryReference:       "entry-1",
				TransactionAmount:    &TransactionAmount{Amount: "12.34", Currency: "pln"},
				CreditDebitIndicator: "DBIT",
				RemittanceInformation: []string{
					"Coffee",
				},
				TransactionDate: "2026-07-02",
			}},
		})
		assert.Equal(t, "entry-1", normalizedTransactions.Transactions[0].ID)
		assert.Equal(t, "PLN", normalizedTransactions.Transactions[0].Currency)
		assert.Equal(t, "Coffee", normalizedTransactions.Transactions[0].Description)
		assert.Equal(t, int64(-1234), normalizedTransactions.Transactions[0].AmountMinor)

		responseErr := &ResponseError{Message: "provider request failed"}
		assert.Equal(t, "provider request failed", responseErr.Message)
		message, code := parseResponseBody([]byte("{"))
		assert.Empty(t, message)
		assert.Empty(t, code)

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.PrivateKeyPath = filepath.Join(t.TempDir(), "missing-app-id.pem")
		})
		_, err := sendJSON[struct{}, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: authPath},
		)
		require.ErrorContains(t, err, "enable banking app ID is required")

		client = makeTestClient("https://provider.example.test", func(args *Args) {
			args.AppID = "app-1"
		})
		_, err = sendJSON[struct{}, ListASPSPsResponse](
			t.Context(),
			client,
			sendJSONParams[struct{}]{Method: http.MethodGet, Path: authPath},
		)
		require.ErrorContains(t, err, "enable banking private key path is required")
	})
}
