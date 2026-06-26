package finance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const providerResponseErrorExcerptLimit = 160

var (
	providerResponsePrivateKeyPattern = regexp.MustCompile(
		`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`,
	)
	providerResponseBearerPattern    = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+/=]+`)
	providerResponseJWTPattern       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9._-]+\.[A-Za-z0-9._-]+`)
	providerResponseJSONTokenPattern = regexp.MustCompile(`(?i)("(?:token|secret)"\s*:\s*")([^"]*)(")`)
	providerResponseTokenPattern     = regexp.MustCompile(`(?i)\b(token|secret)\b([=:]\s*|\s+)([^\s,;]+)`)
)

const enableBankingProviderCodeWrongASPSP = "WRONG_ASPSP_PROVIDED"

type ProviderResponseError struct {
	Provider   string
	Operation  string
	StatusCode int
	Code       string
	Message    string
}

func (e *ProviderResponseError) Error() string {
	return fmt.Sprintf(
		"%s %s failed with status %d: %s",
		e.Provider,
		e.Operation,
		e.StatusCode,
		e.Message,
	)
}

func (e *ProviderResponseError) IsClientError() bool {
	return e != nil && e.StatusCode >= 400 && e.StatusCode < 500
}

func (e *ProviderResponseError) IsEnableBankingWrongASPSP() bool {
	return e != nil && e.Provider == bankConnectorEnableBanking && e.Code == enableBankingProviderCodeWrongASPSP
}

func newProviderResponseError(
	provider string,
	operation string,
	statusCode int,
	body []byte,
) *ProviderResponseError {
	message, code := parseProviderResponseBody(body)
	if message == "" {
		message = sanitizeProviderResponseText(string(body))
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "provider request failed"
	}
	if len(message) > providerResponseErrorExcerptLimit {
		message = message[:providerResponseErrorExcerptLimit] + "..."
	}
	return &ProviderResponseError{
		Provider:   strings.TrimSpace(provider),
		Operation:  strings.TrimSpace(operation),
		StatusCode: statusCode,
		Code:       strings.TrimSpace(code),
		Message:    message,
	}
}

func parseProviderResponseBody(body []byte) (string, string) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	message := ""
	code := ""
	for _, key := range []string{"message", "error_description", "detail", "error"} {
		if value, ok := payload[key].(string); ok {
			value = strings.TrimSpace(sanitizeProviderResponseText(value))
			if value != "" {
				message = value
				break
			}
		}
	}
	if value, ok := payload["error"].(string); ok {
		code = sanitizeProviderResponseText(strings.TrimSpace(value))
	}
	return message, code
}

func sanitizeProviderResponseText(value string) string {
	sanitized := providerResponsePrivateKeyPattern.ReplaceAllString(value, "[REDACTED_PRIVATE_KEY]")
	sanitized = providerResponseBearerPattern.ReplaceAllString(sanitized, "Bearer [REDACTED]")
	sanitized = providerResponseJWTPattern.ReplaceAllString(sanitized, "[REDACTED_JWT]")
	sanitized = providerResponseJSONTokenPattern.ReplaceAllString(sanitized, `$1[REDACTED]$3`)
	sanitized = providerResponseTokenPattern.ReplaceAllString(sanitized, "$1$2[REDACTED]")
	return sanitized
}
