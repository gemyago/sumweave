package finance

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferDetailService(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, effectiveAt time.Time) domain.Transaction {
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   "account-" + fake.UUID().V4(),
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -int64(fake.IntBetween(1, 10000)),
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: effectiveAt,
			CreatedAt:   effectiveAt,
			UpdatedAt:   effectiveAt,
		}
	}
	makeService := func(t *testing.T, fake faker.Faker) (*TransferDetailService, *persistence.TransactionTagStore, string, string, time.Time) {
		t.Helper()
		database := openTestDatabase(t)
		coreStore := persistence.NewStore(database)
		transactions := persistence.NewTransactionTagStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		userID := "user-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.FixedZone("detail", 3*60*60))
		_, err := coreStore.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{TenantID: tenantID, UserID: userID, JoinedAt: now, CreatedAt: now},
		)
		require.NoError(t, err)
		return NewTransferDetailService(
			persistence.NewTransferCandidateStore(database),
		), transactions, tenantID, userID, now
	}

	t.Run("validates candidate requests before reads", func(t *testing.T) {
		fake := faker.New()
		service, _, tenantID, userID, now := makeService(t, fake)
		_, err := service.ListTransferCandidates(t.Context(), ListTransferCandidatesParams{
			ActorUserID: userID, TenantID: tenantID, TransactionID: "transaction-" + fake.UUID().V4(),
			EffectiveFrom: now, EffectiveBefore: now, Limit: 0, Offset: -1,
		})
		require.ErrorIs(t, err, ErrInvalidTransferCandidateQuery)
	})

	t.Run("returns visible candidates after tenant authorization", func(t *testing.T) {
		fake := faker.New()
		service, transactions, tenantID, userID, now := makeService(t, fake)
		source := makeTransaction(fake, tenantID, now)
		sameAccountCandidate := makeTransaction(fake, tenantID, now.Add(90*time.Minute))
		sameAccountCandidate.AccountID = source.AccountID
		candidate := makeTransaction(fake, tenantID, now.Add(time.Hour))
		for _, transaction := range []domain.Transaction{source, sameAccountCandidate, candidate} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		items, err := service.ListTransferCandidates(t.Context(), ListTransferCandidatesParams{
			ActorUserID: userID, TenantID: tenantID, TransactionID: source.ID,
			EffectiveFrom: now.Add(-time.Hour), EffectiveBefore: now.Add(2 * time.Hour), Limit: 20,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{candidate.ID}, transferDetailTransactionIDs(items))
	})

	t.Run("returns the one valid matched partner and bounds unmatched or malformed states", func(t *testing.T) {
		fake := faker.New()
		service, transactions, tenantID, userID, now := makeService(t, fake)
		groupID := "group-" + fake.UUID().V4()
		matchedAt := now.Add(time.Minute)
		source := makeTransaction(fake, tenantID, now)
		source.Kind, source.TransferGroupID, source.TransferMatchedAt = domain.TransactionKindTransfer, &groupID, &matchedAt
		partner := makeTransaction(fake, tenantID, now.Add(time.Minute))
		partner.Kind, partner.TransferGroupID, partner.TransferMatchedAt = domain.TransactionKindTransfer, &groupID, &matchedAt
		for _, transaction := range []domain.Transaction{source, partner} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		actual, err := service.GetTransferPartner(
			t.Context(),
			GetTransferPartnerParams{ActorUserID: userID, TenantID: tenantID, TransactionID: source.ID},
		)
		require.NoError(t, err)
		assert.Equal(t, partner.ID, actual.ID)

		unmatched := makeTransaction(fake, tenantID, now)
		_, err = transactions.SaveTransaction(t.Context(), unmatched)
		require.NoError(t, err)
		_, err = service.GetTransferPartner(
			t.Context(),
			GetTransferPartnerParams{ActorUserID: userID, TenantID: tenantID, TransactionID: unmatched.ID},
		)
		require.ErrorIs(t, err, ErrTransferPartnerNotFound)

		third := makeTransaction(fake, tenantID, now.Add(2*time.Minute))
		third.Kind, third.TransferGroupID, third.TransferMatchedAt = domain.TransactionKindTransfer, &groupID, &matchedAt
		_, err = transactions.SaveTransaction(t.Context(), third)
		require.NoError(t, err)
		_, err = service.GetTransferPartner(
			t.Context(),
			GetTransferPartnerParams{ActorUserID: userID, TenantID: tenantID, TransactionID: source.ID},
		)
		require.ErrorIs(t, err, ErrInvalidTransferPartner)

		malformedGroupID := "group-malformed-" + fake.UUID().V4()
		malformedSource := makeTransaction(fake, tenantID, now.Add(3*time.Minute))
		malformedSource.Kind, malformedSource.TransferGroupID, malformedSource.TransferMatchedAt = domain.TransactionKindTransfer, &malformedGroupID, &matchedAt
		malformedPartner := makeTransaction(fake, tenantID, now.Add(4*time.Minute))
		malformedPartner.TransferGroupID = &malformedGroupID
		for _, transaction := range []domain.Transaction{malformedSource, malformedPartner} {
			_, err = transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		_, err = service.GetTransferPartner(
			t.Context(),
			GetTransferPartnerParams{ActorUserID: userID, TenantID: tenantID, TransactionID: malformedSource.ID},
		)
		require.ErrorIs(t, err, ErrInvalidTransferPartner)

		missingPartnerGroupID := "group-missing-" + fake.UUID().V4()
		missingPartner := makeTransaction(fake, tenantID, now.Add(5*time.Minute))
		missingPartner.Kind, missingPartner.TransferGroupID, missingPartner.TransferMatchedAt = domain.TransactionKindTransfer, &missingPartnerGroupID, &matchedAt
		_, err = transactions.SaveTransaction(t.Context(), missingPartner)
		require.NoError(t, err)
		_, err = service.GetTransferPartner(
			t.Context(),
			GetTransferPartnerParams{ActorUserID: userID, TenantID: tenantID, TransactionID: missingPartner.ID},
		)
		require.ErrorIs(t, err, ErrTransferPartnerNotFound)
	})

	t.Run("does not expose a source from another tenant", func(t *testing.T) {
		fake := faker.New()
		service, transactions, tenantID, userID, now := makeService(t, fake)
		foreign := makeTransaction(fake, "tenant-foreign-"+fake.UUID().V4(), now)
		_, err := transactions.SaveTransaction(t.Context(), foreign)
		require.NoError(t, err)
		_, err = service.ListTransferCandidates(t.Context(), ListTransferCandidatesParams{
			ActorUserID: userID, TenantID: tenantID, TransactionID: foreign.ID,
			EffectiveFrom: now.Add(-time.Hour), EffectiveBefore: now.Add(time.Hour), Limit: 1,
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})

	t.Run("rejects callers without tenant membership and missing transactions", func(t *testing.T) {
		fake := faker.New()
		service, _, tenantID, userID, _ := makeService(t, fake)
		_, err := service.GetTransferPartner(t.Context(), GetTransferPartnerParams{
			ActorUserID: "user-denied-" + fake.UUID().
				V4(),
			TenantID:      tenantID,
			TransactionID: "transaction-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
		_, err = service.GetTransferPartner(t.Context(), GetTransferPartnerParams{
			ActorUserID: userID, TenantID: tenantID, TransactionID: "transaction-missing-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})
}

func transferDetailTransactionIDs(transactions []domain.Transaction) []string {
	ids := make([]string, 0, len(transactions))
	for _, transaction := range transactions {
		ids = append(ids, transaction.ID)
	}
	return ids
}
