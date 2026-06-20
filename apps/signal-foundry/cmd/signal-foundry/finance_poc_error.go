package main

import (
	"fmt"
	"regexp"
	"strings"
)

const financePOCProviderErrorExcerptLimit = 160

var (
	financePOCPrivateKeyPattern = regexp.MustCompile(
		`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`,
	)
	financePOCBearerPattern    = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+/=]+`)
	financePOCJWTPattern       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9._-]+\.[A-Za-z0-9._-]+`)
	financePOCJSONTokenPattern = regexp.MustCompile(`(?i)("(?:token|secret)"\s*:\s*")([^"]*)(")`)
	financePOCTokenPattern     = regexp.MustCompile(`(?i)\b(token|secret)\b([=:]\s*|\s+)([^\s,;]+)`)
)

func newFinancePOCProviderResponseError(provider string, operation string, statusCode int, body []byte) error {
	excerpt := sanitizeFinancePOCText(string(body))
	excerpt = strings.TrimSpace(excerpt)
	if len(excerpt) > financePOCProviderErrorExcerptLimit {
		excerpt = excerpt[:financePOCProviderErrorExcerptLimit] + "..."
	}
	return fmt.Errorf(
		"%s %s failed with status %d: %s",
		provider,
		operation,
		statusCode,
		excerpt,
	)
}

func sanitizeFinancePOCText(value string) string {
	sanitized := financePOCPrivateKeyPattern.ReplaceAllString(value, "[REDACTED_PRIVATE_KEY]")
	sanitized = financePOCBearerPattern.ReplaceAllString(sanitized, "Bearer [REDACTED]")
	sanitized = financePOCJWTPattern.ReplaceAllString(sanitized, "[REDACTED_JWT]")
	sanitized = financePOCJSONTokenPattern.ReplaceAllString(sanitized, `$1[REDACTED]$3`)
	sanitized = financePOCTokenPattern.ReplaceAllString(sanitized, "$1$2[REDACTED]")
	return sanitized
}
