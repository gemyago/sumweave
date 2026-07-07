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

func TestClient_GetAccountDetails(t *testing.T) {
	fake := faker.New()
	requireNoRawField(t, GetAccountDetailsResponse{})

	t.Run("decodes documented account details resource", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "get_account_details_response.json")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/details", r.URL.Path)
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountDetails(t.Context(), GetAccountDetailsParams{AccountID: accountID})

		require.NoError(t, err)
		assert.Equal(t, "Main account", response.Name)
		assert.Equal(t, "Main account", response.OwnerName)
		assert.Equal(t, "Everyday banking", response.Product)
		assert.Equal(t, "NDEAFIHH", response.BIC)
		assert.Equal(t, "FI0455231152453547", response.IBAN)
		assert.Equal(t, "EUR", response.Currency)
	})

	t.Run("ignores undocumented ownerName alias", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ownerName":"Jane Minimal"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountDetails(t.Context(), GetAccountDetailsParams{AccountID: "acc-min"})

		require.NoError(t, err)
		assert.Empty(t, response.Name)
		assert.Empty(t, response.OwnerName)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"upstream failed"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountDetails(t.Context(), GetAccountDetailsParams{AccountID: "acc-bad"})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "get account details failed")
	})
}
