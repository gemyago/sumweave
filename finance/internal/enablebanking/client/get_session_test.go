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

func TestClient_GetSession(t *testing.T) {
	fake := faker.New()

	t.Run("decodes documented get session response fields", func(t *testing.T) {
		sessionID := "session-" + fake.UUID().V4()
		fixture := readDocsFixture(t, "get_session_response.json")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/sessions/"+sessionID, r.URL.Path)
			_, _ = fmt.Fprint(w, fixture)
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetSession(t.Context(), GetSessionParams{SessionID: sessionID})

		require.NoError(t, err)
		assert.Equal(t, "AUTHORIZED", response.Status)
		assert.Equal(t, []string{"497f6eca-6276-4993-bfeb-53cbbbba6f08"}, response.Accounts)
		require.Len(t, response.AccountsData, 1)
		assert.Equal(t, "497f6eca-6276-4993-bfeb-53cbbbba6f08", response.AccountsData[0].UID)
	})

	t.Run("rejects undocumented session account object array", func(t *testing.T) {
		sessionID := "session-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"accounts":[{"uid":"acc-1"}]}`))
		}))
		defer server.Close()

		client := makeTestClient(server.URL, withSignedAuth(t))

		response, err := client.GetSession(t.Context(), GetSessionParams{SessionID: sessionID})

		require.ErrorContains(t, err, "enable banking response decode")
		assert.Nil(t, response)
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
