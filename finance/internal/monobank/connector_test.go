package monobank

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/monobank/client"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAPI struct {
	clientInfoResponse *client.GetPersonalClientInfoResponse
	clientInfoErr      error
	statementResponse  *client.GetPersonalStatementResponse
	statementErr       error
}

func (s *stubAPI) GetPersonalClientInfo(
	context.Context,
	client.GetPersonalClientInfoParams,
) (*client.GetPersonalClientInfoResponse, error) {
	if s.clientInfoErr != nil {
		return nil, s.clientInfoErr
	}
	return s.clientInfoResponse, nil
}

func (s *stubAPI) GetPersonalStatement(
	context.Context,
	client.GetPersonalStatementParams,
) (*client.GetPersonalStatementResponse, error) {
	if s.statementErr != nil {
		return nil, s.statementErr
	}
	return s.statementResponse, nil
}

func TestConnector(t *testing.T) {
	fake := faker.New()

	makeConnection := func() domain.ProviderConnectionRef {
		return domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        domain.ProviderIDMonobank,
			ConnectorID:       domain.ProviderConnectorIDMonobank,
			ProviderReference: "provider-ref-" + fake.UUID().V4(),
			ExternalID:        "external-" + fake.UUID().V4(),
		}
	}

	makeSecret := func(reference string) domain.ConnectionSecret {
		return domain.ConnectionSecret{
			ID:        "secret-" + fake.UUID().V4(),
			Provider:  string(domain.ProviderIDMonobank),
			Reference: reference,
			Envelope: credentials.Envelope{
				KeyVersion: "v1",
				Algorithm:  credentials.AlgorithmAESGCM,
				Nonce:      "nonce-" + fake.UUID().V4(),
				Ciphertext: "ciphertext-" + fake.UUID().V4(),
			},
		}
	}

	t.Run("reports connector identity capabilities and unsupported redirect link methods", func(t *testing.T) {
		connector := NewConnector(Args{BaseURL: "https://example.test"})

		assert.Equal(t, domain.ProviderConnectorIDMonobank, connector.ConnectorID())
		assert.Equal(t, providers.ConnectorCapabilities{
			SupportsTokenLink: true,
			SupportsFetch:     true,
		}, connector.Capabilities())

		_, err := connector.StartLink(t.Context(), providers.StartLinkRequest{})
		require.ErrorIs(t, err, ErrConnectorStartLinkUnsupported)

		_, err = connector.FinishLink(t.Context(), providers.FinishLinkRequest{})
		require.ErrorIs(t, err, ErrConnectorFinishLinkUnsupported)
	})

	t.Run("token link returns link result metadata without local persistence", func(t *testing.T) {
		token := "token-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.June, 29, 10, 30, 0, 0, time.UTC)
		clientName := "mono-" + fake.Person().Name()
		accountID := "account-" + fake.UUID().V4()
		responseBody := fmt.Sprintf(
			`{"name":"%s","accounts":[{"id":"%s","type":"%s","currencyCode":980,"balance":12345,"creditLimit":5000,"maskedPan":["4444********1111"],"iban":"%s"}]}`,
			clientName,
			accountID,
			"card-"+fake.Lorem().Word(),
			"UA"+fake.RandomStringWithLength(27),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/personal/client-info", r.URL.Path)
			assert.Equal(t, token, r.Header.Get("X-Token"))
			_, _ = w.Write([]byte(responseBody))
		}))
		defer server.Close()

		connector := NewConnector(
			Args{BaseURL: server.URL, HTTPClient: server.Client()},
			WithNow(func() time.Time { return capturedAt }),
		)

		result, err := connector.LinkToken(t.Context(), providers.LinkTokenRequest{
			Profile: Profile(),
			Token:   token,
		})
		require.NoError(t, err)

		require.Len(t, result.RawPayloads, 1)
		assert.Equal(t, clientName, result.DisplayName)
		assert.Equal(t, clientName, result.ProviderReference)
		assert.Equal(t, accountID, result.ExternalID)
		assert.Equal(t, token, result.Secret)
		assert.Equal(t, domain.BankConnectionStateActive, result.State)
		assert.Equal(t, domain.ProviderRawPayloadObservation{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: clientName,
			PayloadJSON:      []byte(responseBody),
			CapturedAt:       capturedAt,
		}, result.RawPayloads[0])
		assert.NotContains(t, string(result.RawPayloads[0].PayloadJSON), token)
	})

	t.Run("token link sanitizes upstream errors", func(t *testing.T) {
		token := "token-" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"bad token ` + token + `"}`))
		}))
		defer server.Close()

		connector := NewConnector(Args{BaseURL: server.URL, HTTPClient: server.Client()})

		_, err := connector.LinkToken(t.Context(), providers.LinkTokenRequest{Token: token})
		require.ErrorContains(t, err, "rate limit")
		assert.NotContains(t, err.Error(), token)
	})

	t.Run("fetch maps accounts balances transactions raw payloads and requested window", func(t *testing.T) {
		token := "token-" + fake.UUID().V4()
		capturedAt := time.Date(2026, time.June, 30, 9, 45, 0, 0, time.UTC)
		connection := makeConnection()
		secret := makeSecret("reference-" + fake.UUID().V4())
		requestedWindow := domain.ProviderSyncWindow{
			Start: time.Date(2026, time.June, 1, 12, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60)),
			End:   time.Date(2026, time.June, 3, 8, 30, 0, 0, time.FixedZone("UTC-4", -4*60*60)),
		}
		firstAccountID := "account-1-" + fake.UUID().V4()
		secondAccountID := "account-2-" + fake.UUID().V4()
		firstTransactionID := "txn-1-" + fake.UUID().V4()
		secondTransactionID := "txn-2-" + fake.UUID().V4()
		thirdTransactionID := "txn-3-" + fake.UUID().V4()
		firstDescription := "groceries-" + fake.Lorem().Word()
		secondDescription := "salary-" + fake.Lorem().Word()
		thirdDescription := "fuel-" + fake.Lorem().Word()
		firstTime := time.Date(2026, time.June, 1, 9, 15, 0, 0, time.UTC)
		secondTime := time.Date(2026, time.June, 2, 18, 5, 0, 0, time.UTC)
		thirdTime := time.Date(2026, time.June, 2, 7, 55, 0, 0, time.UTC)

		clientInfoBody := fmt.Sprintf(
			`{"name":"%s","accounts":[{"id":"%s","type":"black","currencyCode":980,"balance":150500,"creditLimit":10000,"maskedPan":["4444********1111","5555********2222"],"iban":"UA123456789012345678901234567"},{"id":"%s","currencyCode":840,"balance":50500,"maskedPan":["6666********3333"],"iban":"US123456789012345678901234567"}]}`,
			"mono-"+fake.Person().Name(),
			firstAccountID,
			secondAccountID,
		)
		firstStatementBody := fmt.Sprintf(
			`[{"id":"%s","time":%d,"description":"%s","hold":true,"amount":-5050,"currencyCode":980,"balance":145450},{"id":"%s","time":%d,"description":"%s","hold":false,"amount":250000,"currencyCode":980,"balance":395450}]`,
			firstTransactionID,
			firstTime.Unix(),
			firstDescription,
			secondTransactionID,
			secondTime.Unix(),
			secondDescription,
		)
		secondStatementBody := fmt.Sprintf(
			`[{"id":"%s","time":%d,"description":"%s","hold":false,"amount":-1200,"currencyCode":840,"balance":49300}]`,
			thirdTransactionID,
			thirdTime.Unix(),
			thirdDescription,
		)

		statementPaths := map[string]string{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			switch r.URL.Path {
			case "/personal/client-info":
				_, _ = w.Write([]byte(clientInfoBody))
			case fmt.Sprintf(
				"/personal/statement/%s/%d/%d",
				firstAccountID,
				requestedWindow.Start.UTC().Unix(),
				requestedWindow.End.UTC().Unix(),
			):
				statementPaths[firstAccountID] = r.URL.Path
				_, _ = w.Write([]byte(firstStatementBody))
			case fmt.Sprintf(
				"/personal/statement/%s/%d/%d",
				secondAccountID,
				requestedWindow.Start.UTC().Unix(),
				requestedWindow.End.UTC().Unix(),
			):
				statementPaths[secondAccountID] = r.URL.Path
				_, _ = w.Write([]byte(secondStatementBody))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		resolverCalls := 0
		connector := NewConnector(
			Args{BaseURL: server.URL, HTTPClient: server.Client()},
			WithNow(func() time.Time { return capturedAt }),
			WithSecretTokenResolver(func(_ context.Context, actual domain.ConnectionSecret) (string, error) {
				resolverCalls++
				assert.Equal(t, secret, actual)
				return token, nil
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
		require.Len(t, batch.RawPayloads, 3)
		for _, transaction := range batch.Transactions {
			assert.NotEmpty(t, transaction.RawPayloadJSON)
			assert.Contains(t, string(transaction.RawPayloadJSON), transaction.ProviderTransactionID)
		}

		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: firstAccountID,
			Name:              "black",
			Currency:          "UAH",
			IBAN:              "UA123456789012345678901234567",
			MaskedPAN:         "4444********1111",
		}, batch.Accounts[0])
		assert.Equal(t, domain.ProviderAccountObservation{
			Connection:        connection,
			ProviderAccountID: secondAccountID,
			Name:              secondAccountID,
			Currency:          "USD",
			IBAN:              "US123456789012345678901234567",
			MaskedPAN:         "6666********3333",
		}, batch.Accounts[1])

		availableBalance := int64(160500)
		assert.Equal(t, domain.ProviderBalanceObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			Currency:              "UAH",
			CurrentBalanceMinor:   150500,
			AvailableBalanceMinor: &availableBalance,
			CapturedAt:            capturedAt,
		}, batch.Balances[0])
		assert.Equal(t, domain.ProviderBalanceObservation{
			Connection:          connection,
			ProviderAccountID:   secondAccountID,
			Currency:            "USD",
			CurrentBalanceMinor: 50500,
			CapturedAt:          capturedAt,
		}, batch.Balances[1])

		firstEffectiveAt := time.Unix(firstTime.Unix(), 0)
		secondEffectiveAt := time.Unix(secondTime.Unix(), 0)
		thirdEffectiveAt := time.Unix(thirdTime.Unix(), 0)
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			ProviderTransactionID: firstTransactionID,
			Status:                domain.TransactionStatusPending,
			AmountMinor:           -5050,
			Currency:              "UAH",
			Description:           firstDescription,
			EffectiveAt:           firstEffectiveAt,
			Fingerprint: providerFingerprint(
				firstAccountID,
				firstDescription,
				int64(-5050),
				"UAH",
				firstEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -5050,
				Currency:    "UAH",
				Description: firstDescription,
				EffectiveAt: &firstEffectiveAt,
			},
			RawPayloadJSON: fmt.Appendf(
				nil,
				`{"id":"%s","time":%d,"description":"%s","hold":true,"amount":-5050,"currencyCode":980,"balance":145450}`,
				firstTransactionID,
				firstTime.Unix(),
				firstDescription,
			),
		}, batch.Transactions[0])
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     firstAccountID,
			ProviderTransactionID: secondTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           250000,
			Currency:              "UAH",
			Description:           secondDescription,
			EffectiveAt:           secondEffectiveAt,
			Fingerprint: providerFingerprint(
				firstAccountID,
				secondDescription,
				int64(250000),
				"UAH",
				secondEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: 250000,
				Currency:    "UAH",
				Description: secondDescription,
				EffectiveAt: &secondEffectiveAt,
			},
			RawPayloadJSON: fmt.Appendf(
				nil,
				`{"id":"%s","time":%d,"description":"%s","amount":250000,"currencyCode":980,"balance":395450}`,
				secondTransactionID,
				secondTime.Unix(),
				secondDescription,
			),
		}, batch.Transactions[1])
		assert.Equal(t, domain.ProviderTransactionObservation{
			Connection:            connection,
			ProviderAccountID:     secondAccountID,
			ProviderTransactionID: thirdTransactionID,
			Status:                domain.TransactionStatusBooked,
			AmountMinor:           -1200,
			Currency:              "USD",
			Description:           thirdDescription,
			EffectiveAt:           thirdEffectiveAt,
			Fingerprint: providerFingerprint(
				secondAccountID,
				thirdDescription,
				int64(-1200),
				"USD",
				thirdEffectiveAt,
			),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -1200,
				Currency:    "USD",
				Description: thirdDescription,
				EffectiveAt: &thirdEffectiveAt,
			},
			RawPayloadJSON: fmt.Appendf(
				nil,
				`{"id":"%s","time":%d,"description":"%s","amount":-1200,"currencyCode":840,"balance":49300}`,
				thirdTransactionID,
				thirdTime.Unix(),
				thirdDescription,
			),
		}, batch.Transactions[2])

		assert.Equal(t, domain.ProviderRawPayloadObservation{
			Connection:       connection,
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: connection.ExternalID,
			PayloadJSON:      []byte(clientInfoBody),
			CapturedAt:       capturedAt,
		}, batch.RawPayloads[0])
		assert.Equal(t, domain.ProviderRawPayloadObservation{
			Connection:       connection,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: firstAccountID,
			PayloadJSON:      []byte(firstStatementBody),
			CapturedAt:       capturedAt,
		}, batch.RawPayloads[1])
		assert.Equal(t, domain.ProviderRawPayloadObservation{
			Connection:       connection,
			Scope:            domain.RawPayloadScopeTransaction,
			ProviderObjectID: secondAccountID,
			PayloadJSON:      []byte(secondStatementBody),
			CapturedAt:       capturedAt,
		}, batch.RawPayloads[2])

		for _, payload := range batch.RawPayloads {
			assert.NotContains(t, string(payload.PayloadJSON), token)
		}
		assert.Equal(t, fmt.Sprintf(
			"/personal/statement/%s/%d/%d",
			firstAccountID,
			requestedWindow.Start.UTC().Unix(),
			requestedWindow.End.UTC().Unix(),
		), statementPaths[firstAccountID])
		assert.Equal(t, fmt.Sprintf(
			"/personal/statement/%s/%d/%d",
			secondAccountID,
			requestedWindow.Start.UTC().Unix(),
			requestedWindow.End.UTC().Unix(),
		), statementPaths[secondAccountID])
	})

	t.Run("fetch returns bounded secret resolver errors", func(t *testing.T) {
		resolverErr := fmt.Errorf("resolver-%s", fake.UUID().V4())
		connector := NewConnector(
			Args{BaseURL: "https://example.test"},
			WithSecretTokenResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
				return "", resolverErr
			}),
		)

		_, err := connector.Fetch(t.Context(), providers.FetchRequest{
			Connection:      makeConnection(),
			Secret:          makeSecret("reference-" + fake.UUID().V4()),
			RequestedWindow: domain.ProviderSyncWindow{Start: time.Now().UTC(), End: time.Now().UTC().Add(time.Hour)},
		})
		require.ErrorIs(t, err, resolverErr)
		assert.Contains(t, err.Error(), "resolve monobank access token")
	})

	t.Run("covers connector options helper and error branches", func(t *testing.T) {
		connection := makeConnection()
		secret := makeSecret("reference-" + fake.UUID().V4())
		connector := NewConnector(
			Args{},
			nil,
			WithAPI(nil),
			WithNow(nil),
			WithSecretTokenResolver(nil),
		)
		require.NotNil(t, connector.api)
		require.NotNil(t, connector.now)

		_, err := connector.Fetch(t.Context(), providers.FetchRequest{Connection: connection, Secret: secret})
		require.ErrorIs(t, err, ErrSecretTokenResolverRequired)

		clientInfoErr := &client.APIError{StatusCode: http.StatusBadGateway}
		statementErr := &client.APIError{StatusCode: http.StatusInternalServerError}
		passthroughErr := fmt.Errorf("passthrough-%s", fake.UUID().V4())
		stubbedAPI := &stubAPI{
			clientInfoResponse: &client.GetPersonalClientInfoResponse{
				ClientInfo: &client.Info{Accounts: []client.InfoAccount{{ID: ""}}},
				RawJSON:    []byte(`{"accounts":[{"id":""}]}`),
			},
			statementResponse: &client.GetPersonalStatementResponse{RawJSON: []byte(`[]`)},
		}
		connector = NewConnector(
			Args{},
			WithAPI(stubbedAPI),
			WithNow(func() time.Time { return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC) }),
			WithSecretTokenResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
				return "  token-with-spaces  ", nil
			}),
		)

		result, err := connector.LinkToken(t.Context(), providers.LinkTokenRequest{Token: "token"})
		require.NoError(t, err)
		assert.Equal(t, "monobank", result.DisplayName)
		assert.Empty(t, result.ExternalID)

		stubbedAPI.clientInfoErr = clientInfoErr
		_, err = connector.LinkToken(t.Context(), providers.LinkTokenRequest{Token: "token"})
		require.ErrorContains(t, err, "status 502")

		stubbedAPI.clientInfoErr = nil
		stubbedAPI.statementErr = statementErr
		_, err = connector.Fetch(t.Context(), providers.FetchRequest{
			Connection: connection,
			Secret:     secret,
			RequestedWindow: domain.ProviderSyncWindow{
				Start: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC),
			},
		})
		require.ErrorContains(t, err, "status 500")

		assert.Equal(t, passthroughErr, normalizeClientError(passthroughErr))
		assert.Empty(t, currencyCodeToISO(monobankCurrencyEUR-1))
		assert.Equal(t, currencyEUR, currencyCodeToISO(monobankCurrencyEUR))
		assert.Empty(t, firstNonEmpty("", "   "))
		assert.Empty(t, firstMaskedPAN([]string{"", "   "}))
		assert.Empty(t, firstAccountID(nil))
		assert.Equal(t, domain.TransactionStatusBooked, statusFromHold(false))
		assert.Equal(
			t,
			[]statementChunk{{accountID: "0"}},
			makeChunks("", domain.ProviderSyncWindow{}),
		)
		longChunks := makeChunks(
			"account",
			domain.ProviderSyncWindow{
				Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
			},
		)
		assert.Len(t, longChunks, 2)
		assert.Equal(t, "account", longChunks[0].accountID)
		assert.NotEmpty(t, providerFingerprint("a", 1, "b"))

		resolverErr := errors.New("resolver")
		connector = NewConnector(
			Args{},
			WithSecretTokenResolver(func(context.Context, domain.ConnectionSecret) (string, error) {
				return "", resolverErr
			}),
		)
		_, err = connector.resolveToken(t.Context(), secret)
		require.ErrorIs(t, err, resolverErr)
	})
}
