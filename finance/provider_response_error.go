package finance

import (
	"fmt"
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
