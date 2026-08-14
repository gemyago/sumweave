package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateSession(t *testing.T) {
	fake := faker.New()
	requireNoRawField(t, SessionResponse{})

	t.Run("sends only documented authorize session request and decodes response", func(t *testing.T) {
		code := "code-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "post_sessions_response.json")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			if !assert.NoError(t, decodeErr) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !assert.Len(t, body, 1) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assert.Equal(t, code, body["code"])
			assert.NotContains(t, body, "state")
			assert.NotContains(t, body, "providerReference")
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateSession(
			t.Context(),
			CreateSessionParams{Request: &CreateSessionRequest{Code: code}},
		)

		require.NoError(t, err)
		assert.Equal(t, "session-docs-1", response.SessionID)
		assert.Equal(t, "business", response.PSUType)
		require.NotNil(t, response.Access)
		assert.Equal(t, "2019-08-24T14:15:22Z", response.Access.ValidUntil)
		require.Len(t, response.Accounts, 1)
		assert.Equal(t, "07cc67f4-45d6-494b-adac-09b5cbc7e2b5", response.Accounts[0].UID)
		require.NotNil(t, response.Accounts[0].AccountID)
		assert.Equal(t, "FI0455231152453547", response.Accounts[0].AccountID.IBAN)
		assert.Equal(t, "eur", response.Accounts[0].Currency)
	})

	t.Run("ignores undocumented create session response aliases", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"session-2","accounts":[{"uid":"acc-2"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.CreateSession(
			t.Context(),
			CreateSessionParams{Request: &CreateSessionRequest{Code: "code-min"}},
		)

		require.NoError(t, err)
		assert.Empty(t, response.SessionID)
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
