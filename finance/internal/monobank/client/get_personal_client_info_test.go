package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetPersonalClientInfo(t *testing.T) {
	makeClient := func(t *testing.T, server *httptest.Server) *Client {
		t.Helper()

		logger := slog.New(slog.DiscardHandler).With("test", t.Name())

		return NewClient(
			Args{BaseURL: server.URL},
			WithHTTPClient(server.Client()),
			WithLogger(logger),
		)
	}

	fake := faker.New()

	t.Run("success with all parameters and fields", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		accountType := "type-" + fake.Lorem().Word()
		iban := "UA" + fake.RandomStringWithLength(27)
		maskedPanA := "masked-a-" + fake.RandomStringWithLength(4)
		maskedPanB := "masked-b-" + fake.RandomStringWithLength(4)
		responseBody := fmt.Sprintf(
			`{"name":"%s","webHookUrl":"%s","permissions":"%s","accounts":[{"id":"%s","type":"%s","currencyCode":980,"balance":12345,"creditLimit":67890,"maskedPan":["%s","%s"],"iban":"%s"}]}`,
			"name-"+fake.Person().Name(),
			"https://example.test/hooks/"+fake.UUID().V4(),
			"psf-credit-info",
			accountID,
			accountType,
			maskedPanA,
			maskedPanB,
			iban,
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/personal/client-info", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, token, r.Header.Get("X-Token"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(responseBody))
		}))
		defer server.Close()

		client := makeClient(t, server)

		expected := &GetPersonalClientInfoResponse{ClientInfo: &Info{}, RawJSON: []byte(responseBody)}
		require.NoError(t, json.Unmarshal([]byte(responseBody), expected.ClientInfo))

		// Act
		actual, err := client.GetPersonalClientInfo(t.Context(), GetPersonalClientInfoParams{Token: token})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		responseBody := fmt.Sprintf(
			`{"name":"%s","accounts":[{"id":"%s"}]}`,
			"name-"+fake.Person().Name(),
			"account-"+fake.UUID().V4(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			_, _ = w.Write([]byte(responseBody))
		}))
		defer server.Close()

		client := makeClient(t, server)

		expected := &GetPersonalClientInfoResponse{
			ClientInfo: &Info{},
			RawJSON:    []byte(responseBody),
		}
		require.NoError(t, json.Unmarshal([]byte(responseBody), expected.ClientInfo))

		// Act
		actual, err := client.GetPersonalClientInfo(t.Context(), GetPersonalClientInfoParams{Token: token})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("generic api error", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"bad request for ` + token + `"}`))
		}))
		defer server.Close()

		client := makeClient(t, server)

		// Act
		actual, err := client.GetPersonalClientInfo(t.Context(), GetPersonalClientInfoParams{Token: token})

		// Assert
		require.Error(t, err)
		assert.Nil(t, actual)
		require.ErrorContains(t, err, "status 500")
		assert.NotContains(t, err.Error(), token)
	})
	t.Run("request build error", func(t *testing.T) {
		client := NewClient(Args{BaseURL: ":bad"})

		actual, err := client.GetPersonalClientInfo(
			t.Context(),
			GetPersonalClientInfoParams{Token: "token-" + fake.UUID().V4()},
		)

		require.ErrorContains(t, err, "build request")
		assert.Nil(t, actual)
	})

	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		defer server.Close()

		client := makeClient(t, server)

		actual, err := client.GetPersonalClientInfo(
			t.Context(),
			GetPersonalClientInfoParams{Token: "token-" + fake.UUID().V4()},
		)

		require.ErrorContains(t, err, "decode response")
		assert.Nil(t, actual)
	})
}
