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

func TestClient_GetPersonalStatement(t *testing.T) {
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
		account := "account-" + fake.UUID().V4()
		fromUnix := int64(1718870400)
		toUnix := int64(1718956800)
		responseBody := fmt.Sprintf(
			`[{"id":"%s","time":%d,"description":"%s","mcc":5411,"originalMcc":5411,"hold":true,"amount":-5050,"operationAmount":-5050,"currencyCode":980,"commissionRate":0,"cashbackAmount":5,"balance":150450,"comment":"%s","receiptId":"%s","counterEdrpou":"%s"}]`,
			"txn-"+fake.UUID().V4(),
			fromUnix,
			"desc-"+fake.Lorem().Sentence(3),
			"comment-"+fake.Lorem().Word(),
			"receipt-"+fake.UUID().V4(),
			"edrpou-"+fake.RandomStringWithLength(8),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, fmt.Sprintf("/personal/statement/%s/%d/%d", account, fromUnix, toUnix), r.URL.Path)
			assert.Equal(t, token, r.Header.Get("X-Token"))
			_, _ = w.Write([]byte(responseBody))
		}))
		defer server.Close()

		client := makeClient(t, server)

		expected := &GetPersonalStatementResponse{}
		require.NoError(t, json.Unmarshal([]byte(responseBody), &expected.Items))

		// Act
		actual, err := client.GetPersonalStatement(t.Context(), GetPersonalStatementParams{
			Token:   token,
			Account: account,
			From:    fromUnix,
			To:      toUnix,
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("success with required parameters only", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		account := "account-" + fake.UUID().V4()
		responseBody := fmt.Sprintf(`[{"id":"%s","time":0,"description":""}]`, "txn-"+fake.UUID().V4())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			assert.Equal(t, fmt.Sprintf("/personal/statement/%s/0/0", account), r.URL.Path)
			_, _ = w.Write([]byte(responseBody))
		}))
		defer server.Close()

		client := makeClient(t, server)

		expected := &GetPersonalStatementResponse{}
		require.NoError(t, json.Unmarshal([]byte(responseBody), &expected.Items))

		// Act
		actual, err := client.GetPersonalStatement(t.Context(), GetPersonalStatementParams{
			Token:   token,
			Account: account,
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("generic api error", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		account := "account-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			assert.Equal(t, fmt.Sprintf("/personal/statement/%s/1/2", account), r.URL.Path)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"statement failed for ` + token + `"}`))
		}))
		defer server.Close()

		client := makeClient(t, server)

		// Act
		actual, err := client.GetPersonalStatement(t.Context(), GetPersonalStatementParams{
			Token:   token,
			Account: account,
			From:    1,
			To:      2,
		})

		// Assert
		require.Error(t, err)
		assert.Nil(t, actual)
		require.ErrorContains(t, err, "status 502")
		assert.NotContains(t, err.Error(), token)
	})

	t.Run("decode error", func(t *testing.T) {
		// Arrange
		token := "token-" + fake.UUID().V4()
		account := "account-" + fake.UUID().V4()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.Header.Get("X-Token"))
			assert.Equal(t, fmt.Sprintf("/personal/statement/%s/3/4", account), r.URL.Path)
			_, _ = w.Write([]byte(`{"broken":true}`))
		}))
		defer server.Close()

		client := makeClient(t, server)

		// Act
		actual, err := client.GetPersonalStatement(t.Context(), GetPersonalStatementParams{
			Token:   token,
			Account: account,
			From:    3,
			To:      4,
		})

		// Assert
		require.Error(t, err)
		assert.Nil(t, actual)
		assert.ErrorContains(t, err, "decode response")
	})
}
