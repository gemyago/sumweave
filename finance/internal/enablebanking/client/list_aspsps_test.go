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
	requireNoRawField(t, ASPSP{})
	requireNoRawField(t, ListASPSPsResponse{})

	t.Run("decodes documented get aspsps response shape", func(t *testing.T) {
		country := "PL"
		fixture := readDocsFixture(t, "get_aspsps_response.json")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/aspsps", r.URL.Path)
			assert.Equal(t, country, r.URL.Query().Get("country"))
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{Country: country})

		require.NoError(t, err)
		require.Len(t, response.ASPSPs, 1)
		assert.Equal(t, "Nordea", response.ASPSPs[0].Name)
		assert.Equal(t, "FI", response.ASPSPs[0].Country)
		assert.Equal(t, "https://enablebanking.com/brands/FI/Nordea/", response.ASPSPs[0].Logo)
		assert.False(t, response.ASPSPs[0].Beta)
		require.NotNil(t, response.ASPSPs[0].Payments)
		require.Len(t, *response.ASPSPs[0].Payments, 1)
		assert.False(t, *(*response.ASPSPs[0].Payments)[0].CreditorNameRequired)
	})

	t.Run("preserves provider country without in-place normalization", func(t *testing.T) {
		responseBody := `{"aspsps":[{"name":"","country":" PL ","logo":"","psu_types":[],"auth_methods":[],"maximum_consent_validity":0,"beta":false}]}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, responseBody)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))
		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{})

		require.NoError(t, err)
		assert.Equal(t, " PL ", response.ASPSPs[0].Country)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		expectedName := "aspsp-" + fake.UUID().V4()
		responseFormat := `{"aspsps":[{"name":%q,"country":"","logo":"","psu_types":[],"auth_methods":[],"maximum_consent_validity":0,"beta":false}]}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.Query().Get("country"))
			_, _ = fmt.Fprintf(w, responseFormat, expectedName)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{})

		require.NoError(t, err)
		require.Len(t, response.ASPSPs, 1)
		assert.Equal(t, expectedName, response.ASPSPs[0].Name)
	})

	t.Run("rejects legacy top level array response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"aspsp-1"}]`)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.ListASPSPs(t.Context(), ListASPSPsParams{})

		require.ErrorContains(t, err, "enable banking response decode")
		assert.Nil(t, response)
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
