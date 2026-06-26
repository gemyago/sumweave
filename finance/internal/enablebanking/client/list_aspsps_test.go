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

func TestClient_ListASPSPs(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		country := "PL"
		expectedID := "aspsp-" + fake.UUID().V4()
		expectedName := "bank-" + fake.Lorem().Word()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/aspsps", r.URL.Path)
			assert.Equal(t, country, r.URL.Query().Get("country"))
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
			_, _ = fmt.Fprintf(w, `[{"id":%q,"name":%q,"country":%q}]`, expectedID, expectedName, country)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{Country: country})

		require.NoError(t, err)
		require.Len(t, response.ASPSPs, 1)
		assert.Equal(t, expectedID, response.ASPSPs[0].ID)
		assert.Equal(t, expectedName, response.ASPSPs[0].Name)
		assert.Equal(t, country, response.ASPSPs[0].Country)
		assert.Len(t, response.Raw, 1)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		expectedID := "aspsp-" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.Query().Get("country"))
			_, _ = fmt.Fprintf(w, `[{"id":%q}]`, expectedID)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{})

		require.NoError(t, err)
		require.Len(t, response.ASPSPs, 1)
		assert.Equal(t, expectedID, response.ASPSPs[0].ID)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"bad request"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "list aspsps failed")
	})
}
