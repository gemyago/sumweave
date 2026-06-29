package enablebanking

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	enablebankingclient "github.com/gemyago/signal-foundry/finance/internal/enablebanking/client"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAPIClient struct {
	response map[string]any
	err      error
}

func (s *stubAPIClient) DoRawObject(
	context.Context,
	enablebankingclient.DoRawJSONParams,
) (map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func TestConnector(t *testing.T) {
	fake := faker.New()

	makeConnection := func() domain.ProviderConnectionRef {
		return domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDPKO,
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "provider-ref-" + fake.UUID().V4(),
			ExternalID:        "session-" + fake.UUID().V4(),
		}
	}

	makeSecret := func(reference string) domain.ConnectionSecret {
		return domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(domain.ProviderIDPKO),
			Reference: reference,
			Envelope: credentials.Envelope{
				KeyVersion: "v1",
				Algorithm:  credentials.AlgorithmAESGCM,
				Nonce:      "nonce-" + fake.UUID().V4(),
				Ciphertext: "ciphertext-" + fake.UUID().V4(),
			},
		}
	}

	makeSignedKeyPath := func(t *testing.T) string {
		t.Helper()

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})
		keyPath := filepath.Join(t.TempDir(), "enable-banking-private-key.pem")
		require.NoError(t, os.WriteFile(keyPath, privateKeyPEM, 0o600))

		return keyPath
	}

	decodeBody := func(t *testing.T, request *http.Request) map[string]any {
		t.Helper()

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))

		return payload
	}

	assertPayloadJSON := func(t *testing.T, payload []byte, expected string) {
		t.Helper()
		require.JSONEq(t, expected, string(payload))
	}

	t.Run("reports connector identity capabilities and unsupported token link", func(t *testing.T) {
		connector := NewConnector(Args{BaseURL: "https://example.test"})

		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, connector.ConnectorID())
		assert.Equal(t, providers.ConnectorCapabilities{
			SupportsStartLink:  true,
			SupportsFinishLink: true,
			SupportsFetch:      true,
		}, connector.Capabilities())

		_, err := connector.LinkToken(t.Context(), providers.LinkTokenRequest{Token: "token-" + fake.UUID().V4()})
		require.ErrorIs(t, err, ErrConnectorTokenLinkUnsupported)
	})

	t.Run("start link supports the legacy redirect auth branch", func(t *testing.T) {
		redirectURL := "https://app.example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		authorizationURL := "https://bank.example.test/auth/" + fake.UUID().V4()
		providerReference := "authorization-" + fake.UUID().V4()
		authResponse := fmt.Sprintf(
			`{"authorizationUrl":"%s","providerReference":"%s"}`,
			authorizationURL,
			providerReference,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "/auth", request.URL.Path)
			assert.Empty(t, request.Header.Get("Authorization"))

			payload := decodeBody(t, request)
			assert.Equal(t, redirectURL, payload["redirectUrl"])
			assert.Equal(t, state, payload["state"])
			assert.NotContains(t, payload, "redirect_url")

			_, _ = w.Write([]byte(authResponse))
		}))
		defer server.Close()

		connector := NewConnector(
			Args{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				StateProvider: func() (string, error) {
					return state, nil
				},
			},
		)

		result, err := connector.StartLink(t.Context(), providers.StartLinkRequest{
			Profile:     providers.PKOProfile(),
			RedirectURL: redirectURL,
		})
		require.NoError(t, err)

		assert.Equal(t, state, result.State)
		assert.Equal(t, authorizationURL, result.AuthorizationURL)
		require.Len(t, result.RawPayloads, 1)
		assert.Equal(t, domain.RawPayloadScopeConnection, result.RawPayloads[0].Scope)
		assert.Equal(t, providerReference, result.RawPayloads[0].ProviderObjectID)
		assertPayloadJSON(
			t,
			result.RawPayloads[0].PayloadJSON,
			`{"authorizationUrl":"`+authorizationURL+`","providerReference":"`+providerReference+`"}`,
		)
	})

	t.Run("finish link supports the legacy redirect auth branch and redacts secrets", func(t *testing.T) {
		redirectURL := "https://app.example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		code := "code-" + fake.UUID().V4()
		providerReference := "authorization-" + fake.UUID().V4()
		sessionID := "session-" + fake.UUID().V4()
		secretValue := "secret-" + fake.UUID().V4()
		authResponse := fmt.Sprintf(
			`{"authorizationUrl":"https://bank.example.test/auth","providerReference":"%s"}`,
			providerReference,
		)
		sessionResponse := fmt.Sprintf(
			`{"externalId":"%s","providerReference":"%s","displayName":"PKO legacy","secret":"%s","state":"active"}`,
			sessionID,
			providerReference,
			secretValue,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/auth":
				_, _ = w.Write([]byte(authResponse))
			case "/sessions":
				assert.Equal(t, http.MethodPost, request.Method)
				assert.Empty(t, request.Header.Get("Authorization"))

				payload := decodeBody(t, request)
				assert.Equal(t, state, payload["state"])
				assert.Equal(t, code, payload["code"])
				assert.Equal(t, providerReference, payload["providerReference"])

				_, _ = w.Write([]byte(sessionResponse))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		connector := NewConnector(
			Args{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				StateProvider: func() (string, error) {
					return state, nil
				},
			},
		)

		start, err := connector.StartLink(t.Context(), providers.StartLinkRequest{
			Profile:     providers.PKOProfile(),
			RedirectURL: redirectURL,
		})
		require.NoError(t, err)

		result, err := connector.FinishLink(t.Context(), providers.FinishLinkRequest{
			Profile: providers.PKOProfile(),
			State:   state,
			Code:    code,
			Start:   start,
		})
		require.NoError(t, err)

		assert.Equal(t, "PKO legacy", result.DisplayName)
		assert.Equal(t, providerReference, result.ProviderReference)
		assert.Equal(t, sessionID, result.ExternalID)
		assert.Equal(t, secretValue, result.Secret)
		assert.Equal(t, domain.BankConnectionStateActive, result.State)
		require.Len(t, result.RawPayloads, 1)
		assert.NotContains(t, string(result.RawPayloads[0].PayloadJSON), secretValue)
		assertPayloadJSON(
			t,
			result.RawPayloads[0].PayloadJSON,
			`{"displayName":"PKO legacy","externalId":"`+sessionID+`","providerReference":"`+providerReference+`","state":"active"}`,
		)
	})

	t.Run("start link supports the signed official redirect auth branch", func(t *testing.T) {
		redirectURL := "https://app.example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		authorizationURL := "https://bank.example.test/auth/" + fake.UUID().V4()
		providerReference := "authorization-" + fake.UUID().V4()
		now := time.Date(2026, time.June, 29, 15, 0, 0, 0, time.UTC)
		validDays := 45
		privateKeyPath := makeSignedKeyPath(t)
		authResponse := fmt.Sprintf(
			`{"authorizationUrl":"%s","id":"%s"}`,
			authorizationURL,
			providerReference,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "/auth", request.URL.Path)
			assert.NotEmpty(t, request.Header.Get("Authorization"))

			payload := decodeBody(t, request)
			assert.Equal(t, state, payload["state"])
			assert.Equal(t, redirectURL, payload["redirect_url"])
			assert.Equal(t, "personal", payload["psu_type"])
			assert.NotContains(t, payload, "redirectUrl")

			access := payload["access"].(map[string]any)
			assert.Equal(t, now.Add(time.Duration(validDays)*24*time.Hour).Format(time.RFC3339), access["valid_until"])

			aspsp := payload["aspsp"].(map[string]any)
			assert.Equal(t, "PKO Bank Polski", aspsp["name"])
			assert.Equal(t, "PL", aspsp["country"])

			_, _ = w.Write([]byte(authResponse))
		}))
		defer server.Close()

		connector := NewConnector(Args{
			BaseURL:        server.URL,
			HTTPClient:     server.Client(),
			StateProvider:  func() (string, error) { return state, nil },
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
			Now:            func() time.Time { return now },
			ValidDays:      validDays,
		})

		result, err := connector.StartLink(t.Context(), providers.StartLinkRequest{
			Profile:     providers.PKOProfile(),
			RedirectURL: redirectURL,
		})
		require.NoError(t, err)

		assert.Equal(t, state, result.State)
		assert.Equal(t, authorizationURL, result.AuthorizationURL)
		require.Len(t, result.RawPayloads, 1)
		assert.Equal(t, providerReference, result.RawPayloads[0].ProviderObjectID)
	})

	t.Run("finish link supports the signed official redirect auth branch and redacts secrets", func(t *testing.T) {
		state := "state-" + fake.UUID().V4()
		code := "code-" + fake.UUID().V4()
		sessionID := "session-" + fake.UUID().V4()
		secretValue := "secret-" + fake.UUID().V4()
		privateKeyPath := makeSignedKeyPath(t)
		authResponse := fmt.Sprintf(
			`{"authorizationUrl":"https://bank.example.test/auth","id":"auth-%s"}`,
			fake.UUID().V4(),
		)
		sessionResponse := fmt.Sprintf(
			`{"id":"%s","displayName":"PKO official","secret":"%s","state":"active"}`,
			sessionID,
			secretValue,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/auth":
				_, _ = w.Write([]byte(authResponse))
			case "/sessions":
				assert.Equal(t, http.MethodPost, request.Method)
				assert.NotEmpty(t, request.Header.Get("Authorization"))

				payload := decodeBody(t, request)
				assert.Equal(t, map[string]any{"code": code}, payload)

				_, _ = w.Write([]byte(sessionResponse))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		connector := NewConnector(Args{
			BaseURL:        server.URL,
			HTTPClient:     server.Client(),
			StateProvider:  func() (string, error) { return state, nil },
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
		})

		start, err := connector.StartLink(t.Context(), providers.StartLinkRequest{
			Profile:     providers.PKOProfile(),
			RedirectURL: "https://app.example.test/callback/" + fake.UUID().V4(),
		})
		require.NoError(t, err)

		result, err := connector.FinishLink(t.Context(), providers.FinishLinkRequest{
			Profile: providers.PKOProfile(),
			State:   state,
			Code:    code,
			Start:   start,
		})
		require.NoError(t, err)

		assert.Equal(t, "PKO official", result.DisplayName)
		assert.Equal(t, sessionID, result.ProviderReference)
		assert.Equal(t, sessionID, result.ExternalID)
		assert.Equal(t, secretValue, result.Secret)
		assert.Equal(t, domain.BankConnectionStateActive, result.State)
		require.Len(t, result.RawPayloads, 1)
		assert.NotContains(t, string(result.RawPayloads[0].PayloadJSON), secretValue)
		assertPayloadJSON(
			t,
			result.RawPayloads[0].PayloadJSON,
			`{"displayName":"PKO official","id":"`+sessionID+`","state":"active"}`,
		)
	})

	t.Run("returns bounded errors for unsupported auth and fetch branches", func(t *testing.T) {
		mixedConnector := NewConnector(Args{BaseURL: "https://example.test", AppID: "app-only"})

		_, err := mixedConnector.StartLink(
			t.Context(),
			providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)

		_, err = mixedConnector.FinishLink(
			t.Context(),
			providers.FinishLinkRequest{Code: "code", Start: providers.StartLinkResult{}},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)

		legacyConnector := NewConnector(Args{BaseURL: "https://example.test"})
		_, err = legacyConnector.Fetch(
			t.Context(),
			providers.FetchRequest{Connection: makeConnection()},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)

		privateKeyPath := makeSignedKeyPath(t)
		officialConnector := NewConnector(
			Args{
				BaseURL:        "https://example.test",
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			},
		)
		_, err = officialConnector.Fetch(
			t.Context(),
			providers.FetchRequest{
				Connection: makeConnection(),
				Secret:     makeSecret("reference-" + fake.UUID().V4()),
			},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)

		missingSessionConnection := makeConnection()
		missingSessionConnection.ExternalID = ""
		_, err = officialConnector.Fetch(
			t.Context(),
			providers.FetchRequest{Connection: missingSessionConnection},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)
	})

	t.Run("fetch maps the legacy bearer-secret branch into v2 observations", func(t *testing.T) {
		secretValue := "secret-" + fake.UUID().V4()
		secret := makeSecret("reference-" + fake.UUID().V4())
		connection := makeConnection()
		capturedAt := time.Date(2026, time.June, 30, 9, 45, 0, 0, time.UTC)
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 1, 12, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60)),
			End:   time.Date(2026, time.June, 3, 8, 30, 0, 0, time.FixedZone("UTC-4", -4*60*60)),
		}
		firstAccountID := "account-1-" + fake.UUID().V4()
		secondAccountID := "account-2-" + fake.UUID().V4()
		firstTransactionID := "txn-1-" + fake.UUID().V4()
		secondTransactionID := "txn-2-" + fake.UUID().V4()
		thirdTransactionID := "txn-3-" + fake.UUID().V4()
		accountsResponse := fmt.Sprintf(
			`{"accounts":[{"id":"%s","name":"Main account","currency":"pln","iban":" PL11111111111111111111111111 "},{"uid":"%s","currency":"eur","iban":" PL22222222222222222222222222 "}]}`,
			firstAccountID,
			secondAccountID,
		)
		firstBalancesResponse := `{"balances":[{"type":"closingBooked","balance_amount":{"amount":"1234.56","currency":"pln"}},{"type":"available","balance_amount":{"amount":"1200.01","currency":"pln"}}]}`
		secondBalancesResponse := `{"balances":[{"type":"available","balance_amount":{"amount":"50.05","currency":"eur"}}]}`
		firstTransactionsResponse := fmt.Sprintf(
			`{"transactions":[{"transactionId":"%s","status":"pending","amountMinor":-5050,"currency":"pln","description":"groceries","effectiveAt":"2026-06-01T09:15:00Z"},{"transactionId":"%s","amountMinor":250000,"currency":"pln","description":"salary","effectiveAt":"2026-06-02T18:05:00Z"}]}`,
			firstTransactionID,
			secondTransactionID,
		)
		secondTransactionsResponse := fmt.Sprintf(
			`{"transactions":[{"id":"%s","status":"booked","amountMinor":-1200,"currency":"eur","remittanceInformationUnstructured":"fuel","bookingDate":"2026-06-02"}]}`,
			thirdTransactionID,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			assert.Equal(t, "Bearer "+secretValue, request.Header.Get("Authorization"))

			switch request.URL.Path {
			case "/accounts":
				_, _ = w.Write([]byte(accountsResponse))
			case "/accounts/" + firstAccountID + "/balances":
				_, _ = w.Write([]byte(firstBalancesResponse))
			case "/accounts/" + secondAccountID + "/balances":
				_, _ = w.Write([]byte(secondBalancesResponse))
			case "/accounts/" + firstAccountID + "/transactions":
				_, _ = w.Write([]byte(firstTransactionsResponse))
			case "/accounts/" + secondAccountID + "/transactions":
				_, _ = w.Write([]byte(secondTransactionsResponse))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		resolverCalls := 0
		connector := NewConnector(
			Args{BaseURL: server.URL, HTTPClient: server.Client()},
			WithNow(func() time.Time { return capturedAt }),
			WithSecretResolver(func(_ context.Context, actual domain.ConnectionSecret) (string, error) {
				resolverCalls++
				assert.Equal(t, secret, actual)
				return secretValue, nil
			}),
		)

		batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      connection,
			Secret:          secret,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)

		assert.Equal(t, 1, resolverCalls)
		assert.Equal(t, connection, batch.Connection)
		assert.Equal(t, requestedWindow, batch.RequestedWindow)
		require.Len(t, batch.Accounts, 2)
		require.Len(t, batch.Balances, 2)
		require.Len(t, batch.Transactions, 3)
		require.Len(t, batch.RawPayloads, 5)

		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: firstAccountID,
			Name:              "Main account",
			Currency:          "PLN",
			IBAN:              "PL11111111111111111111111111",
		}, batch.Accounts[0])
		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: secondAccountID,
			Name:              secondAccountID,
			Currency:          "EUR",
			IBAN:              "PL22222222222222222222222222",
		}, batch.Accounts[1])

		firstAvailable := int64(120001)
		secondAvailable := int64(5005)
		assert.Equal(t, domain.ProviderBalanceObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			Currency:              "PLN",
			CurrentBalanceMinor:   123456,
			AvailableBalanceMinor: &firstAvailable,
			CapturedAt:            capturedAt,
		}, batch.Balances[0])
		assert.Equal(t, domain.ProviderBalanceObservation{
			Connection:            connection,
			ProviderAccountID:     secondAccountID,
			Currency:              "EUR",
			CurrentBalanceMinor:   5005,
			AvailableBalanceMinor: &secondAvailable,
			CapturedAt:            capturedAt,
		}, batch.Balances[1])

		firstEffectiveAt := time.Date(2026, time.June, 1, 9, 15, 0, 0, time.UTC)
		secondEffectiveAt := time.Date(2026, time.June, 2, 18, 5, 0, 0, time.UTC)
		thirdEffectiveAt := time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			ProviderTransactionID: firstTransactionID,
			Status:                domain.TransactionStatusPending,
			AmountMinor:           -5050,
			Currency:              "PLN",
			Description:           "groceries",
			EffectiveAt:           firstEffectiveAt,
			Fingerprint: providerFingerprint(
				firstAccountID,
				"groceries",
				int64(-5050),
				"PLN",
				firstEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -5050,
				Currency:    "PLN",
				Description: "groceries",
				EffectiveAt: &firstEffectiveAt,
			},
		}, batch.Transactions[0])
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			ProviderTransactionID: secondTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           250000,
			Currency:              "PLN",
			Description:           "salary",
			EffectiveAt:           secondEffectiveAt,
			Fingerprint: providerFingerprint(
				firstAccountID,
				"salary",
				int64(250000),
				"PLN",
				secondEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: 250000,
				Currency:    "PLN",
				Description: "salary",
				EffectiveAt: &secondEffectiveAt,
			},
		}, batch.Transactions[1])
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     secondAccountID,
			ProviderTransactionID: thirdTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           -1200,
			Currency:              "EUR",
			Description:           "fuel",
			EffectiveAt:           thirdEffectiveAt,
			Fingerprint: providerFingerprint(
				secondAccountID,
				"fuel",
				int64(-1200),
				"EUR",
				thirdEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -1200,
				Currency:    "EUR",
				Description: "fuel",
				EffectiveAt: &thirdEffectiveAt,
			},
		}, batch.Transactions[2])

		assert.Equal(t, connection.ExternalID, batch.RawPayloads[0].ProviderObjectID)
		assert.Equal(t, capturedAt, batch.RawPayloads[0].CapturedAt)
		assert.Equal(t, firstAccountID, batch.RawPayloads[1].ProviderObjectID)
		assert.Equal(t, firstAccountID, batch.RawPayloads[2].ProviderObjectID)
		assert.Equal(t, secondAccountID, batch.RawPayloads[3].ProviderObjectID)
		assert.Equal(t, secondAccountID, batch.RawPayloads[4].ProviderObjectID)
	})

	t.Run("fetch maps the signed official session branch into v2 observations", func(t *testing.T) {
		connection := makeConnection()
		connection.ExternalID = "session-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 10, 14, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
			End:   time.Date(2026, time.June, 14, 9, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)),
		}
		accountID := "account-" + fake.UUID().V4()
		firstTransactionID := "txn-1-" + fake.UUID().V4()
		secondTransactionID := "txn-2-" + fake.UUID().V4()
		privateKeyPath := makeSignedKeyPath(t)
		sessionResponse := fmt.Sprintf(
			`{"id":"%s","accounts":[{"uid":"%s","name":"Savings","currency":"pln","iban":" PL33333333333333333333333333 "}]}`,
			connection.ExternalID,
			accountID,
		)
		balancesResponse := `{"balances":[{"type":"closingBooked","balance_amount":{"amount":"777.70","currency":"pln"}},{"type":"interimAvailable","balance_amount":{"amount":"900.10","currency":"pln"}}]}`
		firstTransactionsResponse := fmt.Sprintf(
			`{"continuation_key":"page-2","transactions":[{"transactionId":"%s","status":"booked","amount":{"amount":"12.34","currency":"pln"},"credit_debit_indicator":"DBIT","remittance_information_unstructured":"coffee","booking_date":"2026-06-11"}]}`,
			firstTransactionID,
		)
		secondTransactionsResponse := fmt.Sprintf(
			`{"transactions":[{"id":"%s","amountMinor":5050,"currency":"pln","description":"refund","effectiveAt":"2026-06-12T10:30:00Z"}]}`,
			secondTransactionID,
		)

		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			requestCount++
			assert.NotEmpty(t, request.Header.Get("Authorization"))

			switch request.URL.Path {
			case "/sessions/" + connection.ExternalID:
				_, _ = w.Write([]byte(sessionResponse))
			case "/accounts/" + accountID + "/balances":
				_, _ = w.Write([]byte(balancesResponse))
			case "/accounts/" + accountID + "/transactions":
				assert.Equal(t, requestedWindow.Start.UTC().Format(time.DateOnly), request.URL.Query().Get("date_from"))
				assert.Equal(t, requestedWindow.End.UTC().Format(time.DateOnly), request.URL.Query().Get("date_to"))
				if request.URL.Query().Get("continuation_key") == "" {
					_, _ = w.Write([]byte(firstTransactionsResponse))
					return
				}
				assert.Equal(t, "page-2", request.URL.Query().Get("continuation_key"))
				_, _ = w.Write([]byte(secondTransactionsResponse))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		connector := NewConnector(
			Args{
				BaseURL:        server.URL,
				HTTPClient:     server.Client(),
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			},
			WithNow(func() time.Time { return capturedAt }),
		)

		batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      connection,
			RequestedWindow: requestedWindow,
		})
		require.NoError(t, err)

		assert.Equal(t, 4, requestCount)
		assert.Equal(t, connection, batch.Connection)
		assert.Equal(t, requestedWindow, batch.RequestedWindow)
		require.Len(t, batch.Accounts, 1)
		require.Len(t, batch.Balances, 1)
		require.Len(t, batch.Transactions, 2)
		require.Len(t, batch.RawPayloads, 4)

		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: accountID,
			Name:              "Savings",
			Currency:          "PLN",
			IBAN:              "PL33333333333333333333333333",
		}, batch.Accounts[0])

		available := int64(90010)
		assert.Equal(t, domain.ProviderBalanceObservation{
			Connection:            connection,
			ProviderAccountID:     accountID,
			Currency:              "PLN",
			CurrentBalanceMinor:   77770,
			AvailableBalanceMinor: &available,
			CapturedAt:            capturedAt,
		}, batch.Balances[0])

		firstEffectiveAt := time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC)
		secondEffectiveAt := time.Date(2026, time.June, 12, 10, 30, 0, 0, time.UTC)
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     accountID,
			ProviderTransactionID: firstTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           -1234,
			Currency:              "PLN",
			Description:           "coffee",
			EffectiveAt:           firstEffectiveAt,
			Fingerprint: providerFingerprint(
				accountID,
				"coffee",
				int64(-1234),
				"PLN",
				firstEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -1234,
				Currency:    "PLN",
				Description: "coffee",
				EffectiveAt: &firstEffectiveAt,
			},
		}, batch.Transactions[0])
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     accountID,
			ProviderTransactionID: secondTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           5050,
			Currency:              "PLN",
			Description:           "refund",
			EffectiveAt:           secondEffectiveAt,
			Fingerprint: providerFingerprint(
				accountID,
				"refund",
				int64(5050),
				"PLN",
				secondEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: 5050,
				Currency:    "PLN",
				Description: "refund",
				EffectiveAt: &secondEffectiveAt,
			},
		}, batch.Transactions[1])

		assert.Equal(t, connection.ExternalID, batch.RawPayloads[0].ProviderObjectID)
		assert.Equal(t, accountID, batch.RawPayloads[1].ProviderObjectID)
		assert.Equal(t, accountID, batch.RawPayloads[2].ProviderObjectID)
		assert.Equal(t, accountID, batch.RawPayloads[3].ProviderObjectID)
		assert.Equal(t, capturedAt, batch.RawPayloads[0].CapturedAt)
		assert.Equal(t, capturedAt, batch.RawPayloads[1].CapturedAt)
	})

	t.Run("covers helper and error branches", func(t *testing.T) {
		t.Run("start and finish return bounded validation errors", func(t *testing.T) {
			connector := NewConnector(
				Args{
					BaseURL: "https://example.test",
					StateProvider: func() (string, error) {
						return "", assert.AnError
					},
				},
				WithAPI(&stubAPIClient{}),
			)

			_, err := connector.StartLink(
				t.Context(),
				providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
			)
			require.ErrorIs(t, err, assert.AnError)

			connector = NewConnector(
				Args{BaseURL: "https://example.test"},
				WithAPI(&stubAPIClient{response: map[string]any{"id": "auth-1"}}),
			)
			_, err = connector.StartLink(
				t.Context(),
				providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
			)
			require.ErrorContains(t, err, "missing authorization URL")

			connector = NewConnector(
				Args{BaseURL: "https://example.test"},
				WithAPI(&stubAPIClient{response: map[string]any{}}),
			)
			_, err = connector.FinishLink(
				t.Context(),
				providers.FinishLinkRequest{
					State: "state",
					Code:  "code",
					Start: providers.StartLinkResult{
						RawPayloads: []domain.ProviderRawPayloadObservation{{
							Scope:       domain.RawPayloadScopeConnection,
							PayloadJSON: []byte(`{"providerReference":"provider-ref"}`),
						}},
					},
				},
			)
			require.ErrorContains(t, err, "missing session ID")

			_, err = connector.FinishLink(
				t.Context(),
				providers.FinishLinkRequest{
					State: "state",
					Code:  "code",
					Start: providers.StartLinkResult{},
				},
			)
			require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)
		})

		t.Run("direct helpers normalize and sanitize connector-owned values", func(t *testing.T) {
			connection := makeConnection()
			capturedAt := time.Date(2026, time.July, 2, 11, 0, 0, 0, time.UTC)

			assert.Equal(t, authBranchLegacy, NewConnector(Args{}).selectedAuthBranch())
			assert.Equal(
				t,
				authBranchOfficial,
				NewConnector(Args{AppID: "app", PrivateKeyPath: makeSignedKeyPath(t)}).selectedAuthBranch(),
			)
			assert.Equal(
				t,
				authBranchUnsupported,
				NewConnector(Args{AppID: "app"}).selectedAuthBranch(),
			)

			payload := NewConnector(
				Args{Now: func() time.Time { return capturedAt }, ValidDays: 1},
			).buildOfficialStartLinkPayload("https://redirect", "state")
			assert.Equal(t, "state", payload["state"])
			assert.Equal(t, "https://redirect", payload["redirect_url"])

			secretConnector := NewConnector(Args{})
			_, err := secretConnector.resolveSecret(t.Context(), makeSecret("reference-1"))
			require.ErrorIs(t, err, ErrSecretResolverRequired)

			_, err = secretConnector.resolveSecret(t.Context(), domain.ConnectionSecret{})
			require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)

			secretConnector = NewConnector(
				Args{},
				WithSecretResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
					return "", assert.AnError
				}),
			)
			_, err = secretConnector.resolveSecret(t.Context(), makeSecret("reference-2"))
			require.ErrorIs(t, err, assert.AnError)

			account := normalizeAccount(
				connection,
				" account-id ",
				map[string]any{"name": "", "currency": "pln", "iban": " PL123 "},
			)
			assert.Equal(t, "account-id", account.ProviderAccountID)
			assert.Equal(t, "account-id", account.Name)
			assert.Equal(t, "PLN", account.Currency)
			assert.Equal(t, "PL123", account.IBAN)

			current, available, currency := selectBalanceAmounts(map[string]any{
				"balances": []any{
					map[string]any{
						"type": "closingBooked",
						"balance_amount": map[string]any{
							"amount":   "10.50",
							"currency": "pln",
						},
					},
					map[string]any{
						"type": "available",
						"balanceAmount": map[string]any{
							"amount":   "12.00",
							"currency": "pln",
						},
					},
				},
			})
			require.NotNil(t, available)
			assert.Equal(t, int64(1050), current)
			assert.Equal(t, int64(1200), *available)
			assert.Equal(t, "PLN", currency)

			balance := normalizeBalance(
				connection,
				account,
				map[string]any{
					"balances": []any{map[string]any{
						"type": "available",
						"balance_amount": map[string]any{
							"amount":   "77.70",
							"currency": "eur",
						},
					}},
				},
				capturedAt,
			)
			assert.Equal(t, int64(7770), balance.CurrentBalanceMinor)
			require.NotNil(t, balance.AvailableBalanceMinor)
			assert.Equal(t, int64(7770), *balance.AvailableBalanceMinor)
			assert.Equal(t, "EUR", balance.Currency)

			effectiveAt := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)
			transaction := normalizeTransaction(
				connection,
				"account-id",
				map[string]any{
					"id":                                  "",
					"status":                              "",
					"amount":                              map[string]any{"amount": "12.34", "currency": "pln"},
					"credit_debit_indicator":              "DBIT",
					"remittance_information_unstructured": "fallback",
					"bookingDate":                         "2026-07-02",
				},
			)
			assert.Equal(t, "fallback", transaction.Description)
			assert.Equal(t, int64(-1234), transaction.AmountMinor)
			assert.Equal(t, domain.TransactionStatusBooked, transaction.Status)
			assert.Equal(t, effectiveAt, transaction.EffectiveAt)
			assert.NotEmpty(t, transaction.ProviderTransactionID)

			assert.Equal(t, effectiveAt, transactionTime(map[string]any{"booking_date": "2026-07-02"}))
			assert.Equal(
				t,
				int64(-1234),
				amountMinor(map[string]any{
					"amount":               map[string]any{"amount": "12.34"},
					"creditDebitIndicator": "DBIT",
				}),
			)
			assert.Equal(
				t,
				map[string]any{"amount": "1.23"},
				amountObject(map[string]any{"amount": map[string]any{"amount": "1.23"}}),
			)
			assert.Equal(
				t,
				"session-id",
				extractSessionIdentifier(map[string]any{"session": map[string]any{"id": "session-id"}}, "id"),
			)
			assert.Equal(
				t,
				[]map[string]any{{"id": "1"}},
				objectSlice(map[string]any{"items": []any{map[string]any{"id": "1"}, "skip"}}, "items"),
			)
			assert.Equal(t, "value", stringValue(map[string]any{"value": " value "}, "value"))
			assert.Equal(t, int64(12), int64Value(map[string]any{"value": float64(12)}, "value"))
			assert.Equal(t, int64(-1234), decimalToMinor("-12.34"))
			assert.Equal(t, "fallback", firstNonEmpty("", " fallback "))
			assert.Equal(t, int64(7), firstNonZeroInt64(0, 7, 9))
			require.JSONEq(t, `{"key":"value"}`, string(mustJSON(map[string]string{"key": "value"})))

			providerReference := providerReferenceFromStart(providers.StartLinkResult{
				RawPayloads: []domain.ProviderRawPayloadObservation{
					{
						Scope:       domain.RawPayloadScopeConnection,
						PayloadJSON: []byte(`{"providerReference":"provider-ref"}`),
					},
					{
						Scope:       domain.RawPayloadScopeAccount,
						PayloadJSON: []byte(`{"providerReference":"ignored"}`),
					},
				},
			})
			assert.Equal(t, "provider-ref", providerReference)
			assert.Equal(t, "accounts", connectionPayloadProviderObjectID(domain.ProviderConnectionRef{}, false))
			assert.Equal(t, "session", connectionPayloadProviderObjectID(domain.ProviderConnectionRef{}, true))

			redacted := redactRawPayload(map[string]any{
				"secret": "raw-secret",
				"nested": map[string]any{"token": "raw-token", "keep": "value"},
				"items":  []any{map[string]any{"secret": "nested-secret", "name": "kept"}},
			})
			assert.NotContains(t, redacted, "secret")
			assert.Equal(t, "value", redacted["nested"].(map[string]any)["keep"])

			sanitized := sanitizeSecretText(
				"Bearer abc token=xyz secret abc {\"secret\":\"value\"} eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature",
			)
			assert.NotContains(t, sanitized, "abc")
			assert.NotContains(t, sanitized, "xyz")
			assert.NotContains(t, sanitized, "value")

			providerErr := &enablebankingclient.ResponseError{
				Operation:  "auth",
				StatusCode: http.StatusTooManyRequests,
				Message:    "Bearer abc",
			}
			assert.NotContains(t, sanitizeClientError(providerErr).Error(), "abc")
			assert.NotContains(t, sanitizeClientError(errors.New("token abc")).Error(), "abc")

			firstFingerprint := providerFingerprint("a", 1, effectiveAt)
			secondFingerprint := providerFingerprint("a", 1, effectiveAt)
			assert.Equal(t, firstFingerprint, secondFingerprint)
		})
	})
}
