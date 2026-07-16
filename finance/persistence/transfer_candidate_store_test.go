package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferCandidateStore(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, accountID string, effectiveAt time.Time, createdAt time.Time) domain.Transaction {
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -int64(fake.IntBetween(1, 10000)),
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		}
	}

	t.Run("lists the visible half-open tenant page across accounts in stable order", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		transactions := NewTransactionTagStore(database)
		store := NewTransferCandidateStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		otherTenantID := "tenant-other-" + fake.UUID().V4()
		from := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.FixedZone("candidate", -4*60*60))
		before := from.Add(48 * time.Hour)

		source := makeTransaction(fake, tenantID, "account-source-"+fake.UUID().V4(), from.Add(4*time.Hour), from)
		newest := makeTransaction(
			fake,
			tenantID,
			"account-next-"+fake.UUID().V4(),
			from.Add(30*time.Hour),
			from.Add(2*time.Hour),
		)
		sameEffectiveOlder := makeTransaction(
			fake,
			tenantID,
			"account-third-"+fake.UUID().V4(),
			from.Add(20*time.Hour),
			from.Add(time.Hour),
		)
		sameEffectiveNewer := makeTransaction(
			fake,
			tenantID,
			"account-fourth-"+fake.UUID().V4(),
			from.Add(20*time.Hour),
			from.Add(3*time.Hour),
		)
		atFrom := makeTransaction(fake, tenantID, "account-fifth-"+fake.UUID().V4(), from, from)
		hidden := makeTransaction(fake, tenantID, "account-hidden-"+fake.UUID().V4(), from.Add(8*time.Hour), from)
		hiddenAt := from.Add(time.Hour)
		hidden.HiddenAt = &hiddenAt
		atBefore := makeTransaction(fake, tenantID, "account-before-"+fake.UUID().V4(), before, from)
		foreign := makeTransaction(
			fake,
			otherTenantID,
			"account-foreign-"+fake.UUID().V4(),
			from.Add(12*time.Hour),
			from,
		)
		for _, transaction := range []domain.Transaction{source, newest, sameEffectiveOlder, sameEffectiveNewer, atFrom, hidden, atBefore, foreign} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		page, err := store.ListCandidates(t.Context(), tenantID, source.ID, from, before, 3, 0)
		require.NoError(t, err)
		assert.Equal(t, []string{newest.ID, sameEffectiveNewer.ID, sameEffectiveOlder.ID}, transactionIDs(page))
		assert.Equal(t, newest.AccountID, page[0].AccountID)

		secondPage, err := store.ListCandidates(t.Context(), tenantID, source.ID, from, before, 3, 3)
		require.NoError(t, err)
		assert.Equal(t, []string{atFrom.ID}, transactionIDs(secondPage))
	})

	t.Run("reads source records, membership, and transfer groups", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		coreStore := NewStore(database)
		transactions := NewTransactionTagStore(database)
		store := NewTransferCandidateStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		userID := "user-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.FixedZone("candidate", 5*60*60))
		_, err := coreStore.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{TenantID: tenantID, UserID: userID, JoinedAt: now, CreatedAt: now},
		)
		require.NoError(t, err)
		member, err := store.IsTenantMember(t.Context(), tenantID, userID)
		require.NoError(t, err)
		assert.True(t, member)
		notMember, err := store.IsTenantMember(t.Context(), tenantID, "user-"+fake.UUID().V4())
		require.NoError(t, err)
		assert.False(t, notMember)

		groupID := "group-" + fake.UUID().V4()
		transaction := makeTransaction(fake, tenantID, "account-"+fake.UUID().V4(), now, now)
		transaction.TransferGroupID = &groupID
		_, err = transactions.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)
		loaded, err := store.GetTransaction(t.Context(), transaction.ID)
		require.NoError(t, err)
		assert.Equal(t, transaction.ID, loaded.ID)
		group, err := store.ListTransferGroupTransactions(t.Context(), tenantID, groupID)
		require.NoError(t, err)
		assert.Equal(t, []string{transaction.ID}, transactionIDs(group))
	})

	t.Run("wraps storage failures without returning partial rows", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := NewTransferCandidateStore(database)
		sqlDB, err := database.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = store.IsTenantMember(t.Context(), "tenant-"+fake.UUID().V4(), "user-"+fake.UUID().V4())
		require.Error(t, err)
		_, err = store.GetTransaction(t.Context(), "transaction-"+fake.UUID().V4())
		require.Error(t, err)
		_, err = store.ListCandidates(
			t.Context(),
			"tenant-"+fake.UUID().V4(),
			"transaction-"+fake.UUID().V4(),
			time.Now(),
			time.Now().Add(time.Hour),
			1,
			0,
		)
		require.Error(t, err)
	})
}

func transactionIDs(transactions []domain.Transaction) []string {
	ids := make([]string, 0, len(transactions))
	for _, transaction := range transactions {
		ids = append(ids, transaction.ID)
	}
	return ids
}
