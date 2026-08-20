package finance

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderLinkRoutingHelpers(t *testing.T) {
	t.Run("exposes linking operations without the retired sync operation", func(t *testing.T) {
		providerType := reflect.TypeFor[BankConnectionProvider]()

		_, hasSync := providerType.MethodByName("Sync")
		assert.False(t, hasSync)
	})

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

	t.Run("keeps linking defaults explicit", func(t *testing.T) {
		provider, err := configuredBankProviderName(bankProviderMonobank, bankLinkMethodToken)
		require.NoError(t, err)
		require.Equal(t, bankProviderMonobank, provider)
		provider, err = configuredBankProviderName(bankProviderPKO, bankLinkMethodRedirect)
		require.NoError(t, err)
		require.Equal(t, bankConnectorEnableBanking, provider)
		_, err = configuredBankProviderName("unsupported", bankLinkMethodToken)
		require.ErrorIs(t, err, ErrUnsupportedBankProvider)
		require.Nil(t, timePtrOrNil(time.Time{}))
		require.NotNil(t, timePtrOrNil(time.Now()))
	})
}
