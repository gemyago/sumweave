package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetAccountBalances(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/balances", r.URL.Path)
			_, _ = w.Write(
				[]byte(
					`{"balances":[{"type":"closingBooked","balance_amount":{"amount":"551.23","currency":"PLN"}},{"type":"interimAvailable","balance_amount":{"amount":"531.23","currency":"PLN"}}]}`,
				),
			)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountBalances(t.Context(), GetAccountBalancesParams{AccountID: accountID})

		require.NoError(t, err)
		require.Len(t, response.Balances, 2)
		assert.Equal(t, "closingBooked", response.Balances[0].Type)
		assert.Equal(t, "551.23", response.Balances[0].BalanceAmount.Amount)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"balances":[{"type":"current"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountBalances(t.Context(), GetAccountBalancesParams{AccountID: "acc-min"})

		require.NoError(t, err)
		require.Len(t, response.Balances, 1)
		assert.Equal(t, "current", response.Balances[0].Type)
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
