package finance

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderResponseError(t *testing.T) {
	err := &ProviderResponseError{
		Provider:   bankConnectorEnableBanking,
		Operation:  "auth",
		StatusCode: http.StatusUnprocessableEntity,
		Code:       "WRONG_ASPSP_PROVIDED",
		Message:    "Wrong ASPSP name provided",
	}

	assert.Equal(t, "enable-banking auth failed with status 422: Wrong ASPSP name provided", err.Error())
	assert.True(t, err.IsClientError())
	assert.True(t, err.IsTerminal())
	assert.True(t, err.IsEnableBankingWrongASPSP())
	assert.False(t, (*ProviderResponseError)(nil).IsClientError())
	assert.False(t, (*ProviderResponseError)(nil).IsTerminal())
	assert.False(t, (*ProviderResponseError)(nil).IsEnableBankingWrongASPSP())
	assert.False(t, (&ProviderResponseError{StatusCode: http.StatusTooManyRequests}).IsTerminal())
}
