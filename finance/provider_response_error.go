package finance

import (
	"fmt"
	"net/http"
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

// IsTerminal reports a provider rejection that will not succeed on a retry
// without a finance-side correction. Timeouts and rate limits remain retryable.
func (e *ProviderResponseError) IsTerminal() bool {
	return e.IsClientError() && e.StatusCode != http.StatusRequestTimeout &&
		e.StatusCode != http.StatusTooManyRequests
}

func (e *ProviderResponseError) IsEnableBankingWrongASPSP() bool {
	return e != nil && e.Provider == bankConnectorEnableBanking && e.Code == enableBankingProviderCodeWrongASPSP
}
