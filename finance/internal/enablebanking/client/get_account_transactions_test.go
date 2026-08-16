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
		assert.True(t, response.ContinuationKey.Present)
		assert.True(t, response.ContinuationKey.Null)
		require.Len(t, response.Transactions, 1)
		require.NotNil(t, response.Transactions[0].EntryReference)
		assert.Equal(t, "5561990681", *response.Transactions[0].EntryReference)
		assert.True(t, response.Transactions[0].TransactionID.Present)
		assert.Equal(t, "transaction-docs-1", response.Transactions[0].TransactionID.String())
		assert.Equal(t, "EUR", response.Transactions[0].TransactionAmount.Currency)
		require.NotNil(t, response.Transactions[0].DebtorAccountAdditionalIdentification)
		assert.True(t, response.Transactions[0].DebtorAccountAdditionalIdentification.Array)
	})

	t.Run("decodes the documented example object additional identification form", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(
				`{"transactions":[{"transaction_amount":{"currency":"EUR","amount":"0.00"},"credit_debit_indicator":"CRDT","status":"BOOK","debtor_account_additional_identification":{"identification":"additional","scheme_name":"BBAN"}}],"continuation_key":""}`,
			))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))
		response, err := client.GetAccountTransactions(
			t.Context(),
			GetAccountTransactionsParams{AccountID: "acc-" + fake.UUID().V4()},
		)

		require.NoError(t, err)
		assert.True(t, response.ContinuationKey.Present)
		assert.False(t, response.ContinuationKey.Null)
		assert.Empty(t, response.ContinuationKey.Value)
		require.Len(t, response.Transactions, 1)
		identifications := response.Transactions[0].DebtorAccountAdditionalIdentification
		require.NotNil(t, identifications)
		assert.False(t, identifications.Array)
		require.Len(t, identifications.Values, 1)
		assert.Equal(t, "additional", identifications.Values[0].Identification)
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
		assert.False(t, response.Transactions[0].TransactionID.Present)
		assert.Equal(t, Amount{}, response.Transactions[0].TransactionAmount)
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
