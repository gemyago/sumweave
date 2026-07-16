package persistence

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderEvidenceStore(t *testing.T) {
	fake := faker.New()

	t.Run("wraps database failures for provider evidence operations", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewProviderEvidenceStore(database)
		connectionID := "connection-" + fake.UUID().V4()
		tenantID := "tenant-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()
		evidenceID := "evidence-" + fake.UUID().V4()
		now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))

		sqlDB, err := database.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, err = store.IsTenantMember(t.Context(), tenantID, "user-"+fake.UUID().V4())
		require.ErrorContains(t, err, "check provider evidence tenant membership")
		_, err = store.SaveProviderEvidence(t.Context(), domain.ProviderEvidence{
			ID: evidenceID, TenantID: tenantID, ConnectionID: connectionID,
			PayloadJSON: []byte(`{"visible":"safe"}`), CapturedAt: now,
		})
		require.ErrorContains(t, err, "save provider evidence")
		_, err = store.ListAccountProviderEvidence(t.Context(), tenantID, accountID)
		require.ErrorContains(t, err, "check provider evidence account")
		_, err = store.ListTransactionProviderEvidence(t.Context(), tenantID, transactionID)
		require.ErrorContains(t, err, "check provider evidence transaction")
		_, err = store.list(
			t.Context(),
			"finance_account_id = ? AND subject = ?",
			accountID,
			tenantID,
			domain.ProviderEvidenceSubjectAccount,
		)
		require.ErrorContains(t, err, "list provider evidence")
		_, err = store.get(
			t.Context(),
			tenantID,
			"finance_account_id = ? AND id = ? AND subject = ?",
			accountID,
			evidenceID,
			domain.ProviderEvidenceSubjectAccount,
		)
		require.ErrorContains(t, err, "get provider evidence")
		err = store.DeleteProviderEvidenceByConnection(t.Context(), connectionID)
		require.ErrorContains(t, err, "delete provider evidence")
	})

	t.Run("strictly scopes entity reads by tenant subject and entity", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewStore(database)
		evidenceStore := NewProviderEvidenceStore(database)
		now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.FixedZone("test", 2*60*60))
		tenantID := "tenant-" + fake.UUID().V4()
		otherTenantID := "tenant-other-" + fake.UUID().V4()
		accountID := "account-" + fake.UUID().V4()
		otherAccountID := "account-other-" + fake.UUID().V4()
		transactionID := "transaction-" + fake.UUID().V4()

		for _, account := range []domain.Account{
			{
				ID: accountID, TenantID: tenantID, Name: "account-" + fake.Lorem().Word(), Currency: "PLN",
				Kind: domain.AccountKindLinked, CreatedAt: now, UpdatedAt: now,
			},
			{
				ID: otherAccountID, TenantID: otherTenantID, Name: "account-" + fake.Lorem().Word(), Currency: "PLN",
				Kind: domain.AccountKindLinked, CreatedAt: now, UpdatedAt: now,
			},
		} {
			_, err := store.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}
		_, err := store.SaveTransaction(t.Context(), domain.Transaction{
			ID: transactionID, TenantID: tenantID, AccountID: accountID, Source: domain.TransactionSourceProvider,
			Status: domain.TransactionStatusBooked, Kind: domain.TransactionKindRegular, AmountMinor: -123,
			Currency: "PLN", Description: "transaction-" + fake.Lorem().Word(), EffectiveAt: now,
			CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)

		accountEvidence := domain.ProviderEvidence{
			ID: "evidence-account-" + fake.UUID().V4(), TenantID: tenantID,
			ConnectionID: "connection-" + fake.UUID().V4(), FinanceAccountID: accountID,
			Subject: domain.ProviderEvidenceSubjectAccount, Scope: domain.RawPayloadScopeAccount,
			PayloadJSON: []byte(`{"account":"observed"}`), CapturedAt: now,
		}
		transactionEvidence := domain.ProviderEvidence{
			ID:                   "evidence-transaction-" + fake.UUID().V4(),
			TenantID:             tenantID,
			ConnectionID:         accountEvidence.ConnectionID,
			FinanceAccountID:     accountID,
			FinanceTransactionID: transactionID,
			Subject:              domain.ProviderEvidenceSubjectTransaction, Scope: domain.RawPayloadScopeTransaction,
			PayloadJSON: []byte(`{"transaction":"observed"}`), CapturedAt: now,
		}
		connectionEvidence := domain.ProviderEvidence{
			ID: "evidence-connection-" + fake.UUID().V4(), TenantID: tenantID,
			ConnectionID: accountEvidence.ConnectionID, Subject: domain.ProviderEvidenceSubjectConnection,
			Scope: domain.RawPayloadScopeTransaction, PayloadJSON: []byte(`{"page":"observed"}`), CapturedAt: now,
		}
		legacyEvidence := domain.ProviderEvidence{
			ID:                   "evidence-legacy-" + fake.UUID().V4(),
			TenantID:             tenantID,
			ConnectionID:         accountEvidence.ConnectionID,
			FinanceAccountID:     accountID,
			FinanceTransactionID: transactionID,
			Scope:                domain.RawPayloadScopeTransaction,
			PayloadJSON:          []byte(`{"legacy":"retained"}`),
			CapturedAt:           now,
		}
		otherTenantEvidence := domain.ProviderEvidence{
			ID: "evidence-other-tenant-" + fake.UUID().V4(), TenantID: otherTenantID,
			ConnectionID: accountEvidence.ConnectionID, FinanceAccountID: accountID,
			Subject: domain.ProviderEvidenceSubjectAccount, Scope: domain.RawPayloadScopeAccount,
			PayloadJSON: []byte(`{"otherTenant":"hidden"}`), CapturedAt: now,
		}
		for _, evidence := range []domain.ProviderEvidence{
			accountEvidence, transactionEvidence, connectionEvidence, legacyEvidence, otherTenantEvidence,
		} {
			_, saveErr := evidenceStore.SaveProviderEvidence(t.Context(), evidence)
			require.NoError(t, saveErr)
		}

		accountItems, err := evidenceStore.ListAccountProviderEvidence(t.Context(), tenantID, accountID)
		require.NoError(t, err)
		require.Len(t, accountItems, 1)
		assert.Equal(t, accountEvidence.ID, accountItems[0].ID)
		transactionItems, err := evidenceStore.ListTransactionProviderEvidence(t.Context(), tenantID, transactionID)
		require.NoError(t, err)
		require.Len(t, transactionItems, 1)
		assert.Equal(t, transactionEvidence.ID, transactionItems[0].ID)

		_, err = evidenceStore.GetAccountProviderEvidence(t.Context(), tenantID, accountID, transactionEvidence.ID)
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)
		_, err = evidenceStore.GetTransactionProviderEvidence(t.Context(), tenantID, transactionID, legacyEvidence.ID)
		require.ErrorIs(t, err, ErrProviderEvidenceNotFound)
		otherTenantItems, err := evidenceStore.ListAccountProviderEvidence(t.Context(), otherTenantID, otherAccountID)
		require.NoError(t, err)
		assert.Empty(t, otherTenantItems)
	})
}
