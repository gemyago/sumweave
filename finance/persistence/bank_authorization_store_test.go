package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBankAuthorizationPendingStart(t *testing.T) {
	fake := faker.New()
	now := time.Now().Truncate(time.Microsecond)
	store := NewStore(openTestDatabase(t))
	links := NewProviderLinkPersistence(store)
	start := domain.PendingBankConnectionLinkStart{
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
		ExpiresAt: now.Add(time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := links.SavePendingStart(t.Context(), start)
	require.NoError(t, err)
	request := providers.ConsumePendingStartRequest{
		TenantID:    start.TenantID,
		ActorUserID: start.ActorUserID,
		ProviderID:  domain.ProviderID(start.Provider),
		ConnectorID: start.ConnectorID,
		State:       start.State,
		ConsumedAt:  now,
	}
	for _, change := range []func(*providers.ConsumePendingStartRequest){
		func(r *providers.ConsumePendingStartRequest) { r.TenantID = fake.UUID().V4() },
		func(r *providers.ConsumePendingStartRequest) { r.ActorUserID = fake.UUID().V4() },
		func(r *providers.ConsumePendingStartRequest) { r.ProviderID = domain.ProviderID(fake.UUID().V4()) },
		func(r *providers.ConsumePendingStartRequest) {
			r.ConnectorID = domain.ProviderConnectorID(fake.UUID().V4())
		},
		func(r *providers.ConsumePendingStartRequest) { r.State = fake.UUID().V4() },
		func(r *providers.ConsumePendingStartRequest) { r.ConsumedAt = start.ExpiresAt },
	} {
		other := request
		change(&other)
		_, consumeErr := links.ConsumePendingStart(t.Context(), other)
		require.ErrorIs(t, consumeErr, providers.ErrPendingStartNotFound)
	}
	consumed, err := links.ConsumePendingStart(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, start.StartResult.UpdateTarget, consumed.StartResult.UpdateTarget)
	_, err = links.ConsumePendingStart(t.Context(), request)
	require.ErrorIs(t, err, providers.ErrPendingStartNotFound)
}

func TestBankAuthorizationFences(t *testing.T) {
	fake := faker.New()
	store := NewStore(openTestDatabase(t))
	now := time.Now().Truncate(time.Microsecond)
	connection, err := store.SaveBankConnection(
		t.Context(),
		domain.BankConnection{
			ID:                fake.UUID().V4(),
			TenantID:          fake.UUID().V4(),
			Provider:          string(domain.ProviderIDPKO),
			ConnectorID:       domain.ProviderConnectorIDEnableBanking,
			ProviderReference: fake.UUID().V4(),
			State:             domain.BankConnectionStateActive,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	)
	require.NoError(t, err)
	ref := domain.ProviderConnectionRef{
		ConnectionID:      connection.ID,
		ProviderID:        domain.ProviderID(connection.Provider),
		ConnectorID:       connection.ConnectorID,
		ProviderReference: connection.ProviderReference,
	}
	journal := NewProviderSyncStateJournalStore(store, WithSyncStateAuthorizationFence())
	state := domain.ProviderSyncState{
		Connection:   ref,
		Window:       domain.ProviderSyncWindow{Start: now.Add(-time.Hour), End: now},
		AttemptedAt:  &now,
		ErrorSummary: fake.Lorem().Sentence(3),
	}
	require.NoError(t, journal.AppendSyncState(t.Context(), state))
	connection.State = domain.BankConnectionStateReauthRequired
	_, err = store.SaveBankConnection(t.Context(), connection)
	require.NoError(t, err)
	require.ErrorIs(t, journal.AppendSyncState(t.Context(), state), domain.ErrBankReauthRequired)
	projections := NewBankAuthorizationStore(store)
	require.ErrorIs(
		t,
		projections.ProjectSync(t.Context(), BankSyncProjection{Connection: ref, Phase: "started", At: now}),
		domain.ErrBankReauthRequired,
	)
	ref.ConnectionID = fake.UUID().V4()
	state.Connection = ref
	require.ErrorIs(t, journal.AppendSyncState(t.Context(), state), ErrBankConnectionNotFound)
	require.ErrorIs(
		t,
		projections.WithConnection(t.Context(), ref, func(*BankAuthorizationTransaction) error { return nil }),
		ErrBankConnectionNotFound,
	)
	// A cancellation while already inside the claim transaction must not advance due time.
	scheduleStore := &BankConnectionScheduleStore{db: store.db}
	require.NoError(t, scheduleStore.WithTransaction(t.Context(), func(tx *BankConnectionScheduleTransaction) error {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, claimErr := tx.ClaimDue(ctx, connection.ID, now, now.Add(time.Hour), now)
		require.Error(t, claimErr)
		claimed, missingErr := tx.ClaimDue(t.Context(), ref.ConnectionID, now, now.Add(time.Hour), now)
		require.NoError(t, missingErr)
		require.False(t, claimed)
		return nil
	}))
	t.Run("returning deletion reports missing rows and cancellation", func(t *testing.T) {
		_, deleteErr := projections.DeleteConnection(t.Context(), fake.UUID().V4())
		require.ErrorIs(t, deleteErr, ErrBankConnectionNotFound)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, deleteErr = projections.DeleteConnection(ctx, connection.ID)
		require.ErrorIs(t, deleteErr, context.Canceled)
		retained, loadErr := store.GetBankConnection(t.Context(), connection.ID)
		require.NoError(t, loadErr)
		require.Equal(t, connection, *retained)
	})
}
