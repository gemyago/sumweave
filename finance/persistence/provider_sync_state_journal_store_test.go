package persistence

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ providers.SyncStateJournal = (*ProviderSyncStateJournalStore)(nil)

func TestProviderSyncStateJournalStore(t *testing.T) {
	makeStore := func(t *testing.T, now func() time.Time) *Store {
		t.Helper()

		fake := faker.New()
		store, err := NewStore(
			fmt.Sprintf("file:%s?mode=memory&cache=shared", "journal-"+fake.UUID().V4()),
		)
		require.NoError(t, err)
		if now != nil {
			store.now = now
		}
		require.NoError(t, store.Migrate(t.Context()))
		return store
	}

	makeConnection := func(
		t *testing.T,
		fake faker.Faker,
		providerID domain.ProviderID,
		connectorID domain.ProviderConnectorID,
	) domain.ProviderConnectionRef {
		t.Helper()

		return domain.ProviderConnectionRef{
			ConnectionID:      "connection-" + fake.UUID().V4(),
			ProviderID:        providerID,
			ConnectorID:       connectorID,
			ProviderReference: "provider-ref-" + fake.UUID().V4(),
			ExternalID:        "external-" + fake.UUID().V4(),
		}
	}

	makeTimeInLocation := func(fake faker.Faker, location *time.Location) time.Time {
		return time.Date(
			2026,
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 28),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			fake.IntBetween(0, 59),
			0,
			location,
		)
	}

	makeSyncWindow := func(start time.Time, duration time.Duration) domain.ProviderSyncWindow {
		end := start.Add(duration)
		return domain.ProviderSyncWindow{Start: start, End: end}
	}

	makeStats := func(fake faker.Faker) domain.ProviderSyncStats {
		observedTransactions := fake.IntBetween(4, 12)
		return domain.ProviderSyncStats{
			ObservedAccounts:             fake.IntBetween(1, 4),
			ObservedTransactions:         observedTransactions,
			CreatedTransactions:          fake.IntBetween(0, observedTransactions),
			UpdatedTransactions:          fake.IntBetween(0, observedTransactions),
			AmbiguousCreatedTransactions: fake.IntBetween(0, observedTransactions),
		}
	}

	makeState := func(
		connection domain.ProviderConnectionRef,
		attemptedAt *time.Time,
		succeededAt *time.Time,
		window domain.ProviderSyncWindow,
		runID string,
		jobID string,
		errorSummary string,
		stats domain.ProviderSyncStats,
	) domain.ProviderSyncState {
		return domain.ProviderSyncState{
			Connection:     connection,
			AttemptedAt:    attemptedAt,
			SucceededAt:    succeededAt,
			Window:         window,
			RunID:          runID,
			JobID:          jobID,
			ErrorSummary:   errorSummary,
			AggregateStats: stats,
		}
	}

	makeExpectedState := func(
		state domain.ProviderSyncState,
		connection domain.ProviderConnectionRef,
	) domain.ProviderSyncState {
		expected := state
		expected.Connection = connection
		expected.AttemptedAt = normalizeUTCPointer(state.AttemptedAt)
		expected.SucceededAt = normalizeUTCPointer(state.SucceededAt)
		expected.Window = domain.ProviderSyncWindow{
			Start: normalizeUTC(state.Window.Start),
			End:   normalizeUTC(state.Window.End),
		}
		return expected
	}

	t.Run("returns nil when connection has no state", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t, nil)
		journalStore := NewProviderSyncStateJournalStore(store)
		connection := makeConnection(
			t,
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)

		state, err := journalStore.LoadLastState(t.Context(), connection)
		require.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run(
		"appends append-only records with required attempted windows aggregate stats and utc timestamps",
		func(t *testing.T) {
			fake := faker.New()
			warsaw := time.FixedZone("case1-"+fake.UUID().V4(), 2*60*60)
			firstAppendAt := makeTimeInLocation(fake, warsaw)
			secondAppendAt := firstAppendAt.Add(time.Duration(fake.IntBetween(60, 600)) * time.Second)
			appendTimes := []time.Time{firstAppendAt, secondAppendAt}
			appendIndex := 0
			store := makeStore(t, func() time.Time {
				current := appendTimes[appendIndex]
				appendIndex++
				return current
			})
			journalStore := NewProviderSyncStateJournalStore(store)
			connection := makeConnection(
				t,
				fake,
				domain.ProviderIDMonobank,
				domain.ProviderConnectorIDMonobank,
			)
			firstAttemptedAt := makeTimeInLocation(fake, warsaw)
			firstSucceededAt := firstAttemptedAt.Add(time.Duration(fake.IntBetween(60, 900)) * time.Second)
			secondAttemptedAt := firstSucceededAt.Add(time.Duration(fake.IntBetween(60, 900)) * time.Second)
			firstWindow := makeSyncWindow(
				makeTimeInLocation(fake, warsaw),
				time.Duration(fake.IntBetween(1, 5))*24*time.Hour,
			)
			secondWindow := makeSyncWindow(
				firstWindow.End.Add(time.Duration(fake.IntBetween(1, 6))*time.Hour),
				time.Duration(fake.IntBetween(1, 5))*24*time.Hour,
			)
			firstState := makeState(
				connection,
				&firstAttemptedAt,
				&firstSucceededAt,
				firstWindow,
				"run-first-"+fake.UUID().V4(),
				"job-first-"+fake.UUID().V4(),
				"",
				makeStats(fake),
			)
			secondState := makeState(
				connection,
				&secondAttemptedAt,
				nil,
				secondWindow,
				"",
				"job-second-"+fake.UUID().V4(),
				"summary-second-"+fake.Lorem().Sentence(3),
				makeStats(fake),
			)

			require.NoError(t, journalStore.AppendSyncState(t.Context(), firstState))
			require.NoError(t, journalStore.AppendSyncState(t.Context(), secondState))

			var records []providerSyncStateJournalModel
			require.NoError(
				t,
				store.db.WithContext(t.Context()).
					Table((providerSyncStateJournalModel{}).TableName()).
					Where("connection_id = ?", connection.ConnectionID).
					Order("journal_id ASC").
					Find(&records).Error,
			)
			require.Len(t, records, 2)

			assert.Equal(t, firstAppendAt.UTC(), records[0].CreatedAt.UTC())
			assert.Equal(t, firstAttemptedAt.UTC(), records[0].AttemptedAt.UTC())
			assert.Equal(t, firstSucceededAt.UTC(), records[0].SucceededAt.UTC())
			assert.Equal(t, firstWindow.Start.UTC(), records[0].WindowStart.UTC())
			assert.Equal(t, firstWindow.End.UTC(), records[0].WindowEnd.UTC())

			assert.Equal(t, secondAppendAt.UTC(), records[1].CreatedAt.UTC())
			assert.Equal(t, secondAttemptedAt.UTC(), records[1].AttemptedAt.UTC())
			assert.Nil(t, records[1].SucceededAt)
			assert.Equal(t, secondWindow.Start.UTC(), records[1].WindowStart.UTC())
			assert.Equal(t, secondWindow.End.UTC(), records[1].WindowEnd.UTC())
			assert.Equal(t, int64(secondState.AggregateStats.ObservedAccounts), records[1].ObservedAccounts)
			assert.Equal(
				t,
				int64(secondState.AggregateStats.ObservedTransactions),
				records[1].ObservedTransactions,
			)
			assert.Equal(
				t,
				int64(secondState.AggregateStats.CreatedTransactions),
				records[1].CreatedTransactions,
			)
			assert.Equal(
				t,
				int64(secondState.AggregateStats.UpdatedTransactions),
				records[1].UpdatedTransactions,
			)
			assert.Equal(
				t,
				int64(secondState.AggregateStats.AmbiguousCreatedTransactions),
				records[1].AmbiguousCreatedTransactions,
			)
		},
	)

	t.Run("loads the newest state scoped to one connection", func(t *testing.T) {
		fake := faker.New()
		firstAppendAt := makeTimeInLocation(fake, time.UTC)
		secondAppendAt := firstAppendAt.Add(time.Duration(fake.IntBetween(60, 300)) * time.Second)
		thirdAppendAt := secondAppendAt.Add(time.Duration(fake.IntBetween(60, 300)) * time.Second)
		appendTimes := []time.Time{firstAppendAt, secondAppendAt, thirdAppendAt}
		appendIndex := 0
		store := makeStore(t, func() time.Time {
			current := appendTimes[appendIndex]
			appendIndex++
			return current
		})
		journalStore := NewProviderSyncStateJournalStore(store)
		connectionOne := makeConnection(
			t,
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		connectionTwo := makeConnection(
			t,
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		firstWindow := makeSyncWindow(
			makeTimeInLocation(fake, time.UTC),
			time.Duration(fake.IntBetween(1, 3))*24*time.Hour,
		)
		secondWindow := makeSyncWindow(
			firstWindow.End.Add(time.Duration(fake.IntBetween(1, 6))*time.Hour),
			time.Duration(fake.IntBetween(1, 3))*24*time.Hour,
		)
		firstAttemptedAt := makeTimeInLocation(fake, time.UTC)
		secondAttemptedAt := firstAttemptedAt.Add(time.Duration(fake.IntBetween(60, 600)) * time.Second)
		thirdAttemptedAt := secondAttemptedAt.Add(time.Duration(fake.IntBetween(60, 600)) * time.Second)
		firstState := makeState(
			connectionOne,
			&firstAttemptedAt,
			&firstAttemptedAt,
			firstWindow,
			"run-first-"+fake.UUID().V4(),
			"job-first-"+fake.UUID().V4(),
			"",
			makeStats(fake),
		)
		secondState := makeState(
			connectionTwo,
			&secondAttemptedAt,
			&secondAttemptedAt,
			firstWindow,
			"run-second-"+fake.UUID().V4(),
			"job-second-"+fake.UUID().V4(),
			"",
			makeStats(fake),
		)
		thirdState := makeState(
			connectionOne,
			&thirdAttemptedAt,
			nil,
			secondWindow,
			"",
			"job-third-"+fake.UUID().V4(),
			"error-third-"+fake.Lorem().Sentence(3),
			makeStats(fake),
		)

		require.NoError(t, journalStore.AppendSyncState(t.Context(), firstState))
		require.NoError(t, journalStore.AppendSyncState(t.Context(), secondState))
		require.NoError(t, journalStore.AppendSyncState(t.Context(), thirdState))

		loadedOne, err := journalStore.LoadLastState(t.Context(), connectionOne)
		require.NoError(t, err)
		require.NotNil(t, loadedOne)
		assert.Equal(t, makeExpectedState(thirdState, connectionOne), *loadedOne)

		loadedTwo, err := journalStore.LoadLastState(t.Context(), connectionTwo)
		require.NoError(t, err)
		require.NotNil(t, loadedTwo)
		assert.Equal(t, makeExpectedState(secondState, connectionTwo), *loadedTwo)
	})

	t.Run("round-trips all sync state fields without loss", func(t *testing.T) {
		fake := faker.New()
		eest := time.FixedZone("case2-"+fake.UUID().V4(), 3*60*60)
		appendedAt := makeTimeInLocation(fake, time.FixedZone("case3-"+fake.UUID().V4(), 2*60*60))
		store := makeStore(t, func() time.Time {
			return appendedAt
		})
		journalStore := NewProviderSyncStateJournalStore(store)
		connection := makeConnection(
			t,
			fake,
			domain.ProviderIDPKO,
			domain.ProviderConnectorIDEnableBanking,
		)
		attemptedAt := makeTimeInLocation(fake, eest)
		succeededAt := attemptedAt.Add(time.Duration(fake.IntBetween(60, 1800)) * time.Second)
		window := makeSyncWindow(
			makeTimeInLocation(fake, eest),
			time.Duration(fake.IntBetween(1, 7))*24*time.Hour,
		)
		state := makeState(
			connection,
			&attemptedAt,
			&succeededAt,
			window,
			"run-"+fake.UUID().V4(),
			"job-"+fake.UUID().V4(),
			"summary-"+fake.Lorem().Sentence(4),
			makeStats(fake),
		)

		require.NoError(t, journalStore.AppendSyncState(t.Context(), state))

		loadedState, err := journalStore.LoadLastState(t.Context(), connection)
		require.NoError(t, err)
		require.NotNil(t, loadedState)
		assert.Equal(t, makeExpectedState(state, connection), *loadedState)
	})

	t.Run("wraps append and load database errors with journal-specific context", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t, nil)
		journalStore := NewProviderSyncStateJournalStore(store)
		connection := makeConnection(
			t,
			fake,
			domain.ProviderIDMonobank,
			domain.ProviderConnectorIDMonobank,
		)
		attemptedAt := makeTimeInLocation(fake, time.UTC)
		state := makeState(
			connection,
			&attemptedAt,
			&attemptedAt,
			makeSyncWindow(attemptedAt.Add(-24*time.Hour), 24*time.Hour),
			"run-"+fake.UUID().V4(),
			"job-"+fake.UUID().V4(),
			"",
			makeStats(fake),
		)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		appendErr := journalStore.AppendSyncState(t.Context(), state)
		require.Error(t, appendErr)
		require.ErrorContains(t, appendErr, "append provider sync state journal")

		loadErrState, loadErr := journalStore.LoadLastState(t.Context(), connection)
		require.Error(t, loadErr)
		assert.Nil(t, loadErrState)
		assert.ErrorContains(t, loadErr, "load provider sync state journal")
	})
}
