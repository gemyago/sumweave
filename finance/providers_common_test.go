package finance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerStringer string

func (s providerStringer) String() string { return string(s) }

func TestProvidersCommon(t *testing.T) {
	t.Run("extracts values from mixed payload shapes", func(t *testing.T) {
		payload := map[string]any{
			"string": "value",
			"float":  4.2,
			"int64":  int64(7),
			"int":    9,
			"number": json.Number("42"),
			"time":   "2026-06-20T10:00:00Z",
			"items":  []any{map[string]any{"id": "first"}},
		}
		assert.Equal(t, "value", stringValue(payload, "string"))
		assert.Equal(t, int64(4), int64Value(payload, "float"))
		assert.Equal(t, int64(7), int64Value(payload, "int64"))
		assert.Equal(t, int64(9), int64Value(payload, "int"))
		assert.Equal(t, int64(42), int64Value(payload, "number"))
		assert.Equal(t, 42, intValue(payload, "number"))
		assert.Equal(t, "first", stringValueFromFirstObject(payload, "items", "id"))
		require.Len(t, objectSlice(payload, "items"), 1)
		assert.Equal(
			t,
			time.Date(2026, time.June, 20, 10, 0, 0, 0, time.UTC),
			timeValue(payload, "time"),
		)
		assert.NotEmpty(t, mustJSON(payload))
	})

	t.Run("handles missing and invalid values defensively", func(t *testing.T) {
		payload := map[string]any{
			"items":        "bad",
			"number":       "oops",
			"value":        providerStringer("stringer-value"),
			"stringNumber": "91",
		}
		assert.Empty(t, stringValue(payload, "missing"))
		assert.Equal(t, "stringer-value", stringValue(payload, "value"))
		assert.Equal(t, []byte("{}"), mustJSON(make(chan int)))
		assert.Zero(t, int64Value(payload, "number"))
		assert.Equal(t, int64(91), int64Value(payload, "stringNumber"))
		assert.Nil(t, objectSlice(payload, "missing"))
		assert.Nil(t, objectSlice(payload, "items"))
		assert.Empty(t, stringValueFromFirstObject(payload, "missing", "id"))
		assert.True(t, timeValue(payload, "missing").IsZero())
	})
}
