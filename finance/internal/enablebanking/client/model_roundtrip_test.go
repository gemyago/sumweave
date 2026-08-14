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
		{name: "start authorization", fixture: "post_auth_response.json", model: &CreateAuthResponse{}},
		{name: "aspsp list", fixture: "get_aspsps_response.json", model: &ListASPSPsResponse{}},
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

	t.Run("preserves documented schema and example conflict forms", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload string
			model   any
		}{
			{
				name:    "numeric clearing member ID from example",
				payload: `{"account_servicer":{"clearing_system_member_id":{"member_id":20368}},"cash_account_type":"","currency":"","identification_hash":"","identification_hashes":[]}`,
				model:   &Account{},
			},
			{
				name:    "object additional identification from example",
				payload: `{"transactions":[{"transaction_amount":{"amount":"","currency":""},"credit_debit_indicator":"","status":"","debtor_account_additional_identification":{"identification":"","scheme_name":""}}]}`,
				model:   &GetAccountTransactionsResponse{},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				require.NoError(t, json.Unmarshal([]byte(testCase.payload), testCase.model))
				encoded, err := json.Marshal(testCase.model)
				require.NoError(t, err)
				require.JSONEq(t, testCase.payload, string(encoded))
			})
		}
	})

	t.Run("preserves documented nullable account and transaction values", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload string
			model   any
		}{
			{
				name:    "account legal age null",
				payload: `{"cash_account_type":"","currency":"","identification_hash":"","identification_hashes":[],"legal_age":null}`,
				model:   &Account{},
			},
			{
				name:    "account legal age false",
				payload: `{"cash_account_type":"","currency":"","identification_hash":"","identification_hashes":[],"legal_age":false}`,
				model:   &Account{},
			},
			{
				name:    "account legal age absent",
				payload: `{"cash_account_type":"","currency":"","identification_hash":"","identification_hashes":[]}`,
				model:   &Account{},
			},
			{
				name:    "transaction ID null",
				payload: `{"transactions":[{"transaction_amount":{"amount":"","currency":""},"credit_debit_indicator":"","status":"","transaction_id":null}]}`,
				model:   &GetAccountTransactionsResponse{},
			},
			{
				name:    "transaction ID empty",
				payload: `{"transactions":[{"transaction_amount":{"amount":"","currency":""},"credit_debit_indicator":"","status":"","transaction_id":""}]}`,
				model:   &GetAccountTransactionsResponse{},
			},
			{
				name:    "transaction ID absent",
				payload: `{"transactions":[{"transaction_amount":{"amount":"","currency":""},"credit_debit_indicator":"","status":""}]}`,
				model:   &GetAccountTransactionsResponse{},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				require.NoError(t, json.Unmarshal([]byte(testCase.payload), testCase.model))
				encoded, err := json.Marshal(testCase.model)
				require.NoError(t, err)
				require.JSONEq(t, testCase.payload, string(encoded))
			})
		}
	})

	t.Run("preserves required start authorization response fields when empty", func(t *testing.T) {
		payload := `{"url":"","authorization_id":"","psu_id_hash":""}`
		response := &CreateAuthResponse{}
		require.NoError(t, json.Unmarshal([]byte(payload), response))
		encoded, err := json.Marshal(response)
		require.NoError(t, err)
		require.JSONEq(t, payload, string(encoded))
	})
}
