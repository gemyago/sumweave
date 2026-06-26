package finance

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type errorReadCloser struct{ err error }

func (r errorReadCloser) Read(_ []byte) (int, error) { return 0, r.err }

func (errorReadCloser) Close() error { return nil }

func TestProviderErrors(t *testing.T) {
	t.Run("enable banking surfaces unsupported and decode branches", func(t *testing.T) {
		provider := NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:       "http://127.0.0.1:1",
			StateProvider: func() (string, error) { return "", errors.New("state failed") },
		})
		_, err := provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.Error(t, err)

		provider = NewEnableBankingProvider(
			EnableBankingProviderConfig{BaseURL: "http://127.0.0.1:1"},
		)
		_, err = provider.LinkToken(t.Context(), ProviderTokenLinkParams{Token: "token"})
		require.Error(t, err)

		badJSON := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte("{"))
			}),
		)
		defer badJSON.Close()
		provider = NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: badJSON.URL})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.Error(t, err)
		_, err = provider.FinishLink(t.Context(), ProviderFinishLinkParams{})
		require.Error(t, err)

		upstreamValidation := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(
					writer,
					`{"code":422,"message":"Wrong ASPSP name provided","error":"WRONG_ASPSP_PROVIDED"}`,
				)
			}),
		)
		defer upstreamValidation.Close()
		provider = NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: upstreamValidation.URL})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.Error(t, err)
		var responseErr *ProviderResponseError
		require.ErrorAs(t, err, &responseErr)
		assert.Equal(t, "enable-banking", responseErr.Provider)
		assert.Equal(t, "auth", responseErr.Operation)
		assert.Equal(t, http.StatusUnprocessableEntity, responseErr.StatusCode)
		assert.Equal(t, "WRONG_ASPSP_PROVIDED", responseErr.Code)
		assert.Equal(t, "Wrong ASPSP name provided", responseErr.Message)
		assert.True(t, responseErr.IsClientError())
		assert.True(t, responseErr.IsEnableBankingWrongASPSP())

		assert.False(t, (*ProviderResponseError)(nil).IsClientError())
		assert.False(t, (*ProviderResponseError)(nil).IsEnableBankingWrongASPSP())

		fallbackErr := newProviderResponseError(
			"enable-banking",
			"auth",
			http.StatusInternalServerError,
			[]byte("token=secret-value "+strings.Repeat("x", 240)),
		)
		assert.False(t, fallbackErr.IsClientError())
		assert.Contains(t, fallbackErr.Message, "token=[REDACTED]")
		assert.Len(t, fallbackErr.Message, providerResponseErrorExcerptLimit+3)

		blankErr := newProviderResponseError(
			"enable-banking",
			"auth",
			http.StatusBadGateway,
			[]byte(" \n\t "),
		)
		assert.Equal(t, "provider request failed", blankErr.Message)
		assert.Empty(t, blankErr.Code)

		message, code := parseProviderResponseBody([]byte("{"))
		assert.Empty(t, message)
		assert.Empty(t, code)
	})

	t.Run("enable banking validates signed setup and required response fields", func(t *testing.T) {
		provider := NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:        "https://provider.example.test",
			PrivateKeyPath: filepath.Join(t.TempDir(), "missing-app-id.pem"),
		})
		_, err := provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.ErrorContains(t, err, "enable banking app ID is required")

		provider = NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL: "https://provider.example.test",
			AppID:   "app-1",
		})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.ErrorContains(t, err, "enable banking private key path is required")

		provider = NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:        "https://provider.example.test",
			AppID:          "app-1",
			PrivateKeyPath: filepath.Join(t.TempDir(), "missing-key.pem"),
		})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.ErrorContains(t, err, "read enable banking private key file")

		badKeyPath := filepath.Join(t.TempDir(), "bad-key.pem")
		require.NoError(t, os.WriteFile(badKeyPath, []byte("not a pem"), 0o600))
		provider = NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL:        "https://provider.example.test",
			AppID:          "app-1",
			PrivateKeyPath: badKeyPath,
		})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.ErrorContains(t, err, "parse enable banking private key file")

		missingFieldsServer := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/auth", "/sessions":
					_, _ = writer.Write([]byte(`{}`))
				default:
					http.NotFound(writer, request)
				}
			}),
		)
		defer missingFieldsServer.Close()
		provider = NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: missingFieldsServer.URL})
		_, err = provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.ErrorContains(t, err, "authorization URL")
		_, err = provider.FinishLink(t.Context(), ProviderFinishLinkParams{})
		require.ErrorContains(t, err, "session ID")
	})

	t.Run("enable banking doJSON reports encode and transport failures", func(t *testing.T) {
		provider := NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: "https://provider.example.test"})
		_, err := provider.doJSON(
			t.Context(),
			http.MethodPost,
			"/auth",
			nil,
			make(chan int),
			"",
		)
		require.ErrorContains(t, err, "enable banking request encode")

		provider = NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: ":bad"})
		_, err = provider.doJSON(t.Context(), http.MethodGet, "/auth", nil, nil, "")
		require.ErrorContains(t, err, "enable banking request build")

		provider = NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL: "https://provider.example.test",
			HTTPClient: &http.Client{Transport: roundTripperFunc(
				func(*http.Request) (*http.Response, error) {
					return nil, errors.New("transport failed")
				},
			)},
		})
		_, err = provider.doJSON(t.Context(), http.MethodGet, "/auth", nil, nil, "")
		require.ErrorContains(t, err, "transport failed")

		provider = NewEnableBankingProvider(EnableBankingProviderConfig{
			BaseURL: "https://provider.example.test",
			HTTPClient: &http.Client{Transport: roundTripperFunc(
				func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       errorReadCloser{err: errors.New("read failed")},
					}, nil
				},
			)},
		})
		_, err = provider.doJSON(t.Context(), http.MethodGet, "/auth", nil, nil, "")
		require.ErrorContains(t, err, "enable banking response read: read failed")
	})

	t.Run("enable banking sync legacy surfaces balance fetch failures", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/accounts":
					_, _ = io.WriteString(writer, `{"accounts":[{"id":"acc-1"}]}`)
				case "/accounts/acc-1/balances":
					writer.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(writer, `{"message":"boom"}`)
				default:
					http.NotFound(writer, request)
				}
			}),
		)
		defer server.Close()

		provider := NewEnableBankingProvider(EnableBankingProviderConfig{BaseURL: server.URL})
		_, err := provider.Sync(t.Context(), ProviderSyncParams{Secret: "token"})
		require.ErrorContains(t, err, "status 500")
	})

	t.Run("enable banking helper branches stay normalized", func(t *testing.T) {
		assert.Nil(t, enableBankingReauth(map[string]any{"state": "active"}))
		require.NotNil(t, enableBankingReauth(map[string]any{"state": "reauth_required"}))

		applyEnableBankingBalances(nil, map[string]any{"balances": []any{}})

		account := &ProviderNormalizedAccount{}
		applyEnableBankingBalances(account, map[string]any{
			"balances": []any{
				map[string]any{
					"type": "closingBooked",
					"balance_amount": map[string]any{
						"amount":   "551.23",
						"currency": "pln",
					},
				},
				map[string]any{
					"type": "interimAvailable",
					"balance_amount": map[string]any{
						"amount":   "531.23",
						"currency": "PLN",
					},
				},
			},
		})
		require.NotNil(t, account.CurrentBalanceMinor)
		require.NotNil(t, account.AvailableBalanceMinor)
		assert.Equal(t, int64(55123), *account.CurrentBalanceMinor)
		assert.Equal(t, int64(53123), *account.AvailableBalanceMinor)
		assert.Equal(t, "PLN", account.Currency)

		fallbackAccount := &ProviderNormalizedAccount{}
		applyEnableBankingBalances(fallbackAccount, map[string]any{
			"balances": []any{
				map[string]any{
					"balance_amount": map[string]any{
						"amount":   "10.00",
						"currency": "EUR",
					},
				},
			},
		})
		require.NotNil(t, fallbackAccount.CurrentBalanceMinor)
		require.NotNil(t, fallbackAccount.AvailableBalanceMinor)
		assert.Equal(t, int64(1000), *fallbackAccount.CurrentBalanceMinor)
		assert.Equal(t, int64(1000), *fallbackAccount.AvailableBalanceMinor)

		assert.Equal(
			t,
			"nested-session",
			enableBankingSessionIdentifier(map[string]any{"session": map[string]any{"id": "nested-session"}}, "id"),
		)
		assert.Empty(t, enableBankingSessionIdentifier(map[string]any{}, "id"))
		assert.Equal(
			t,
			int64(-2500),
			enableBankingAmountMinor(map[string]any{
				"amount":                 map[string]any{"amount": "25.00", "currency": "PLN"},
				"credit_debit_indicator": "DBIT",
			}),
		)
		assert.Equal(t, int64(125), enableBankingAmountMinor(map[string]any{"amountMinor": float64(125)}))
		assert.Equal(
			t,
			map[string]any{"amount": "1.00"},
			enableBankingAmountObject(
				map[string]any{"amount": map[string]any{"amount": "1.00"}},
			),
		)
		assert.Equal(
			t,
			map[string]any{"amount": "2.00"},
			enableBankingAmountObject(
				map[string]any{"balance_amount": map[string]any{"amount": "2.00"}},
			),
		)
		assert.Equal(
			t,
			time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			enableBankingTransactionTime(map[string]any{"booking_date": "2026-06-01"}),
		)
		assert.True(t, parseEnableBankingTime("bad").IsZero())
		assert.Equal(t, int64(-1234), enableBankingDecimalToMinor("-12.34"))
	})

	t.Run("monobank covers unsupported and chunk edge cases", func(t *testing.T) {
		provider := NewMonobankProvider(MonobankProviderConfig{BaseURL: "http://127.0.0.1:1"})
		assert.Equal(t, "monobank", provider.Name())
		_, err := provider.StartLink(
			t.Context(),
			ProviderStartLinkParams{RedirectURL: "https://example.com"},
		)
		require.Error(t, err)
		_, err = provider.FinishLink(t.Context(), ProviderFinishLinkParams{})
		require.Error(t, err)

		chunks := makeMonobankChunks("acct", time.Time{}, time.Time{})
		require.Len(t, chunks, 1)
		assert.Equal(t, int64(0), chunks[0].fromUnix)
		assert.Equal(t, int64(0), chunks[0].toUnix)
		assert.Equal(
			t,
			domain.TransactionStatusPending,
			bookedStatusFromMonobankHold(true),
		)
		assert.Equal(
			t,
			domain.TransactionStatusBooked,
			bookedStatusFromMonobankHold(false),
		)
		assert.Equal(t, "EUR", monobankCurrencyCodeToISO(978))
		assert.Equal(t, "USD", monobankCurrencyCodeToISO(840))
		assert.Empty(t, monobankCurrencyCodeToISO(999))

		badJSON := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte("{"))
			}),
		)
		defer badJSON.Close()
		provider = NewMonobankProvider(MonobankProviderConfig{BaseURL: badJSON.URL})
		_, err = provider.LinkToken(t.Context(), ProviderTokenLinkParams{Token: "token"})
		require.Error(t, err)

		rateLimited := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusTooManyRequests)
			}),
		)
		defer rateLimited.Close()
		provider = NewMonobankProvider(MonobankProviderConfig{BaseURL: rateLimited.URL})
		_, err = provider.LinkToken(t.Context(), ProviderTokenLinkParams{Token: "token"})
		require.Error(t, err)
		_, err = provider.Sync(t.Context(), ProviderSyncParams{Secret: "token"})
		require.Error(t, err)

		badPathProvider := NewMonobankProvider(MonobankProviderConfig{BaseURL: ":bad"})
		_, err = badPathProvider.LinkToken(t.Context(), ProviderTokenLinkParams{Token: "token"})
		require.Error(t, err)
		_, err = badPathProvider.Sync(t.Context(), ProviderSyncParams{Secret: "token"})
		require.Error(t, err)
	})

	t.Run("monobank sync respects canceled timer wait and empty external id", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/personal/client-info":
					_, _ = writer.Write(
						[]byte(
							`{"accounts":[{"id":"acc-1","type":"black","currencyCode":980,"balance":1}]}`,
						),
					)
				case "/personal/statement/acc-1/0/0":
					_, _ = writer.Write([]byte(`[]`))
				default:
					_, _ = writer.Write([]byte(`[]`))
				}
			}),
		)
		defer server.Close()

		provider := NewMonobankProvider(
			MonobankProviderConfig{BaseURL: server.URL, SleepBetweenRequests: time.Hour},
		)
		_, err := provider.Sync(t.Context(), ProviderSyncParams{
			Secret:      "token",
			WindowStart: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			WindowEnd:   time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = provider.Sync(ctx, ProviderSyncParams{
			Secret:      "token",
			WindowStart: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			WindowEnd:   time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)

		rateLimited := httptest.NewServer(
			http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/personal/client-info":
					_, _ = writer.Write(
						[]byte(
							`{"accounts":[{"id":"acc-1","type":"black","currencyCode":980,"balance":1}]}`,
						),
					)
				default:
					writer.WriteHeader(http.StatusTooManyRequests)
				}
			}),
		)
		defer rateLimited.Close()
		provider = NewMonobankProvider(MonobankProviderConfig{BaseURL: rateLimited.URL})
		_, err = provider.Sync(t.Context(), ProviderSyncParams{
			Secret:      "token",
			WindowStart: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			WindowEnd:   time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		})
		require.Error(t, err)
	})
}
