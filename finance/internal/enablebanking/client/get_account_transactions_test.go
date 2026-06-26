package client

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetAccountTransactions(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/transactions", r.URL.Path)
			assert.Equal(t, url.Values{
				"date_from":        []string{"2026-06-01"},
				"date_to":          []string{"2026-06-15"},
				"strategy":         []string{"prefetched"},
				"status":           []string{"booked"},
				"continuation_key": []string{"next-1"},
			}, r.URL.Query())
			_, _ = w.Write(
				[]byte(
					`{"continuation_key":"next-2","transactions":[{"id":"txn-1","status":"booked","booking_date":"2026-06-10","amount":{"amount":"25.00","currency":"PLN"},"credit_debit_indicator":"DBIT","remittance_information_unstructured":"Coffee"}]}`,
				),
			)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountTransactions(t.Context(), GetAccountTransactionsParams{
			AccountID:       accountID,
			DateFrom:        time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			DateTo:          time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC),
			Strategy:        "prefetched",
			Status:          "booked",
			ContinuationKey: "next-1",
		})

		require.NoError(t, err)
		assert.Equal(t, "next-2", response.ContinuationKey)
		require.Len(t, response.Transactions, 1)
		assert.Equal(t, "txn-1", response.Transactions[0].TransactionID)
		assert.Equal(t, int64(-2500), response.Transactions[0].AmountMinor)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"transactions":[{"transaction_id":"txn-2"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountTransactions(t.Context(), GetAccountTransactionsParams{AccountID: "acc-min"})

		require.NoError(t, err)
		require.Len(t, response.Transactions, 1)
		assert.Equal(t, "txn-2", response.Transactions[0].TransactionID)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"upstream failed"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountTransactions(t.Context(), GetAccountTransactionsParams{AccountID: "acc-bad"})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "get account transactions failed")
	})
}
