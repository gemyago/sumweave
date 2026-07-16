package domain

import (
	"encoding/json"
	"strings"
)

// SanitizeProviderEvidenceJSON removes credential-like fields from a JSON payload.
func SanitizeProviderEvidenceJSON(payload []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	return json.Marshal(sanitizeProviderEvidenceValue(value))
}

func sanitizeProviderEvidenceValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			if credentialLikeProviderEvidenceKey(key) {
				continue
			}
			redacted[key] = sanitizeProviderEvidenceValue(item)
		}
		return redacted
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, item := range typed {
			redacted = append(redacted, sanitizeProviderEvidenceValue(item))
		}
		return redacted
	default:
		return value
	}
}

func credentialLikeProviderEvidenceKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		normalized == "authorization" ||
		strings.Contains(normalized, "bearer")
}
