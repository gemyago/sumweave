package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetSession(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		sessionID := "session-" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/sessions/"+sessionID, r.URL.Path)
			_, _ = w.Write(
				[]byte(
					`{"id":"` + sessionID + `","displayName":"PKO","accounts":[{"uid":"acc-1","name":"ROR"}]}`,
				),
			)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetSession(t.Context(), GetSessionParams{SessionID: sessionID})

		require.NoError(t, err)
		assert.Equal(t, sessionID, response.SessionID)
		assert.Equal(t, "PKO", response.DisplayName)
		require.Len(t, response.Accounts, 1)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		sessionID := "session-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"` + sessionID + `"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetSession(t.Context(), GetSessionParams{SessionID: sessionID})

		require.NoError(t, err)
		assert.Equal(t, sessionID, response.SessionID)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"missing"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetSession(t.Context(), GetSessionParams{SessionID: "missing"})

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "get session failed")
	})
}
