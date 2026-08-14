package client

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocumentationResponseRoundTrip(t *testing.T) {
	// Fixtures are copied from https://enablebanking.com/docs/api/reference/
	// (official Enable Banking API reference, checked 2026-08-14).
	testCases := []struct {
		name    string
		fixture string
		model   any
	}{
		{name: "session accounts", fixture: "post_sessions_response.json", model: &CreateSessionResponse{}},
		{name: "session account data", fixture: "get_session_response.json", model: &SessionResponse{}},
		{name: "account details", fixture: "get_account_details_response.json", model: &GetAccountDetailsResponse{}},
		{name: "account balances", fixture: "get_account_balances_response.json", model: &GetAccountBalancesResponse{}},
		{
			name:    "transaction page and item",
			fixture: "get_account_transactions_response.json",
			model:   &GetAccountTransactionsResponse{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := []byte(readDocsFixture(t, testCase.fixture))
			require.NoError(t, json.Unmarshal(fixture, testCase.model))

			encoded, err := json.Marshal(testCase.model)
			require.NoError(t, err)

			var expected any
			require.NoError(t, json.Unmarshal(fixture, &expected))
			var actual any
			require.NoError(t, json.Unmarshal(encoded, &actual))
			require.Equal(t, expected, actual)
		})
	}
}
