package providers_test

import (
	"log/slog"
	"testing"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/enablebanking"
	"github.com/gemyago/sumweave/finance/internal/monobank"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticRegistriesRealConnectorComposition(t *testing.T) {
	t.Run("keeps technical connectors and product profiles aligned", func(t *testing.T) {
		connectorRegistry := providers.NewStaticConnectorRegistry(
			monobank.NewConnector(monobank.Args{BaseURL: "https://example.test"}),
			enablebanking.NewConnector(enablebanking.Args{
				BaseURL: "https://example.test",
				Logger:  slog.New(slog.DiscardHandler),
			}),
		)

		resolvedMonobank, err := connectorRegistry.Resolve(domain.ProviderConnectorIDMonobank)
		require.NoError(t, err)
		resolvedEnableBanking, err := connectorRegistry.Resolve(domain.ProviderConnectorIDEnableBanking)
		require.NoError(t, err)

		assert.Equal(t, domain.ProviderConnectorIDMonobank, resolvedMonobank.ConnectorID())
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, resolvedEnableBanking.ConnectorID())

		profileRegistry := providers.NewStaticProviderProfileRegistry(
			monobank.Profile(),
			providers.PKOProfile(),
		)

		resolvedMonobankProfile, err := profileRegistry.Resolve(domain.ProviderIDMonobank)
		require.NoError(t, err)
		resolvedPKOProfile, err := profileRegistry.Resolve(domain.ProviderIDPKO)
		require.NoError(t, err)

		assert.Equal(t, monobank.Profile(), resolvedMonobankProfile)
		assert.Equal(t, providers.PKOProfile(), resolvedPKOProfile)
		assert.Equal(t, domain.ProviderConnectorIDEnableBanking, resolvedPKOProfile.ConnectorID)

		_, err = profileRegistry.Resolve(domain.ProviderID(domain.ProviderConnectorIDEnableBanking))
		require.ErrorIs(t, err, providers.ErrProviderNotConfigured)
	})
}
