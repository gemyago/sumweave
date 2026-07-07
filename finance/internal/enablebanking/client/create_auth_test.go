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
	requireNoRawField(t, CreateAuthResponse{})

	t.Run("decodes documented post auth response fields", func(t *testing.T) {
		redirectURL := "https://example.test/callback/" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "post_auth_response.json")

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
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateAuth(
			t.Context(),
			CreateAuthParams{Request: &CreateAuthRequest{
				Access: CreateAuthAccess{
					ValidUntil: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				},
				ASPSP:       CreateAuthASPSP{Name: "PKO Bank Polski", Country: "PL"},
				State:       state,
				RedirectURL: redirectURL,
				PSUType:     "personal",
			}},
		)

		require.NoError(t, err)
		assert.Equal(
			t,
			"https://auth.enablebanking.com/ais/start?sessionid=73100c65-c54d-46a1-87d1-aa3effde435a",
			response.URL,
		)
		assert.Equal(t, response.URL, response.AuthorizationURL)
		assert.Equal(t, "73100c65-c54d-46a1-87d1-aa3effde435a", response.AuthorizationID)
		assert.Equal(t, response.AuthorizationID, response.ID)
		assert.Equal(t, response.AuthorizationID, response.ProviderReference)
		assert.NotEmpty(t, response.PSUIDHash)
	})

	t.Run("ignores undocumented auth response aliases", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `{"authorizationUrl":"https://legacy.example","id":"auth-legacy"}`)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateAuth(t.Context(), CreateAuthParams{Request: &CreateAuthRequest{}})

		require.NoError(t, err)
		assert.Empty(t, response.URL)
		assert.Empty(t, response.AuthorizationURL)
		assert.Empty(t, response.AuthorizationID)
		assert.Empty(t, response.ID)
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
