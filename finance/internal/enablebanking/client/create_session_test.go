package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateSession(t *testing.T) {
	fake := faker.New()

	t.Run("success with all parameters", func(t *testing.T) {
		code := "code-" + fake.UUID().V4()
		state := "state-" + fake.UUID().V4()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			if !assert.NoError(t, decodeErr) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assert.Equal(t, code, body["code"])
			assert.Equal(t, state, body["state"])
			_, _ = w.Write(
				[]byte(
					`{"id":"session-1","externalId":"session-1","secret":"secret-1","accounts":[{"uid":"acc-1"}]}`,
				),
			)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateSession(
			t.Context(),
			CreateSessionParams{Request: &CreateSessionRequest{
				Code:              code,
				State:             state,
				ProviderReference: "ref-1",
			}},
		)

		require.NoError(t, err)
		assert.Equal(t, "session-1", response.SessionID)
		assert.Equal(t, "secret-1", response.Secret)
		require.Len(t, response.Accounts, 1)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"session-2"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateSession(
			t.Context(),
			CreateSessionParams{Request: &CreateSessionRequest{Code: "code-min"}},
		)

		require.NoError(t, err)
		assert.Equal(t, "session-2", response.SessionID)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"bad code"}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateSession(
			t.Context(),
			CreateSessionParams{Request: &CreateSessionRequest{Code: "bad"}},
		)

		require.Error(t, err)
		assert.Nil(t, response)
		assert.ErrorContains(t, err, "create session failed")
	})
}
