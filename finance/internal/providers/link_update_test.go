package providers

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLinkCoordinatorUpdate(t *testing.T) {
	fake := faker.New()
	makeStart := func() domain.PendingBankConnectionLinkStart {
		return domain.PendingBankConnectionLinkStart{
			ID:          fake.UUID().V4(),
			TenantID:    fake.UUID().V4(),
			ActorUserID: fake.UUID().V4(),
			Provider:    string(domain.ProviderIDPKO),
			ConnectorID: domain.ProviderConnectorIDEnableBanking,
			State:       fake.UUID().V4(),
			StartResult: domain.PendingBankConnectionLinkStartResult{
				UpdateTarget: &domain.BankLinkUpdateTarget{
					ConnectionID:    fake.UUID().V4(),
					ExpectedSession: fake.UUID().V4(),
				},
			},
		}
	}
	for _, stage := range []string{"validate", "exchange", "replace", "success"} {
		t.Run(stage, func(t *testing.T) {
			start := makeStart()
			expectedErr := errors.New(fake.UUID().V4())
			connector := NewMockConnector(t)
			connector.EXPECT().ConnectorID().Return(domain.ProviderConnectorIDEnableBanking)
			connector.EXPECT().
				Capabilities().
				Return(ConnectorCapabilities{SupportsFinishLink: true, RequiresRedirectCode: true})
			pending := NewMockPendingStartStore(t)
			pending.EXPECT().ConsumePendingStart(mock.Anything, mock.Anything).Return(&start, nil).Once()
			replacement := NewMockRedirectLinkReplacement(t)
			var validationErr, exchangeErr, replacementErr error
			switch stage {
			case "validate":
				validationErr = expectedErr
			case "exchange":
				exchangeErr = expectedErr
			case "replace":
				replacementErr = expectedErr
			}
			replacement.EXPECT().ValidateUpdate(mock.Anything, start).Return(validationErr).Once()
			result := LinkResult{ProviderReference: fake.UUID().V4()}
			if stage != "validate" {
				connector.EXPECT().FinishLink(mock.Anything, mock.Anything).Return(result, exchangeErr).Once()
			}
			if stage == "replace" || stage == "success" {
				replacement.EXPECT().
					ReplaceLink(mock.Anything, start, result).
					Return(domain.BankConnection{ID: start.StartResult.UpdateTarget.ConnectionID}, replacementErr).
					Once()
			}

			coordinator, err := NewLinkCoordinator(
				LinkCoordinatorArgs{
					ProviderProfileRegistry: NewStaticProviderProfileRegistry(PKOProfile()),
					ConnectorRegistry:       NewStaticConnectorRegistry(connector),
					PendingStartStore:       pending,
					ConnectionSecretWriter:  NewMockConnectionSecretWriter(t),
					ConnectionStore:         NewMockConnectionStore(t),
					Replacement:             replacement,
					Now:                     time.Now,
				},
			)
			require.NoError(t, err)
			connection, err := coordinator.FinishRedirectLink(
				t.Context(),
				RedirectLinkFinishRequest{
					TenantID:    start.TenantID,
					ActorUserID: start.ActorUserID,
					ProviderID:  domain.ProviderIDPKO,
					State:       start.State,
					Code:        fake.UUID().V4(),
				},
			)
			if stage == "success" {
				require.NoError(t, err)
				require.Equal(t, start.StartResult.UpdateTarget.ConnectionID, connection.ID)
			} else {
				require.ErrorIs(t, err, expectedErr)
			}
			// No restore expectation: any attempted reuse of consumed update state fails.
		})
	}
}
