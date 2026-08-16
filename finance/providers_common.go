package finance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func stringValue(input map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := input[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		}
	}
	return ""
}

func int64Value(input map[string]any, key string) int64 {
	value, ok := input[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func intValue(input map[string]any, key string) int {
	return int(int64Value(input, key))
}

func objectSlice(input map[string]any, key string) []map[string]any {
	value, ok := input[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if typed, castOK := item.(map[string]any); castOK {
			result = append(result, typed)
		}
	}
	return result
}

func timeValue(input map[string]any, key string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, stringValue(input, key))
	return parsed
}

func providerFingerprint(parts ...any) string {
	var joined strings.Builder
	for _, part := range parts {
		_, _ = fmt.Fprintf(&joined, "%v\n", part)
	}
	hash := sha256.Sum256([]byte(joined.String()))
	return hex.EncodeToString(hash[:16])
}

func stringValueFromFirstObject(input map[string]any, key string, nestedKey string) string {
	items := objectSlice(input, key)
	if len(items) == 0 {
		return ""
	}
	return stringValue(items[0], nestedKey)
}
