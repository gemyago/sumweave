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

	t.Run("treats connector ids as literal values without trimming", func(t *testing.T) {
		registry := NewStaticConnectorRegistry(
			&stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			&stubConnector{connectorID: domain.ProviderConnectorID("   ")},
		)

		_, err := registry.Resolve(domain.ProviderConnectorID(" " + string(domain.ProviderConnectorIDMonobank) + " "))
		require.ErrorIs(t, err, ErrConnectorNotConfigured)

		resolvedWhitespace, err := registry.Resolve(domain.ProviderConnectorID("   "))
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderConnectorID("   "), resolvedWhitespace.ConnectorID())
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

func TestStaticProviderProfileRegistry(t *testing.T) {
	t.Run("resolves registered finance provider profiles", func(t *testing.T) {
		registry := NewStaticProviderProfileRegistry(
			MonobankProfile(),
			PKOProfile(),
		)

		resolvedMonobank, err := registry.Resolve(domain.ProviderIDMonobank)
		require.NoError(t, err)
		resolvedPKO, err := registry.Resolve(domain.ProviderIDPKO)
		require.NoError(t, err)

		assert.Equal(t, MonobankProfile(), resolvedMonobank)
		assert.Equal(t, PKOProfile(), resolvedPKO)
	})

	t.Run("returns bounded errors for empty and unknown provider ids", func(t *testing.T) {
		fake := faker.New()
		registry := NewStaticProviderProfileRegistry(PKOProfile())

		_, err := registry.Resolve("")
		require.ErrorIs(t, err, ErrProviderIDRequired)

		unknownProviderID := domain.ProviderID("unknown-" + fake.UUID().V4())
		_, err = registry.Resolve(unknownProviderID)
		require.ErrorIs(t, err, ErrProviderNotConfigured)
		assert.ErrorContains(t, err, string(unknownProviderID))
	})

	t.Run("treats provider ids as literal values without trimming", func(t *testing.T) {
		whitespaceProfile := ProviderProfile{
			ProviderID:  domain.ProviderID("   "),
			DisplayName: "whitespace",
		}
		registry := NewStaticProviderProfileRegistry(
			PKOProfile(),
			whitespaceProfile,
		)

		_, err := registry.Resolve(domain.ProviderID(" " + string(domain.ProviderIDPKO) + " "))
		require.ErrorIs(t, err, ErrProviderNotConfigured)

		resolvedWhitespace, err := registry.Resolve(domain.ProviderID("   "))
		require.NoError(t, err)
		assert.Equal(t, whitespaceProfile, resolvedWhitespace)
	})

	t.Run("skips empty and duplicate provider registrations", func(t *testing.T) {
		firstMonobankProfile := MonobankProfile()
		duplicateMonobankProfile := MonobankProfile()
		duplicateMonobankProfile.DisplayName = "Monobank Duplicate"
		emptyProviderProfile := ProviderProfile{DisplayName: "empty"}

		registry := NewStaticProviderProfileRegistry(
			emptyProviderProfile,
			firstMonobankProfile,
			duplicateMonobankProfile,
		)

		resolvedMonobank, err := registry.Resolve(domain.ProviderIDMonobank)
		require.NoError(t, err)
		_, err = registry.Resolve(domain.ProviderIDPKO)
		require.ErrorIs(t, err, ErrProviderNotConfigured)

		assert.Equal(t, firstMonobankProfile, resolvedMonobank)
		assert.NotEqual(t, duplicateMonobankProfile, resolvedMonobank)
	})
}
