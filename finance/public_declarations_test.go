package finance

import (
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicDeclarationsRemainAvailable(t *testing.T) {
	t.Run("public params and sentinel errors stay exported", func(t *testing.T) {
		fake := faker.New()
		effectiveAt := time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC)
		providerOriginalEffectiveAt := effectiveAt.Add(time.Hour)
		providerOriginal := &domain.ProviderTransactionOriginal{
			AmountMinor: 321,
			Currency:    "usd",
			Description: "provider-" + fake.Lorem().Word(),
			EffectiveAt: &providerOriginalEffectiveAt,
		}
		makeTenantID := func() string { return "tenant-" + fake.UUID().V4() }
		makeUserID := func() string { return "user-" + fake.UUID().V4() }
		makeAccountID := func() string { return "account-" + fake.UUID().V4() }
		makeCategoryID := func() string { return "category-" + fake.UUID().V4() }
		makeTagID := func() string { return "tag-" + fake.UUID().V4() }
		makeTransactionID := func(prefix string) string { return prefix + fake.UUID().V4() }
		updateCategoryID := makeCategoryID()

		params := []any{
			CreateTenantParams{
				ActorUserID:     makeUserID(),
				Name:            "tenant-" + fake.Company().Name(),
				DisplayCurrency: "usd",
				SeedDefaults:    true,
			},
			UpdateTenantParams{
				ActorUserID:     makeUserID(),
				TenantID:        makeTenantID(),
				Name:            "tenant-" + fake.Company().Name(),
				DisplayCurrency: "eur",
			},
			ArchiveTenantParams{ActorUserID: makeUserID(), TenantID: makeTenantID()},
			CreateTenantInviteParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				Recipient:   "recipient-" + fake.Internet().User() + "@example.com",
			},
			AcceptTenantInviteParams{ActorUserID: makeUserID(), Code: "code-" + fake.UUID().V4()},
			ListTenantMembersParams{ActorUserID: makeUserID(), TenantID: makeTenantID()},
			ListTenantInvitesParams{ActorUserID: makeUserID(), TenantID: makeTenantID()},
			CreateAccountParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				Name:        "account-" + fake.Lorem().Word(),
				Currency:    "usd",
				Kind:        domain.AccountKindManual,
			},
			UpdateAccountParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				AccountID:   makeAccountID(),
				Name:        "account-" + fake.Lorem().Word(),
			},
			HideAccountParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), AccountID: makeAccountID()},
			UnhideAccountParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), AccountID: makeAccountID()},
			AttachLinkedAccountParams{
				ActorUserID:       makeUserID(),
				TenantID:          makeTenantID(),
				AccountID:         makeAccountID(),
				Provider:          "provider-" + fake.Lorem().Word(),
				ProviderAccountID: "provider-account-" + fake.UUID().V4(),
			},
			GetAccountParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), AccountID: makeAccountID()},
			ListAccountsParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), IncludeHidden: true},
			CreateCategoryParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				Name:        "category-" + fake.Lorem().Word(),
				Kind:        domain.CategoryKindExpense,
			},
			UpdateCategoryParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				CategoryID:  makeCategoryID(),
				Name:        "category-" + fake.Lorem().Word(),
				Kind:        domain.CategoryKindExpense,
			},
			HideCategoryParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				CategoryID:  makeCategoryID(),
			},
			ListCategoriesParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), IncludeHidden: true},
			CreateTagParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), Name: "tag-" + fake.Lorem().Word()},
			UpdateTagParams{
				ActorUserID: makeUserID(),
				TenantID:    makeTenantID(),
				TagID:       makeTagID(),
				Name:        "tag-" + fake.Lorem().Word(),
			},
			HideTagParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), TagID: makeTagID()},
			ListTagsParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), IncludeHidden: true},
			RecordTransactionParams{
				ActorUserID:      makeUserID(),
				TenantID:         makeTenantID(),
				AccountID:        makeAccountID(),
				Source:           domain.TransactionSourceManual,
				Status:           domain.TransactionStatusBooked,
				Kind:             domain.TransactionKindTransfer,
				AmountMinor:      123,
				Currency:         "usd",
				Description:      "transaction-" + fake.Lorem().Word(),
				EffectiveAt:      effectiveAt,
				CategoryID:       makeCategoryID(),
				TransferGroupID:  "group-" + fake.UUID().V4(),
				ProviderOriginal: providerOriginal,
			},
			UpdateTransactionParams{
				ActorUserID:   makeUserID(),
				TenantID:      makeTenantID(),
				TransactionID: makeTransactionID("txn-"),
				Description:   "transaction-" + fake.Lorem().Word(),
				AmountMinor:   456,
				EffectiveAt:   &effectiveAt,
				CategoryID:    updateCategoryID,
			},
			HideTransactionParams{
				ActorUserID:   makeUserID(),
				TenantID:      makeTenantID(),
				TransactionID: makeTransactionID("txn-"),
			},
			GetTransactionParams{
				ActorUserID:   makeUserID(),
				TenantID:      makeTenantID(),
				TransactionID: makeTransactionID("txn-"),
			},
			LinkTransfersParams{
				ActorUserID:         makeUserID(),
				TenantID:            makeTenantID(),
				FirstTransactionID:  makeTransactionID("txn-a-"),
				SecondTransactionID: makeTransactionID("txn-b-"),
			},
			ListTransactionsParams{
				ActorUserID:   makeUserID(),
				TenantID:      makeTenantID(),
				AccountID:     makeAccountID(),
				Source:        domain.TransactionSourceCSV,
				Status:        domain.TransactionStatusPending,
				IncludeHidden: true,
			},
			SummarizeTransactionsParams{ActorUserID: makeUserID(), TenantID: makeTenantID()},
			GetAccountBalanceParams{ActorUserID: makeUserID(), TenantID: makeTenantID(), AccountID: makeAccountID()},
		}

		require.Len(t, params, 30)
		assert.Len(t, []error{
			ErrTenantAccessDenied,
			ErrInviteNotFound,
			ErrInviteAccepted,
			ErrInvalidTenantDisplayCurrency,
			ErrAccountNotFound,
			ErrHiddenAccount,
			ErrCategoryNotFound,
			ErrTagNotFound,
			ErrTransactionNotFound,
			ErrCSVImportAlreadyConfirmed,
			ErrCSVImportAlreadyCompleted,
		}, 11)
	})

	t.Run("internal tenant seed and transfer helpers keep behavior", func(t *testing.T) {
		require.Len(t, defaultTenantCategorySeeds(), 23)
		assert.Equal(t, []string{
			defaultTenantTagTax,
			defaultTenantTagReimburse,
			defaultTenantTagSplit,
			defaultTenantTagBusiness,
			defaultTenantTagSubscription,
			defaultTenantTagTravel,
		}, defaultTenantTags())
		assert.False(t, bookedMatchedTransfer(domain.Transaction{Kind: domain.TransactionKindRegular}))

		groupID := "group-id"
		assert.Equal(t, groupID, existingTransferGroupID(
			domain.Transaction{TransferGroupID: &groupID},
			domain.Transaction{},
		))
	})
}
