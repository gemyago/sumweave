package finance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderLinkRoutingHelpers(t *testing.T) {
	t.Run("rejects unsupported link methods by provider", func(t *testing.T) {
		_, err := configuredBankProviderName(bankProviderMonobank, bankLinkMethodRedirect)
		require.ErrorIs(t, err, ErrUnsupportedBankLinkingMethod)
		require.ErrorContains(t, err, bankProviderMonobank)

		_, err = configuredBankProviderName(bankProviderPKO, bankLinkMethodToken)
		require.ErrorIs(t, err, ErrUnsupportedBankLinkingMethod)
		require.ErrorContains(t, err, bankProviderPKO)
	})

	t.Run("uses direct provider error when connector name is not distinct", func(t *testing.T) {
		err := bankProviderNotConfiguredForBankError(bankProviderMonobank, bankProviderMonobank)
		require.ErrorIs(t, err, ErrBankProviderNotConfigured)
		require.ErrorContains(t, err, bankProviderMonobank)
	})
}
