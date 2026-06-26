package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListAccounts(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		token := "token-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
			_, _ = w.Write(
				[]byte(
					`{"state":"reauth_required","reauthReason":"sca_expired","accounts":[{"id":"acc-1","name":"ROR","currency":"PLN"}]}`,
				),
			)
		}))
		defer server.Close()

		client := makeTestClient(server.URL)
		ctx := WithBearerToken(t.Context(), token)

		response, err := client.ListAccounts(ctx, ListAccountsParams{})

		require.NoError(t, err)
		assert.Equal(t, "reauth_required", response.State)
		assert.Equal(t, "sca_expired", response.ReauthReason)
		require.Len(t, response.Accounts, 1)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"accounts":[{"id":"acc-2"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL)

		response, err := client.ListAccounts(t.Context(), ListAccountsParams{})

		require.NoError(t, err)
		require.Len(t, response.Accounts, 1)
		assert.Equal(t, "acc-2", response.Accounts[0].ID)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL)

		response, err := client.ListAccounts(t.Context(), ListAccountsParams{})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "list accounts failed")
	})
}
