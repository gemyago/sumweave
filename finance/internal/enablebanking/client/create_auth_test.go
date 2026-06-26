package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateAuth(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		redirectURL := "https://example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		expectedURL := "https://bank.example/auth/" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/auth", r.URL.Path)
			var body map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			if !assert.NoError(t, decodeErr) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assert.Equal(t, state, body["state"])
			assert.Equal(t, redirectURL, body["redirect_url"])
			_, _ = fmt.Fprintf(w, `{"url":%q,"id":"auth-1","providerReference":"ref-1"}`, expectedURL)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateAuth(
			t.Context(),
			CreateAuthParams{Request: &CreateAuthRequest{
				Access: CreateAuthAccess{
					ValidUntil: time.Date(
						2026,
						time.July,
						1,
						0,
						0,
						0,
						0,
						time.UTC,
					).Format(time.RFC3339),
				},
				ASPSP:       CreateAuthASPSP{Name: "PKO Bank Polski", Country: "PL"},
				State:       state,
				RedirectURL: redirectURL,
				PSUType:     "personal",
			}},
		)

		require.NoError(t, err)
		assert.Equal(t, expectedURL, response.AuthorizationURL)
		assert.Equal(t, "auth-1", response.ID)
		assert.Equal(t, "ref-1", response.ProviderReference)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		expectedURL := "https://bank.example/auth/" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"authorizationUrl":%q}`, expectedURL)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateAuth(t.Context(), CreateAuthParams{Request: &CreateAuthRequest{}})

		require.NoError(t, err)
		assert.Equal(t, expectedURL, response.AuthorizationURL)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"wrong aspsp"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateAuth(t.Context(), CreateAuthParams{Request: &CreateAuthRequest{}})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "create auth failed")
	})
}
