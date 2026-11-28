package providers

import (
	"testing"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLoadSyncConnection(t *testing.T) {
	fake := faker.New()
	for _, state := range []domain.BankConnectionState{domain.BankConnectionStateActive, domain.BankConnectionStateReauthRequired} {
		t.Run(string(state), func(t *testing.T) {
			expected := makeRandomProviderConnectionRef(
				fake,
				domain.ProviderIDPKO,
				domain.ProviderConnectorIDEnableBanking,
			)
			connection := domain.BankConnection{
				ID:                expected.ConnectionID,
				ProviderReference: expected.ProviderReference,
				State:             state,
			}
			store := NewMockWindowSyncApplyStore(t)
			store.EXPECT().GetBankConnection(mock.Anything, expected.ConnectionID).Return(&connection, nil).Twice()
			_, err := loadSyncConnection(t.Context(), store, expected)
			if state == domain.BankConnectionStateActive {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, domain.ErrBankReauthRequired)
			}
			expected.ProviderReference = fake.UUID().V4()
			_, err = loadSyncConnection(t.Context(), store, expected)
			require.ErrorIs(t, err, domain.ErrBankAuthorizationStale)
		})
	}
}
