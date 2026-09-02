//go:build postgres_test

package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntheticProviderStateStore(t *testing.T) {
	makeStore := func(t *testing.T) *SyntheticProviderStateStore {
		t.Helper()
		database := openTestDatabase(t)
		return NewSyntheticProviderStateStore(database)
	}

	t.Run("round-trips typed versioned synthetic provider state envelopes", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)

		state, err := store.SaveSyntheticProviderState(
			t.Context(),
			makeRandomSyntheticProviderState(fake),
		)
		require.NoError(t, err)

		loaded, err := store.GetSyntheticProviderState(t.Context(), state.ProviderReference)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, state.ProviderReference, loaded.ProviderReference)
		assert.Equal(t, state.Envelope.Version, loaded.Envelope.Version)
		require.Len(t, loaded.Envelope.ConfiguredAccounts, 2)
		assert.Equal(t, state.Envelope.ConfiguredAccounts, loaded.Envelope.ConfiguredAccounts)
		require.Len(t, loaded.Envelope.WindowHistory, 1)
		assert.Equal(
			t,
			state.Envelope.WindowHistory[0].RepeatCount,
			loaded.Envelope.WindowHistory[0].RepeatCount,
		)
		assert.True(
			t,
			state.Envelope.WindowHistory[0].Window.Start.Equal(
				loaded.Envelope.WindowHistory[0].Window.Start,
			),
		)
		assert.True(
			t,
			state.Envelope.WindowHistory[0].Window.End.Equal(
				loaded.Envelope.WindowHistory[0].Window.End,
			),
		)
		require.Len(t, loaded.Envelope.SequenceCounters, 1)
		assert.True(t, state.Envelope.SequenceCounters[0].Instant.Equal(loaded.Envelope.SequenceCounters[0].Instant))
		assert.True(t, state.CreatedAt.Equal(loaded.CreatedAt))
		assert.True(t, state.UpdatedAt.Equal(loaded.UpdatedAt))
	})

	t.Run(
		"updates and deletes provider state by provider reference while keeping duplicates distinct",
		func(t *testing.T) {
			fake := faker.New()
			store := makeStore(t)
			now := time.Date(2026, time.June, 25, 8, 0, 0, 0, time.UTC)
			providerReference := makeRandomSyntheticProviderReference(fake)
			duplicateName := "account-" + fake.Lorem().Word()
			duplicateCurrency := "EUR"
			firstAccountKey := "synthetic-account-a-" + fake.UUID().V4()
			secondAccountKey := "synthetic-account-b-" + fake.UUID().V4()

			_, err := store.SaveSyntheticProviderState(t.Context(), makeRandomSyntheticProviderState(
				fake,
				withSyntheticProviderReference(providerReference),
				withSyntheticProviderCreatedAt(now),
				withSyntheticProviderUpdatedAt(now),
				func(fixture *syntheticProviderStateFixture) {
					fixture.accounts = []domain.SyntheticConfiguredAccount{{
						Key:      firstAccountKey,
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}, {
						Key:      secondAccountKey,
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}}
					fixture.windowHistory = nil
					fixture.sequenceCounters = nil
				},
			))
			require.NoError(t, err)

			updated, err := store.SaveSyntheticProviderState(t.Context(), makeRandomSyntheticProviderState(
				fake,
				withSyntheticProviderReference(providerReference),
				withSyntheticProviderCreatedAt(now),
				withSyntheticProviderUpdatedAt(now.Add(2*time.Hour)),
				withSyntheticProviderWindowHistoryFrom(now),
				func(fixture *syntheticProviderStateFixture) {
					fixture.accounts = []domain.SyntheticConfiguredAccount{{
						Key:      firstAccountKey,
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}, {
						Key:      secondAccountKey,
						Name:     duplicateName,
						Currency: duplicateCurrency,
					}}
				},
			))
			require.NoError(t, err)
			require.Len(t, updated.Envelope.ConfiguredAccounts, 2)
			assert.Equal(t, firstAccountKey, updated.Envelope.ConfiguredAccounts[0].Key)
			assert.Equal(t, secondAccountKey, updated.Envelope.ConfiguredAccounts[1].Key)

			loaded, err := store.GetSyntheticProviderState(t.Context(), providerReference)
			require.NoError(t, err)
			require.NotNil(t, loaded)
			assert.Equal(t, updated.Envelope, loaded.Envelope)
			assert.True(t, updated.CreatedAt.Equal(loaded.CreatedAt))
			assert.True(t, updated.UpdatedAt.Equal(loaded.UpdatedAt))

			require.NoError(t, store.DeleteSyntheticProviderState(t.Context(), providerReference))

			missing, err := store.GetSyntheticProviderState(t.Context(), providerReference)
			require.ErrorIs(t, err, ErrSyntheticProviderStateNotFound)
			assert.Nil(t, missing)
		},
	)

	t.Run("surfaces database errors across synthetic provider state operations", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = store.SaveSyntheticProviderState(
			t.Context(),
			makeRandomSyntheticProviderState(fake, withSyntheticProviderEmptyEnvelope()),
		)
		require.ErrorContains(t, err, "save synthetic provider state")

		_, err = store.GetSyntheticProviderState(t.Context(), makeRandomSyntheticProviderReference(fake))
		require.ErrorContains(t, err, "get synthetic provider state")

		err = store.DeleteSyntheticProviderState(t.Context(), makeRandomSyntheticProviderReference(fake))
		require.ErrorContains(t, err, "delete synthetic provider state")
	})
}
