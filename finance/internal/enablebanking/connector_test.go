package enablebanking

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	enablebankingclient "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnector(t *testing.T) {
	fake := faker.New()
	logger := slog.New(slog.DiscardHandler)
	stringPointer := func(value string) *string { return &value }

	makeConnection := func() domain.ProviderConnectionRef {
		return domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDPKO,
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: "session-" + fake.UUID().V4(),
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
		connector := NewConnector(Args{BaseURL: "https://example.test", Logger: logger})

		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, connector.ConnectorID())
		assert.Equal(t, providers.ConnectorCapabilities{
			SupportsStartLink:    true,
			SupportsFinishLink:   true,
			RequiresRedirectCode: true,
			SupportsFetch:        true,
		}, connector.Capabilities())

		_, err := connector.LinkToken(t.Context(), providers.LinkTokenRequest{Token: "token-" + fake.UUID().V4()})
		require.ErrorIs(t, err, ErrConnectorTokenLinkUnsupported)
	})

	t.Run("start link uses typed auth creation for the official redirect branch", func(t *testing.T) {
		redirectURL := "https://app.example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		authorizationURL := "https://bank.example.test/auth/" + fake.UUID().V4()
		providerReference := "authorization-" + fake.UUID().V4()
		now := time.Date(2026, time.June, 29, 15, 0, 0, 0, time.UTC)
		validDays := 45
		privateKeyPath := makeSignedKeyPath(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "/auth", request.URL.Path)
			assert.NotEmpty(t, request.Header.Get("Authorization"))

			payload := decodeBody(t, request)
			assert.Equal(t, state, payload["state"])
			assert.Equal(t, redirectURL, payload["redirect_url"])
			assert.Equal(t, "personal", payload["psu_type"])

			access := payload["access"].(map[string]any)
			assert.Equal(t, now.Add(time.Duration(validDays)*24*time.Hour).Format(time.RFC3339), access["valid_until"])

			aspsp := payload["aspsp"].(map[string]any)
			assert.Equal(t, "PKO Bank Polski", aspsp["name"])
			assert.Equal(t, "PL", aspsp["country"])

			_, _ = w.Write([]byte(
				`{"url":"` + authorizationURL + `","authorization_id":"` + providerReference + `","psu_id_hash":"psu-hash"}`,
			))
		}))
		defer server.Close()

		connector := NewConnector(Args{
			BaseURL:        server.URL,
			HTTPClient:     server.Client(),
			Logger:         logger,
			StateProvider:  func() (string, error) { return state, nil },
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
			ASPSPName:      "PKO Bank Polski",
			Country:        "PL",
			PSUType:        "personal",
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
		assertPayloadJSON(
			t,
			result.PendingDocument,
			`{"url":"`+authorizationURL+`","authorization_id":"`+providerReference+`","psu_id_hash":"psu-hash"}`,
		)
	})

	t.Run("finish link uses typed session creation and redacts secret from observations", func(t *testing.T) {
		state := "state-" + fake.UUID().V4()
		code := "code-" + fake.UUID().V4()
		sessionID := "session-" + fake.UUID().V4()
		secretValue := "secret-" + fake.UUID().V4()
		privateKeyPath := makeSignedKeyPath(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "/sessions", request.URL.Path)
			assert.NotEmpty(t, request.Header.Get("Authorization"))

			payload := decodeBody(t, request)
			assert.Equal(t, map[string]any{"code": code}, payload)

			_, _ = w.Write([]byte(
				`{"session_id":"` + sessionID + `","accounts":[],"aspsp":{"name":"PKO official","country":"PL"},"psu_type":"personal","access":{"valid_until":"2026-08-14T00:00:00Z"},"secret":"` + secretValue + `"}`,
			))
		}))
		defer server.Close()

		connector := NewConnector(Args{
			BaseURL:        server.URL,
			HTTPClient:     server.Client(),
			Logger:         logger,
			StateProvider:  func() (string, error) { return state, nil },
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
			ASPSPName:      "PKO Bank Polski",
			Country:        "PL",
			PSUType:        "personal",
		})

		result, err := connector.FinishLink(t.Context(), providers.FinishLinkRequest{
			Profile: providers.PKOProfile(),
			State:   state,
			Code:    code,
		})
		require.NoError(t, err)

		assert.Equal(t, "PKO official", result.DisplayName)
		assert.Equal(t, sessionID, result.ProviderReference)
		assert.Empty(t, result.Secret)
		assert.Equal(t, domain.BankConnectionStateActive, result.State)
		require.NotNil(t, result.ConnectionSnapshot)
		assert.NotContains(t, string(result.ConnectionSnapshot.DocumentJSON), secretValue)
		assertPayloadJSON(
			t,
			result.ConnectionSnapshot.DocumentJSON,
			`{"session_id":"`+sessionID+`","accounts":[],"aspsp":{"name":"PKO official","country":"PL"},"psu_type":"personal","access":{"valid_until":"2026-08-14T00:00:00Z"}}`,
		)
	})

	t.Run("returns bounded errors for unsupported auth and fetch branches", func(t *testing.T) {
		mixedConnector := NewConnector(Args{BaseURL: "https://example.test", Logger: logger, AppID: "app-only"})

		_, err := mixedConnector.StartLink(
			t.Context(),
			providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)

		_, err = mixedConnector.FinishLink(
			t.Context(),
			providers.FinishLinkRequest{Code: "code"},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)

		credentiallessConnector := NewConnector(Args{BaseURL: "https://example.test", Logger: logger})
		_, err = credentiallessConnector.StartLink(
			t.Context(),
			providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)

		_, err = credentiallessConnector.Fetch(
			t.Context(),
			providers.FetchRequest{Connection: makeConnection()},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)

		privateKeyPath := makeSignedKeyPath(t)
		officialConnector := NewConnector(
			Args{
				BaseURL:        "https://example.test",
				Logger:         logger,
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
		missingSessionConnection.ProviderReference = ""
		_, err = officialConnector.Fetch(
			t.Context(),
			providers.FetchRequest{Connection: missingSessionConnection},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedFetchBranch)
	})

	t.Run("fetch uses typed session balance and paged transaction operations", func(t *testing.T) {
		connection := makeConnection()
		connection.ProviderReference = "session-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 10, 14, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
			End:   time.Date(2026, time.June, 14, 9, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)),
		}
		accountID := "account-" + fake.UUID().V4()
		firstTransactionID := "txn-1-" + fake.UUID().V4()
		firstTransactionDetailsID := "transaction-details-1-" + fake.UUID().V4()
		secondTransactionID := "txn-2-" + fake.UUID().V4()
		secondTransactionDetailsID := "transaction-details-2-" + fake.UUID().V4()
		privateKeyPath := makeSignedKeyPath(t)

		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			requestCount++
			assert.NotEmpty(t, request.Header.Get("Authorization"))

			switch request.URL.Path {
			case "/sessions/" + connection.ProviderReference:
				_, _ = w.Write([]byte(
					`{"accounts":["` + accountID + `"],"accounts_data":[{"uid":"` + accountID + `","identification_hash":"hash","identification_hashes":[]}]}`,
				))
			case "/accounts/" + accountID + "/details":
				_, _ = w.Write([]byte(
					`{"name":"Savings","currency":"pln","account_id":{"iban":" PL33333333333333333333333333 "}}`,
				))
			case "/accounts/" + accountID + "/balances":
				_, _ = w.Write([]byte(
					`{"balances":[{"balance_type":"closingBooked","balance_amount":{"amount":"777.70","currency":"pln"}},{"balance_type":"interimAvailable","balance_amount":{"amount":"900.10","currency":"pln"}}]}`,
				))
			case "/accounts/" + accountID + "/transactions":
				assert.Equal(t, requestedWindow.Start.UTC().Format(time.DateOnly), request.URL.Query().Get("date_from"))
				assert.Equal(t, requestedWindow.End.UTC().Format(time.DateOnly), request.URL.Query().Get("date_to"))
				if request.URL.Query().Get("continuation_key") == "" {
					_, _ = w.Write([]byte(
						`{"continuation_key":"page-2","transactions":[{"entry_reference":"` + firstTransactionID + `","transaction_id":"` + firstTransactionDetailsID + `","status":"BOOKED","transaction_amount":{"amount":"12.34","currency":"pln"},"credit_debit_indicator":"DBIT","remittance_information":["coffee"],"booking_date":"2026-06-11"}]}`,
					))
					return
				}
				assert.Equal(t, "page-2", request.URL.Query().Get("continuation_key"))
				_, _ = w.Write([]byte(
					`{"transactions":[{"entry_reference":"` + secondTransactionID + `","transaction_id":"` + secondTransactionDetailsID + `","status":"BOOKED","transaction_amount":{"amount":"50.50","currency":"pln"},"credit_debit_indicator":"CRDT","remittance_information":["refund"],"booking_date":"2026-06-12T10:30:00Z"}]}`,
				))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		connector := NewConnector(
			Args{
				BaseURL:        server.URL,
				HTTPClient:     server.Client(),
				Logger:         logger,
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

		assert.Equal(t, 5, requestCount)
		assert.Equal(t, connection, batch.Connection)
		assert.Equal(t, requestedWindow, batch.RequestedWindow)
		require.Len(t, batch.Accounts, 1)
		require.Len(t, batch.Balances, 1)
		require.Len(t, batch.Transactions, 2)
		require.Len(t, batch.Snapshots, 5)

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
	})

	t.Run("fetch fills account name from account details when session only returns account IDs", func(t *testing.T) {
		connection := makeConnection()
		connection.ProviderReference = "session-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		accountName := "Mock ASPSP " + fake.Lorem().Word()
		privateKeyPath := makeSignedKeyPath(t)
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			requestCount++
			switch request.URL.Path {
			case "/sessions/" + connection.ProviderReference:
				_, _ = w.Write([]byte(
					`{"session_id":"` + connection.ProviderReference + `","accounts":["` + accountID + `"]}`,
				))
			case "/accounts/" + accountID + "/details":
				_, _ = w.Write([]byte(
					`{"details":"` + accountName + `","currency":"eur","account_id":{"iban":"FI1234567890123456"}}`,
				))
			case "/accounts/" + accountID + "/balances":
				_, _ = w.Write([]byte(`{"balances":[]}`))
			case "/accounts/" + accountID + "/transactions":
				_, _ = w.Write([]byte(`{"transactions":[]}`))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		connector := NewConnector(
			Args{
				BaseURL:        server.URL,
				HTTPClient:     server.Client(),
				Logger:         logger,
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			},
		)

		batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      connection,
			RequestedWindow: domain.ProviderSyncWindow{},
		})
		require.NoError(t, err)

		assert.Equal(t, 4, requestCount)
		require.Len(t, batch.Accounts, 1)
		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: accountID,
			Name:              accountName,
			Currency:          "EUR",
			IBAN:              "FI1234567890123456",
		}, batch.Accounts[0])
		require.Len(t, batch.Snapshots, 3)
	})

	t.Run("normalizes account display names without exposing IBAN by default", func(t *testing.T) {
		accountID := "account-" + fake.UUID().V4()
		iban := "PL" + fake.RandomStringWithLength(26)
		name := "name-" + fake.Lorem().Word()
		details := "details-" + fake.Lorem().Word()
		product := "product-" + fake.Lorem().Word()

		makeAccount := func(name string, details string, product string) enablebankingclient.Account {
			return enablebankingclient.Account{
				Name:      stringPointer(name),
				Details:   stringPointer(details),
				Product:   stringPointer(product),
				AccountID: &enablebankingclient.AccountIdentification{IBAN: stringPointer(iban)},
				Currency:  "pln",
			}
		}

		for _, scenario := range []struct {
			name    string
			account enablebankingclient.Account
			want    string
		}{
			{
				name:    "uses standard name first",
				account: makeAccount(name, details, product),
				want:    name,
			},
			{
				name:    "uses details when standard name is absent",
				account: makeAccount("", details, product),
				want:    details,
			},
			{
				name:    "uses product when name and details are absent",
				account: makeAccount("", "", product),
				want:    product,
			},
			{
				name:    "uses provider ID rather than IBAN as final fallback",
				account: makeAccount("", "", ""),
				want:    accountID,
			},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				account := normalizeAccount(makeConnection(), accountID, scenario.account)

				assert.Equal(t, scenario.want, account.Name)
			})
		}
	})

	t.Run(
		"fetch does not recover raw-only account fields that are absent from typed session models",
		func(t *testing.T) {
			connection := makeConnection()
			accountID := "account-" + fake.UUID().V4()
			privateKeyPath := makeSignedKeyPath(t)
			requestCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				requestCount++
				switch request.URL.Path {
				case "/sessions/" + connection.ProviderReference:
					_, _ = w.Write([]byte(
						`{"session_id":"` + connection.ProviderReference + `","accounts":["` + accountID + `"],"accounts_data":[{"name":"Mock ROR","currency":"EUR","iban":"PL123"}]}`,
					))
				default:
					http.NotFound(w, request)
				}
			}))
			defer server.Close()

			connector := NewConnector(Args{
				BaseURL:        server.URL,
				HTTPClient:     server.Client(),
				Logger:         logger,
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			})

			batch, err := connector.Fetch(t.Context(), providers.FetchRequest{
				Connection:      connection,
				RequestedWindow: domain.ProviderSyncWindow{},
			})
			require.NoError(t, err)

			assert.Equal(t, 1, requestCount)
			assert.Empty(t, batch.Accounts)
			assert.Empty(t, batch.Balances)
			assert.Empty(t, batch.Transactions)
			require.Len(t, batch.Snapshots, 1)
		},
	)

	t.Run("covers typed helper mapping and sanitized errors", func(t *testing.T) {
		connection := makeConnection()
		capturedAt := time.Date(2026, time.July, 2, 11, 0, 0, 0, time.UTC)

		request := NewConnector(Args{
			ASPSPName: "PKO Bank Polski",
			Country:   "PL",
			Logger:    logger,
			PSUType:   "personal",
			Now:       func() time.Time { return capturedAt },
			ValidDays: 1,
		}).buildOfficialStartLinkRequest("https://redirect", "state")
		assert.Equal(t, "state", request.State)
		assert.Equal(t, "https://redirect", request.RedirectURL)
		assert.Equal(t, "personal", request.PSUType)
		assert.Equal(t, "PKO Bank Polski", request.ASPSP.Name)
		assert.Equal(t, "PL", request.ASPSP.Country)

		account := normalizeAccount(
			connection,
			" account-id ",
			enablebankingclient.Account{
				Name:      stringPointer(""),
				Currency:  "pln",
				AccountID: &enablebankingclient.AccountIdentification{IBAN: stringPointer(" PL123 ")},
			},
		)
		assert.Equal(t, " account-id ", account.ProviderAccountID)
		assert.Equal(t, "account-id", account.Name)
		assert.Equal(t, "PLN", account.Currency)
		assert.Equal(t, "PL123", account.IBAN)

		current, available, currency := selectBalanceAmounts([]enablebankingclient.AccountBalance{
			{
				BalanceType: "closingBooked",
				BalanceAmount: enablebankingclient.BalanceAmount{
					Amount:   "10.50",
					Currency: "pln",
				},
			},
			{
				BalanceType: "available",
				BalanceAmount: enablebankingclient.BalanceAmount{
					Amount:   "12.00",
					Currency: "pln",
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
			[]enablebankingclient.AccountBalance{{
				BalanceType: "available",
				BalanceAmount: enablebankingclient.BalanceAmount{
					Amount:   "77.70",
					Currency: "eur",
				},
			}},
			capturedAt,
		)
		assert.Equal(t, int64(7770), balance.CurrentBalanceMinor)
		require.NotNil(t, balance.AvailableBalanceMinor)
		assert.Equal(t, int64(7770), *balance.AvailableBalanceMinor)
		assert.Equal(t, "EUR", balance.Currency)

		effectiveAt := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)
		fallbackNote := "fallback"
		transaction := normalizeTransaction(
			connection,
			"account-id",
			enablebankingclient.AccountTransaction{
				TransactionAmount: enablebankingclient.TransactionAmount{
					Amount:   "12.34",
					Currency: "pln",
				},
				CreditDebitIndicator: "DBIT",
				Note:                 &fallbackNote,
				BookingDate:          stringPointer("2026-07-02"),
			},
		)
		assert.Equal(t, "fallback", transaction.Description)
		assert.Equal(t, int64(-1234), transaction.AmountMinor)
		assert.Equal(t, domain.TransactionStatusBooked, transaction.Status)
		assert.Equal(t, effectiveAt, transaction.EffectiveAt)
		assert.NotEmpty(t, transaction.ProviderTransactionID)

		assert.Equal(
			t,
			effectiveAt,
			transactionTime(enablebankingclient.AccountTransaction{BookingDate: stringPointer("2026-07-02")}),
		)
		assert.Equal(
			t,
			int64(-1234),
			amountMinor(enablebankingclient.AccountTransaction{
				TransactionAmount:    enablebankingclient.TransactionAmount{Amount: "12.34"},
				CreditDebitIndicator: "DBIT",
			}),
		)

		assertPayloadJSON(
			t,
			mustJSON(&enablebankingclient.SessionResponse{}),
			`{"status":"","accounts":null,"accounts_data":null,"aspsp":{"name":"","country":""},"psu_type":"","psu_id_hash":"","access":{"valid_until":""},"created":""}`,
		)

		assert.Equal(t, int64(-1234), decimalToMinor("-12.34"))
		assert.Equal(t, "fallback", firstNonEmpty("", " fallback "))
		assert.Equal(t, int64(7), firstNonZeroInt64(0, 7, 9))
		require.JSONEq(t, `{"key":"value"}`, string(mustJSON(map[string]string{"key": "value"})))

		firstFingerprint := providerFingerprint("a", 1, effectiveAt)
		secondFingerprint := providerFingerprint("a", 1, effectiveAt)
		assert.Equal(t, firstFingerprint, secondFingerprint)
	})

	t.Run("covers typed validation failures without raw transport fallback", func(t *testing.T) {
		privateKeyPath := makeSignedKeyPath(t)
		connector := NewConnector(
			Args{
				BaseURL:        "https://example.test",
				Logger:         logger,
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
				StateProvider: func() (string, error) {
					return "", assert.AnError
				},
			},
		)

		_, err := connector.StartLink(
			t.Context(),
			providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
		)
		require.ErrorIs(t, err, assert.AnError)

		missingAuthURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"auth-1"}`))
		}))
		defer missingAuthURLServer.Close()

		connector = NewConnector(Args{
			BaseURL:        missingAuthURLServer.URL,
			HTTPClient:     missingAuthURLServer.Client(),
			Logger:         logger,
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
		})
		_, err = connector.StartLink(
			t.Context(),
			providers.StartLinkRequest{RedirectURL: "https://example.test/callback"},
		)
		require.ErrorContains(t, err, "missing authorization URL")

		missingSessionIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer missingSessionIDServer.Close()

		connector = NewConnector(Args{
			BaseURL:        missingSessionIDServer.URL,
			HTTPClient:     missingSessionIDServer.Client(),
			Logger:         logger,
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
		})
		_, err = connector.FinishLink(
			t.Context(),
			providers.FinishLinkRequest{State: "state", Code: "code"},
		)
		require.ErrorContains(t, err, "missing session ID")

		missingCredentialsConnector := NewConnector(Args{BaseURL: "https://example.test", Logger: logger})
		_, err = missingCredentialsConnector.FinishLink(
			t.Context(),
			providers.FinishLinkRequest{State: "state", Code: "code"},
		)
		require.ErrorIs(t, err, ErrConnectorUnsupportedAuthBranch)
	})

	t.Run("covers typed option and helper edge branches", func(t *testing.T) {
		privateKeyPath := makeSignedKeyPath(t)

		api := enablebankingclient.NewClient(enablebankingclient.Args{
			BaseURL:        "https://example.test",
			HTTPClient:     http.DefaultClient,
			Logger:         logger,
			AppID:          "app-" + fake.UUID().V4(),
			PrivateKeyPath: privateKeyPath,
			Now:            func() time.Time { return time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC) },
		})
		connector := NewConnector(
			Args{
				BaseURL:        "https://example.test",
				Logger:         logger,
				AppID:          "app-" + fake.UUID().V4(),
				PrivateKeyPath: privateKeyPath,
			},
			nil,
			WithAPI(api),
		)
		require.NotNil(t, connector)

		account := domain.ProviderAccountObservation{
			Connection:        makeConnection(),
			ProviderAccountID: "account-" + fake.UUID().V4(),
			Currency:          "USD",
		}
		balance := normalizeBalance(
			account.Connection,
			account,
			[]enablebankingclient.AccountBalance{{BalanceType: "available"}},
			time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC),
		)
		assert.Equal(t, "USD", balance.Currency)

		current, available, currency := selectBalanceAmounts(
			[]enablebankingclient.AccountBalance{{BalanceType: "available"}},
		)
		require.NotNil(t, available)
		assert.Equal(t, int64(0), current)
		assert.Equal(t, int64(0), *available)
		assert.Empty(t, currency)

		assert.True(t, transactionTime(enablebankingclient.AccountTransaction{}).IsZero())
		assert.Equal(
			t,
			int64(1234),
			amountMinor(enablebankingclient.AccountTransaction{
				TransactionAmount: enablebankingclient.TransactionAmount{Amount: "12.34"},
			}),
		)
		assert.Empty(t, balanceAmountValue(enablebankingclient.BalanceAmount{}))
		assert.Empty(t, balanceAmountCurrency(enablebankingclient.BalanceAmount{}))
		assert.Empty(t, transactionAmountValue(enablebankingclient.TransactionAmount{}))
		assert.Empty(t, transactionAmountCurrency(enablebankingclient.TransactionAmount{}))
		assert.Equal(t, domain.TransactionStatusPending, normalizeTransactionStatus("pending"))
		assert.Equal(t, domain.TransactionStatus("custom"), normalizeTransactionStatus(" custom "))
		assert.Equal(t, int64(0), decimalToMinor("bad"))
		assert.Equal(t, int64(0), firstNonZeroInt64(0, 0))
		assertPayloadJSON(t, mustJSON((*enablebankingclient.SessionResponse)(nil)), `null`)

		assert.Equal(t, "{}", string(mustJSON(func() {})))
	})
}
