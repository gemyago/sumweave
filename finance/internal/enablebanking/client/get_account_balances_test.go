package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetAccountBalances(t *testing.T) {
	fake := faker.New()
	requireNoRawField(t, BalanceAmount{})
	requireNoRawField(t, AccountBalance{})
	requireNoRawField(t, GetAccountBalancesResponse{})

	t.Run("decodes documented account balances response", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "get_account_balances_response.json")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/balances", r.URL.Path)
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountBalances(t.Context(), GetAccountBalancesParams{AccountID: accountID})

		require.NoError(t, err)
		require.Len(t, response.Balances, 1)
		assert.Equal(t, "CLAV", response.Balances[0].BalanceType)
		assert.Equal(t, "1.23", response.Balances[0].BalanceAmount.Amount)
		assert.Equal(t, "EUR", response.Balances[0].BalanceAmount.Currency)
	})

	t.Run("ignores undocumented balance type alias", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"balances":[{"type":"current"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountBalances(t.Context(), GetAccountBalancesParams{AccountID: "acc-min"})

		require.NoError(t, err)
		require.Len(t, response.Balances, 1)
		assert.Empty(t, response.Balances[0].BalanceType)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountBalances(t.Context(), GetAccountBalancesParams{AccountID: "acc-bad"})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "get account balances failed")
	})
}
