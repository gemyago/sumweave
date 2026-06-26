package providers

import (
	"fmt"
	"testing"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnectorRegistry struct {
	connector    Connector
	err          error
	resolveCalls []domain.ProviderConnectorID
}

func (r *stubConnectorRegistry) Resolve(connectorID domain.ProviderConnectorID) (Connector, error) {
	r.resolveCalls = append(r.resolveCalls, connectorID)
	if r.err != nil {
		return nil, r.err
	}
	return r.connector, nil
}

func TestWindowSyncExecutor(t *testing.T) {
	makeRequest := func(
		fake faker.Faker,
		providerID domain.ProviderID,
		connectorID domain.ProviderConnectorID,
	) WindowSyncRequest {
		return WindowSyncRequest{
			Connection:      makeRandomProviderConnectionRef(fake, providerID, connectorID),
			Secret:          makeRandomConnectionSecret(fake, providerID),
			RequestedWindow: makeRandomProviderSyncWindow(fake),
			JobID:           "job-" + fake.UUID().V4(),
			Reason:          "manual",
		}
	}

	t.Run("coordinate", func(t *testing.T) {
		t.Run("resolves monobank connections by technical connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank)

			executor := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)

			result, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderConnectorID{domain.ProviderConnectorIDMonobank}, registry.resolveCalls)
			assert.NotEmpty(t, result.RunID)
			assert.Equal(t, domain.ProviderSyncStats{}, result.Stats)
		})

		t.Run("resolves pko connections through enable banking connector id", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking},
			}
			request := makeRequest(fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking)

			executor := NewWindowSyncExecutor(
				WithConnectorRegistry(registry),
				WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
			)

			_, err := executor.Execute(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(
				t,
				[]domain.ProviderConnectorID{domain.ProviderConnectorIDEnableBanking},
				registry.resolveCalls,
			)
		})

		t.Run("fails early for empty connector ids without calling registry", func(t *testing.T) {
			fake := faker.New()
			registry := &stubConnectorRegistry{
				connector: &stubConnector{connectorID: domain.ProviderConnectorIDMonobank},
			}
			request := makeRequest(fake, domain.ProviderIDMonobank, "")

			executor := NewWindowSyncExecutor(WithConnectorRegistry(registry))

			_, err := executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorIDRequired)
			assert.Empty(t, registry.resolveCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})

		t.Run("fails early for unconfigured connector ids before fetch", func(t *testing.T) {
			fake := faker.New()
			unknownConnectorID := domain.ProviderConnectorID("unknown-" + fake.UUID().V4())
			connector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
			registry := &stubConnectorRegistry{
				connector: connector,
				err:       fmt.Errorf("%w: %s", ErrConnectorNotConfigured, unknownConnectorID),
			}
			request := makeRequest(fake, domain.ProviderIDPKO, unknownConnectorID)

			executor := NewWindowSyncExecutor(WithConnectorRegistry(registry))

			_, err := executor.Execute(t.Context(), request)
			require.ErrorIs(t, err, ErrConnectorNotConfigured)
			assert.Equal(t, []domain.ProviderConnectorID{unknownConnectorID}, registry.resolveCalls)
			assert.Zero(t, connector.fetchCalls)
			assert.NotContains(t, err.Error(), request.Secret.Reference)
			assert.NotContains(t, err.Error(), request.Secret.Envelope.Ciphertext)
		})
	})

	t.Run("with connectors wires supported technical connectors through registry-backed setup", func(t *testing.T) {
		fake := faker.New()
		monobankConnector := &stubConnector{connectorID: domain.ProviderConnectorIDMonobank}
		enableBankingConnector := &stubConnector{connectorID: domain.ProviderConnectorIDEnableBanking}

		executor := NewWindowSyncExecutor(
			WithConnectors(monobankConnector, enableBankingConnector, monobankConnector),
			WithRunIDGenerator(func() string { return "run-" + fake.UUID().V4() }),
		)

		_, err := executor.Execute(
			t.Context(),
			makeRequest(fake, domain.ProviderIDMonobank, domain.ProviderConnectorIDMonobank),
		)
		require.NoError(t, err)

		_, err = executor.Execute(
			t.Context(),
			makeRequest(fake, domain.ProviderIDPKO, domain.ProviderConnectorIDEnableBanking),
		)
		require.NoError(t, err)
	})
}
