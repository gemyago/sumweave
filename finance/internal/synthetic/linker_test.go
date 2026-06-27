package synthetic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinker(t *testing.T) {
	type linkerDepsStub struct {
		requireTenantMemberErr               error
		saveConnectionSecretID               string
		saveConnectionSecretErr              error
		deleteConnectionSecretErr            error
		saveBankConnectionErr                error
		deleteBankConnectionOwnedMetadataErr error
		saveSyntheticProviderStateErr        error
		savedConnectionSecretProvider        string
		savedConnectionSecretReference       string
		savedConnectionSecretValue           string
		deletedConnectionSecretIDs           []string
		savedBankConnection                  *domain.BankConnection
		deletedBankConnection                *domain.BankConnection
		savedSyntheticProviderState          *domain.SyntheticProviderState
		ids                                  []string
		idIndex                              int
		now                                  time.Time
	}

	makeDeps := func(fake faker.Faker) *linkerDepsStub {
		return &linkerDepsStub{
			saveConnectionSecretID: "secret-" + fake.UUID().V4(),
			ids: []string{
				"account-a-" + fake.UUID().V4(),
				"account-b-" + fake.UUID().V4(),
				"provider-ref-" + fake.UUID().V4(),
				"connection-" + fake.UUID().V4(),
			},
			now: time.Date(2026, time.June, 26, 9, 0, 0, 0, time.UTC),
		}
	}

	newLinker := func(stub *linkerDepsStub) *Linker {
		return NewLinker(LinkerDeps{
			RequireTenantMember: func(_ context.Context, _, _ string) error {
				return stub.requireTenantMemberErr
			},
			SaveConnectionSecret: func(
				_ context.Context,
				provider string,
				reference string,
				secret string,
			) (string, error) {
				stub.savedConnectionSecretProvider = provider
				stub.savedConnectionSecretReference = reference
				stub.savedConnectionSecretValue = secret
				if stub.saveConnectionSecretErr != nil {
					return "", stub.saveConnectionSecretErr
				}
				return stub.saveConnectionSecretID, nil
			},
			DeleteConnectionSecret: func(_ context.Context, secretID string) error {
				stub.deletedConnectionSecretIDs = append(stub.deletedConnectionSecretIDs, secretID)
				return stub.deleteConnectionSecretErr
			},
			SaveBankConnection: func(
				_ context.Context,
				connection domain.BankConnection,
			) (domain.BankConnection, error) {
				copyConnection := connection
				stub.savedBankConnection = &copyConnection
				if stub.saveBankConnectionErr != nil {
					return domain.BankConnection{}, stub.saveBankConnectionErr
				}
				return copyConnection, nil
			},
			DeleteBankConnectionOwnedMetadata: func(
				_ context.Context,
				connection domain.BankConnection,
			) error {
				copyConnection := connection
				stub.deletedBankConnection = &copyConnection
				return stub.deleteBankConnectionOwnedMetadataErr
			},
			SaveSyntheticProviderState: func(
				_ context.Context,
				state domain.SyntheticProviderState,
			) (domain.SyntheticProviderState, error) {
				copyState := state
				stub.savedSyntheticProviderState = &copyState
				if stub.saveSyntheticProviderStateErr != nil {
					return domain.SyntheticProviderState{}, stub.saveSyntheticProviderStateErr
				}
				return copyState, nil
			},
			Now: func() time.Time { return stub.now },
			NewID: func() string {
				if stub.idIndex >= len(stub.ids) {
					return ""
				}
				value := stub.ids[stub.idIndex]
				stub.idIndex++
				return value
			},
		})
	}

	makeParams := func(fake faker.Faker) LinkConfiguredBankConnectionParams {
		return LinkConfiguredBankConnectionParams{
			ActorUserID: "user-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			Provider:    string(domain.ProviderIDSynthetic),
			Accounts: []ConfiguredAccount{
				{Name: "wallet-a-" + fake.Lorem().Word(), Currency: "USD"},
				{Name: "wallet-b-" + fake.Lorem().Word(), Currency: "EUR"},
			},
		}
	}

	t.Run("links configured accounts and persists synthetic provider state", func(t *testing.T) {
		fake := faker.New()
		deps := makeDeps(fake)
		linker := newLinker(deps)

		connection, err := linker.LinkConfiguredBankConnection(t.Context(), makeParams(fake))
		require.NoError(t, err)
		assert.Equal(t, string(domain.ProviderIDSynthetic), connection.Provider)
		assert.Equal(t, ConnectionDisplayName, connection.DisplayName)
		assert.Equal(t, domain.BankConnectionStateActive, connection.State)
		require.NotNil(t, deps.savedBankConnection)
		require.NotNil(t, deps.savedSyntheticProviderState)
		assert.Equal(t, deps.saveConnectionSecretID, connection.SecretID)
		assert.Equal(t, string(domain.ProviderIDSynthetic), deps.savedConnectionSecretProvider)
		assert.Empty(t, deps.savedConnectionSecretValue)
		require.Len(t, deps.savedSyntheticProviderState.Envelope.ConfiguredAccounts, 2)
		assert.Equal(
			t,
			"synthetic-account-"+deps.ids[0],
			deps.savedSyntheticProviderState.Envelope.ConfiguredAccounts[0].Key,
		)
		assert.Equal(
			t,
			"synthetic-account-"+deps.ids[1],
			deps.savedSyntheticProviderState.Envelope.ConfiguredAccounts[1].Key,
		)
		assert.Empty(t, deps.savedSyntheticProviderState.Envelope.WindowHistory)
		assert.Empty(t, deps.savedSyntheticProviderState.Envelope.SequenceCounters)
		assert.Equal(t, deps.now, deps.savedSyntheticProviderState.CreatedAt)
		assert.Equal(t, deps.now, deps.savedSyntheticProviderState.UpdatedAt)
	})

	t.Run("rejects invalid requests before persistence", func(t *testing.T) {
		fake := faker.New()

		t.Run("tenant access failure", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.requireTenantMemberErr = fmt.Errorf("tenant-member-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorIs(t, err, deps.requireTenantMemberErr)
		})

		t.Run("unsupported provider", func(t *testing.T) {
			deps := makeDeps(fake)
			params := makeParams(fake)
			params.Provider = "provider-" + fake.Lorem().Word()
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), params)
			require.ErrorIs(t, err, ErrUnsupportedConfiguredBankProvider)
		})

		t.Run("blank account name", func(t *testing.T) {
			deps := makeDeps(fake)
			params := makeParams(fake)
			params.Accounts[0].Name = " "
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), params)
			require.ErrorIs(t, err, ErrConfiguredBankAccountNameRequired)
		})

		t.Run("blank account currency", func(t *testing.T) {
			deps := makeDeps(fake)
			params := makeParams(fake)
			params.Accounts[0].Currency = " "
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), params)
			require.ErrorIs(t, err, ErrConfiguredBankAccountCurrencyRequired)
		})
	})

	t.Run("cleans up persistence failures", func(t *testing.T) {
		fake := faker.New()

		t.Run("secret save failure", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.saveConnectionSecretErr = fmt.Errorf("save-secret-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorIs(t, err, deps.saveConnectionSecretErr)
		})

		t.Run("bank connection save failure removes secret", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.saveBankConnectionErr = fmt.Errorf("save-connection-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorContains(t, err, "save bank connection")
			assert.Equal(t, []string{deps.saveConnectionSecretID}, deps.deletedConnectionSecretIDs)
		})

		t.Run("bank connection save failure joins secret cleanup error", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.saveBankConnectionErr = fmt.Errorf("save-connection-%s", fake.UUID().V4())
			deps.deleteConnectionSecretErr = fmt.Errorf("delete-secret-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorContains(t, err, "save bank connection")
			require.ErrorContains(t, err, "cleanup synthetic link secret")
		})

		t.Run("state save failure removes linked connection metadata", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.saveSyntheticProviderStateErr = fmt.Errorf("save-state-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorContains(t, err, "save synthetic provider state")
			require.NotNil(t, deps.deletedBankConnection)
			assert.Equal(t, deps.savedBankConnection.ID, deps.deletedBankConnection.ID)
		})

		t.Run("state save failure joins cleanup error", func(t *testing.T) {
			deps := makeDeps(fake)
			deps.saveSyntheticProviderStateErr = fmt.Errorf("save-state-%s", fake.UUID().V4())
			deps.deleteBankConnectionOwnedMetadataErr = fmt.Errorf("cleanup-connection-%s", fake.UUID().V4())
			_, err := newLinker(deps).LinkConfiguredBankConnection(t.Context(), makeParams(fake))
			require.ErrorContains(t, err, "save synthetic provider state")
			require.ErrorContains(t, err, "cleanup synthetic bank connection")
		})
	})

	t.Run("falls back to indexed account keys when id generator is blank", func(t *testing.T) {
		fake := faker.New()
		configuredAccounts, err := makeConfiguredAccounts([]ConfiguredAccount{{
			Name:     "wallet-" + fake.Lorem().Word(),
			Currency: "USD",
		}}, func() string { return "" })
		require.NoError(t, err)
		require.Len(t, configuredAccounts, 1)
		assert.Equal(t, "synthetic-account-1", configuredAccounts[0].Key)
	})
}
