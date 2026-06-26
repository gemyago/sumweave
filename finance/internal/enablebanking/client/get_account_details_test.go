package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetAccountDetails(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		accountID := "acc-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/accounts/"+accountID+"/details", r.URL.Path)
			_, _ = w.Write([]byte(`{"account":{"owner_name":"Jane Example","product":"ROR","bic":"BPKOPLPW"}}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountDetails(t.Context(), GetAccountDetailsParams{AccountID: accountID})

		require.NoError(t, err)
		assert.Equal(t, "Jane Example", response.OwnerName)
		assert.Equal(t, "ROR", response.Product)
		assert.Equal(t, "BPKOPLPW", response.BIC)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ownerName":"Jane Minimal"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetAccountDetails(t.Context(), GetAccountDetailsParams{AccountID: "acc-min"})

		require.NoError(t, err)
		assert.Equal(t, "Jane Minimal", response.OwnerName)
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
