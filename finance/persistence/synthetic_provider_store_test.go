package persistence

import (
	"testing"
	"time"

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

		loaded, err := store.GetSyntheticProviderState(t.Context(), state.ConnectionID)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, state.ConnectionID, loaded.ConnectionID)
		assert.Equal(t, state.Envelope.Version, loaded.Envelope.Version)
		require.Len(t, loaded.Envelope.ConfiguredAccounts, 2)
		assert.Equal(t, state.Envelope.ConfiguredAccounts, loaded.Envelope.ConfiguredAccounts)
		require.Len(t, loaded.Envelope.WindowHistory, 1)
		assert.Equal(
			t,
			state.Envelope.WindowHistory[0].RepeatCount,
			loaded.Envelope.WindowHistory[0].RepeatCount,
		)
		assert.Equal(
			t,
			state.Envelope.WindowHistory[0].Window.NormalizedStartUTC.UTC(),
			loaded.Envelope.WindowHistory[0].Window.NormalizedStartUTC,
		)
		assert.Equal(
			t,
			state.Envelope.WindowHistory[0].Window.NormalizedEndExclusiveUTC.UTC(),
			loaded.Envelope.WindowHistory[0].Window.NormalizedEndExclusiveUTC,
		)
		require.Len(t, loaded.Envelope.SequenceCounters, 1)
		assert.Equal(
			t,
			state.Envelope.SequenceCounters[0].DayUTC.UTC(),
			loaded.Envelope.SequenceCounters[0].DayUTC,
		)
		assert.Equal(t, state.CreatedAt.UTC(), loaded.CreatedAt)
		assert.Equal(t, state.UpdatedAt.UTC(), loaded.UpdatedAt)
	})

	t.Run("updates and deletes provider state by connection", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		now := time.Date(2026, time.June, 25, 8, 0, 0, 0, time.UTC)
		connectionID := makeRandomSyntheticConnectionID(fake)

		_, err := store.SaveSyntheticProviderState(t.Context(), makeRandomSyntheticProviderState(
			fake,
			withSyntheticProviderConnectionID(connectionID),
			withSyntheticProviderCreatedAt(now),
			withSyntheticProviderUpdatedAt(now),
			withSyntheticProviderSingleAccount(fake, "account", "EUR"),
		))
		require.NoError(t, err)

		updated, err := store.SaveSyntheticProviderState(t.Context(), makeRandomSyntheticProviderState(
			fake,
			withSyntheticProviderConnectionID(connectionID),
			withSyntheticProviderCreatedAt(now),
			withSyntheticProviderUpdatedAt(now.Add(2*time.Hour)),
			withSyntheticProviderSingleAccount(fake, "savings", "GBP"),
			withSyntheticProviderWindowHistoryFrom(now),
		))
		require.NoError(t, err)
		assert.Equal(t, "GBP", updated.Envelope.ConfiguredAccounts[0].Currency)

		loaded, err := store.GetSyntheticProviderState(t.Context(), connectionID)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, updated, *loaded)

		require.NoError(t, store.DeleteSyntheticProviderState(t.Context(), connectionID))

		missing, err := store.GetSyntheticProviderState(t.Context(), connectionID)
		require.ErrorIs(t, err, ErrSyntheticProviderStateNotFound)
		assert.Nil(t, missing)
	})

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

		_, err = store.GetSyntheticProviderState(t.Context(), makeRandomSyntheticConnectionID(fake))
		require.ErrorContains(t, err, "get synthetic provider state")

		err = store.DeleteSyntheticProviderState(t.Context(), makeRandomSyntheticConnectionID(fake))
		require.ErrorContains(t, err, "delete synthetic provider state")
	})
}
