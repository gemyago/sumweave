package client

import (
	"fmt"
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
	requireNoRawField(t, TransactionAmount{})
	requireNoRawField(t, AccountTransaction{})
	requireNoRawField(t, GetAccountTransactionsResponse{})

	t.Run("decodes documented account transactions response and query fields", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "get_account_transactions_response.json")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/transactions", r.URL.Path)
			assert.Equal(t, url.Values{
				"date_from":          []string{"2026-06-01"},
				"date_to":            []string{"2026-06-15"},
				"strategy":           []string{"prefetched"},
				"transaction_status": []string{"booked"},
				"continuation_key":   []string{"next-1"},
			}, r.URL.Query())
			_, _ = fmt.Fprint(w, fixture)
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
		assert.Equal(t, "string", response.ContinuationKey)
		require.Len(t, response.Transactions, 1)
		assert.Equal(t, "5561990681", response.Transactions[0].EntryReference)
		assert.Equal(t, "transaction-docs-1", response.Transactions[0].TransactionID)
		assert.Equal(t, "string", response.Transactions[0].Description)
		assert.Equal(t, "RF07850352502356628678117", response.Transactions[0].RemittanceInformationUnstructured)
		assert.Equal(t, int64(123), response.Transactions[0].AmountMinor)
		assert.Equal(t, "EUR", response.Transactions[0].Currency)
		require.NotNil(t, response.Transactions[0].TransactionAmount)
		assert.Equal(t, response.Transactions[0].TransactionAmount, response.Transactions[0].Amount)
	})

	t.Run("ignores undocumented transaction aliases", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.RawQuery)
			_, _ = w.Write([]byte(
				`{"transactions":[{"id":"txn-2","amount":{"amount":"1.23","currency":"EUR"},"description":"legacy"}]}`,
			))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountTransactions(t.Context(), GetAccountTransactionsParams{AccountID: "acc-min"})

		require.NoError(t, err)
		require.Len(t, response.Transactions, 1)
		assert.Empty(t, response.Transactions[0].TransactionID)
		assert.Nil(t, response.Transactions[0].TransactionAmount)
		assert.Empty(t, response.Transactions[0].Description)
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
