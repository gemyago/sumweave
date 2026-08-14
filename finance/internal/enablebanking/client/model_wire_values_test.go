package client

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedWireValues(t *testing.T) {
	t.Run("clearing member ID retains either official primitive form", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload string
		}{
			{name: "string", payload: `"member"`},
			{name: "number", payload: `20368`},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				var value ClearingMemberID
				require.NoError(t, json.Unmarshal([]byte(testCase.payload), &value))
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				assert.JSONEq(t, testCase.payload, string(encoded))
			})
		}

		for _, payload := range [][]byte{nil, []byte(`{}`), []byte(`"`)} {
			var value ClearingMemberID
			require.Error(t, value.UnmarshalJSON(payload))
		}
		_, err := json.Marshal(ClearingMemberID{})
		require.Error(t, err)
	})

	t.Run("additional identification preserves collection shape and rejects invalid values", func(t *testing.T) {
		for _, payload := range [][]byte{nil, []byte(`true`), []byte(`{`), []byte(`[true]`)} {
			var value AdditionalIdentifications
			require.Error(t, value.UnmarshalJSON(payload))
		}
		_, err := json.Marshal(AdditionalIdentifications{})
		require.Error(t, err)
		encoded, err := json.Marshal(AdditionalIdentifications{Array: true, Values: []GenericIdentification{}})
		require.NoError(t, err)
		assert.JSONEq(t, `[]`, string(encoded))
	})

	t.Run("nullable continuation retains explicit strings and null", func(t *testing.T) {
		for _, payload := range []string{`null`, `""`, `"next"`} {
			var value NullableString
			require.NoError(t, json.Unmarshal([]byte(payload), &value))
			assert.True(t, value.Present)
			encoded, err := json.Marshal(&value)
			require.NoError(t, err)
			assert.JSONEq(t, payload, string(encoded))
			assert.Equal(t, value.Value, value.String())
		}
		var value NullableString
		require.Error(t, value.UnmarshalJSON([]byte(`1`)))
	})

	t.Run("transaction response ignores nested undocumented values without raw storage", func(t *testing.T) {
		var response GetAccountTransactionsResponse
		require.Error(t, response.UnmarshalJSON(nil))
		require.NoError(
			t,
			json.Unmarshal(
				[]byte(`{"undocumented":{"array":[true,{"value":"ignored"}]},"transactions":[]}`),
				&response,
			),
		)
		encoded, err := json.Marshal(response)
		require.NoError(t, err)
		assert.JSONEq(t, `{"transactions":[]}`, string(encoded))

		for _, payload := range []string{`[]`, `{"transactions":{}}`, `{"continuation_key":1}`, `{"transactions":[]`} {
			require.Error(t, json.Unmarshal([]byte(payload), &response))
		}
	})

	t.Run("discard JSON value handles primitive, object, array, and decoder failures", func(t *testing.T) {
		for _, payload := range []string{`true`, `{"field":[]}`, `[{}]`} {
			decoder := json.NewDecoder(bytes.NewBufferString(payload))
			require.NoError(t, discardJSONValue(decoder))
		}
		decoder := json.NewDecoder(bytes.NewBufferString(`{`))
		require.Error(t, discardJSONValue(decoder))
	})
}
