package providers

import (
	"testing"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticConnectorRegistry(t *testing.T) {
	t.Run("resolves supported connectors by declared connector id", func(t *testing.T) {
		monobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		enableBankingConnector := &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking}

		registry := NewStaticConnectorRegistry(monobankConnector, enableBankingConnector)

		resolvedMonobank, err := registry.Resolve(domain.ProviderConnectorIDMonobank)
		require.NoError(t, err)
		resolvedEnableBanking, err := registry.Resolve(domain.ProviderConnectorIDEnableBanking)
		require.NoError(t, err)

		assert.Same(t, monobankConnector, resolvedMonobank)
		assert.Same(t, enableBankingConnector, resolvedEnableBanking)
	})

	t.Run("returns bounded errors for empty and unknown connector ids", func(t *testing.T) {
		fake := faker.New()
		registry := NewStaticConnectorRegistry(
			&stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
		)

		_, err := registry.Resolve("")
		require.ErrorIs(t, err, ErrConnectorIDRequired)
		assert.NotContains(t, err.Error(), "ciphertext")

		unknownConnectorID := domain.ProviderConnectorID("unknown-" + fake.UUID().V4())
		_, err = registry.Resolve(unknownConnectorID)
		require.ErrorIs(t, err, ErrConnectorNotConfigured)
		assert.ErrorContains(t, err, string(unknownConnectorID))
	})

	t.Run("skips nil empty and duplicate connector registrations", func(t *testing.T) {
		firstMonobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		duplicateMonobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		emptyConnectorID := &stubConnector{forceConnectorID: true}

		registry := NewStaticConnectorRegistry(
			nil,
			emptyConnectorID,
			firstMonobankConnector,
			duplicateMonobankConnector,
		)

		resolvedMonobank, err := registry.Resolve(domain.ProviderConnectorIDMonobank)
		require.NoError(t, err)
		_, err = registry.Resolve(domain.ProviderConnectorIDEnableBanking)
		require.ErrorIs(t, err, ErrConnectorNotConfigured)

		assert.Same(t, firstMonobankConnector, resolvedMonobank)
		assert.NotSame(t, duplicateMonobankConnector, resolvedMonobank)
	})
}
