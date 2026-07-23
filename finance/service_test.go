package finance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	t.Run("ledger timestamp policy rejects zero", func(t *testing.T) {
		require.Error(t, validateLedgerTimestamp(time.Time{}))
		require.NoError(t, validateLedgerTimestamp(
			time.Date(2026, time.July, 11, 12, 0, 0, 0, time.FixedZone("fixed", 2*60*60)),
		))
		_, err := new(LedgerService).RecordTransaction(
			t.Context(),
			RecordTransactionParams{EffectiveAt: time.Time{}},
		)
		require.Error(t, err)
	})

	timePointer := func(value time.Time) *time.Time { return &value }
	makeService := func(t *testing.T) *Service {
		t.Helper()

		database := openTestDatabase(t)
		store := persistence.NewStore(database)

		return NewService(store)
	}

	makeTenant := func(t *testing.T, service *Service, actorUserID string) domain.Tenant {
		t.Helper()

		fake := faker.New()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
			DisplayCurrency: "USD",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		return tenant
	}

	t.Run("manages tenants memberships invites accounts categories and tags", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()

		ownerUserID := fmt.Sprintf("user-owner-%s", fake.Lorem().Word())
		memberUserID := fmt.Sprintf("user-member-%s", fake.Lorem().Word())
		outsiderUserID := fmt.Sprintf("user-outsider-%s", fake.Lorem().Word())

		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
			DisplayCurrency: "PLN",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, "PLN", tenant.DisplayCurrency)

		ownerTenants, err := service.ListTenantsForUser(t.Context(), ownerUserID)
		require.NoError(t, err)
		require.Len(t, ownerTenants, 1)
		assert.Equal(t, tenant.ID, ownerTenants[0].Tenant.ID)

		categories, err := service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, categories)

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   fmt.Sprintf("recipient-%s@example.com", fake.Internet().User()),
		})
		require.NoError(t, err)

		membership, err := service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: memberUserID,
			Code:        invite.Code,
		})
		require.NoError(t, err)
		assert.Equal(t, tenant.ID, membership.TenantID)
		assert.Equal(t, memberUserID, membership.UserID)

		memberTenants, err := service.ListTenantsForUser(t.Context(), memberUserID)
		require.NoError(t, err)
		require.Len(t, memberTenants, 1)
		assert.Equal(t, tenant.ID, memberTenants[0].Tenant.ID)

		members, err := service.ListTenantMembers(t.Context(), ListTenantMembersParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, members, 2)
		assert.ElementsMatch(
			t,
			[]string{ownerUserID, memberUserID},
			[]string{members[0].UserID, members[1].UserID},
		)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        fmt.Sprintf("cash-%s", fake.Lorem().Word()),
			Currency:    "PLN",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		updatedAccount, err := service.UpdateAccount(t.Context(), UpdateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Name:        fmt.Sprintf("cash-updated-%s", fake.Lorem().Word()),
		})
		require.NoError(t, err)
		assert.Equal(t, account.ID, updatedAccount.ID)
		assert.Equal(t, domain.AccountKindManual, updatedAccount.Kind)

		linkedAccount, err := service.AttachLinkedAccount(t.Context(), AttachLinkedAccountParams{
			ActorUserID:       ownerUserID,
			TenantID:          tenant.ID,
			AccountID:         account.ID,
			Provider:          fmt.Sprintf("provider-%s", fake.Lorem().Word()),
			ProviderAccountID: fmt.Sprintf("provider-account-%s", fake.Lorem().Word()),
		})
		require.NoError(t, err)
		assert.Equal(t, account.ID, linkedAccount.ID)
		assert.Equal(t, domain.AccountKindLinked, linkedAccount.Kind)
		require.NotNil(t, linkedAccount.LinkedAccount)

		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        fmt.Sprintf("category-%s", fake.Lorem().Word()),
			Kind:        domain.CategoryKindExpense,
		})
		require.NoError(t, err)

		updatedCategory, err := service.UpdateCategory(t.Context(), UpdateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			CategoryID:  category.ID,
			Name:        fmt.Sprintf("category-updated-%s", fake.Lorem().Word()),
			Kind:        domain.CategoryKindIncome,
		})
		require.NoError(t, err)
		expectedCategory := category
		expectedCategory.Name = updatedCategory.Name
		expectedCategory.Kind = domain.CategoryKindIncome
		expectedCategory.UpdatedAt = updatedCategory.UpdatedAt
		assert.Equal(t, expectedCategory, updatedCategory)

		categories, err = service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Contains(t, categories, expectedCategory)

		tag, err := service.CreateTag(t.Context(), CreateTagParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        fmt.Sprintf("tag-%s", fake.Lorem().Word()),
		})
		require.NoError(t, err)

		updatedTag, err := service.UpdateTag(t.Context(), UpdateTagParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			TagID:       tag.ID,
			Name:        fmt.Sprintf("tag-updated-%s", fake.Lorem().Word()),
		})
		require.NoError(t, err)
		assert.Equal(t, tag.ID, updatedTag.ID)

		require.NoError(t, service.HideCategory(t.Context(), HideCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			CategoryID:  category.ID,
		}))
		require.NoError(t, service.HideTag(t.Context(), HideTagParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			TagID:       tag.ID,
		}))
		require.NoError(t, service.HideAccount(t.Context(), HideAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
		}))

		visibleAccounts, err := service.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Empty(t, visibleAccounts)

		visibleCategories, err := service.ListCategories(t.Context(), ListCategoriesParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.NotContains(t, visibleCategories, updatedCategory)

		visibleTags, err := service.ListTags(t.Context(), ListTagsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.NotContains(t, visibleTags, updatedTag)
		assert.NotEmpty(t, visibleTags)

		_, err = service.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
			Recipient:   fmt.Sprintf("recipient-two-%s@example.com", fake.Internet().User()),
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("archives tenants for current members and removes them from active flows", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()
		outsiderUserID := "user-outsider-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		archived, err := service.ArchiveTenant(t.Context(), ArchiveTenantParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, archived.ArchivedAt)
		assert.Equal(t, tenant.ID, archived.ID)

		ownerTenants, err := service.ListTenantsForUser(t.Context(), ownerUserID)
		require.NoError(t, err)
		assert.Empty(t, ownerTenants)

		loadedAccount, err := service.store.GetAccount(t.Context(), account.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedAccount)
		assert.Equal(t, account.ID, loadedAccount.ID)

		_, err = service.ArchiveTenant(t.Context(), ArchiveTenantParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("updates tenants with supported currencies and rejects unsupported ones", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		currentTime := time.Date(2026, time.July, 6, 9, 0, 0, 0, time.UTC)
		service := NewService(store, WithNow(func() time.Time { return currentTime }))

		ownerUserID := "user-owner-" + fake.UUID().V4()
		memberUserID := "user-member-" + fake.UUID().V4()
		outsiderUserID := "user-outsider-" + fake.UUID().V4()

		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "eur",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, "EUR", tenant.DisplayCurrency)

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   "recipient-" + fake.Internet().User() + "@example.com",
		})
		require.NoError(t, err)

		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: memberUserID,
			Code:        invite.Code,
		})
		require.NoError(t, err)

		currentTime = currentTime.Add(2 * time.Hour)
		updatedName := "tenant-updated-" + fake.Company().Name()
		updatedTenant, err := service.UpdateTenant(t.Context(), UpdateTenantParams{
			ActorUserID:     memberUserID,
			TenantID:        tenant.ID,
			Name:            updatedName,
			DisplayCurrency: "pln",
		})
		require.NoError(t, err)
		assert.Equal(t, tenant.ID, updatedTenant.ID)
		assert.Equal(t, updatedName, updatedTenant.Name)
		assert.Equal(t, "PLN", updatedTenant.DisplayCurrency)
		assert.Equal(t, tenant.CreatedAt, updatedTenant.CreatedAt)
		assert.Equal(t, currentTime, updatedTenant.UpdatedAt)
		assert.True(t, updatedTenant.UpdatedAt.After(tenant.UpdatedAt))

		storedTenant, err := service.store.GetTenant(t.Context(), tenant.ID)
		require.NoError(t, err)
		require.NotNil(t, storedTenant)
		assert.Equal(t, updatedTenant, *storedTenant)

		memberTenants, err := service.ListTenantsForUser(t.Context(), memberUserID)
		require.NoError(t, err)
		require.Len(t, memberTenants, 1)
		assert.Equal(t, updatedName, memberTenants[0].Tenant.Name)
		assert.Equal(t, "PLN", memberTenants[0].Tenant.DisplayCurrency)

		_, err = service.UpdateTenant(t.Context(), UpdateTenantParams{
			ActorUserID:     outsiderUserID,
			TenantID:        tenant.ID,
			Name:            "forbidden-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		invalidCreateUserID := "user-invalid-create-" + fake.UUID().V4()
		_, err = service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     invalidCreateUserID,
			Name:            "tenant-invalid-" + fake.Company().Name(),
			DisplayCurrency: "btc",
			SeedDefaults:    true,
		})
		require.ErrorIs(t, err, ErrInvalidTenantDisplayCurrency)

		invalidCreateTenants, err := service.ListTenantsForUser(t.Context(), invalidCreateUserID)
		require.NoError(t, err)
		assert.Empty(t, invalidCreateTenants)

		_, err = service.UpdateTenant(t.Context(), UpdateTenantParams{
			ActorUserID:     ownerUserID,
			TenantID:        tenant.ID,
			Name:            "tenant-invalid-update-" + fake.Company().Name(),
			DisplayCurrency: "",
		})
		require.ErrorIs(t, err, ErrInvalidTenantDisplayCurrency)

		storedTenantAfterInvalidUpdate, err := service.store.GetTenant(t.Context(), tenant.ID)
		require.NoError(t, err)
		require.NotNil(t, storedTenantAfterInvalidUpdate)
		assert.Equal(t, updatedTenant, *storedTenantAfterInvalidUpdate)
	})

	t.Run("manages ledger driven transaction behavior", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := fmt.Sprintf("user-owner-%s", fake.Lorem().Word())
		outsiderUserID := fmt.Sprintf("user-outsider-%s", fake.Lorem().Word())
		tenant := makeTenant(t, service, ownerUserID)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        fmt.Sprintf("checking-%s", fake.Lorem().Word()),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        fmt.Sprintf("category-%s", fake.Lorem().Word()),
			Kind:        domain.CategoryKindExpense,
		})
		require.NoError(t, err)

		record := func(
			source domain.TransactionSource,
			status domain.TransactionStatus,
			kind domain.TransactionKind,
			amount int64,
			effectiveAt time.Time,
			categoryID string,
			transferGroupID string,
		) domain.Transaction {
			t.Helper()

			transaction, recordErr := service.RecordTransaction(
				t.Context(),
				RecordTransactionParams{
					ActorUserID:     ownerUserID,
					TenantID:        tenant.ID,
					AccountID:       account.ID,
					Source:          source,
					Status:          status,
					Kind:            kind,
					AmountMinor:     amount,
					Currency:        "USD",
					Description:     fmt.Sprintf("txn-%s", fake.Lorem().Word()),
					EffectiveAt:     effectiveAt,
					CategoryID:      categoryID,
					TransferGroupID: transferGroupID,
				},
			)
			require.NoError(t, recordErr)
			return transaction
		}

		opening := record(
			domain.TransactionSourceSystem,
			domain.TransactionStatusBooked,
			domain.TransactionKindOpeningBalance,
			100_00,
			time.Date(2026, time.June, 1, 8, 0, 0, 0, time.UTC),
			"",
			"",
		)
		expense := record(
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			domain.TransactionKindRegular,
			-40_00,
			time.Date(2026, time.June, 2, 9, 0, 0, 0, time.UTC),
			category.ID,
			"",
		)
		refund := record(
			domain.TransactionSourceProvider,
			domain.TransactionStatusBooked,
			domain.TransactionKindRefund,
			10_00,
			time.Date(2026, time.June, 3, 9, 0, 0, 0, time.UTC),
			category.ID,
			"",
		)
		transferOut := record(
			domain.TransactionSourceCSV,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			-15_00,
			time.Date(2026, time.June, 4, 9, 0, 0, 0, time.UTC),
			"",
			fmt.Sprintf("transfer-group-%s", fake.Lorem().Word()),
		)
		transferIn := record(
			domain.TransactionSourceCSV,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			15_00,
			time.Date(2026, time.June, 4, 10, 0, 0, 0, time.UTC),
			"",
			*transferOut.TransferGroupID,
		)
		externalTransferOut := record(
			domain.TransactionSourceCSV,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			-7_00,
			time.Date(2026, time.June, 4, 11, 0, 0, 0, time.UTC),
			"",
			"",
		)
		externalTransferIn := record(
			domain.TransactionSourceCSV,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			9_00,
			time.Date(2026, time.June, 4, 12, 0, 0, 0, time.UTC),
			"",
			"",
		)
		groupedUnmatchedTransferOut := record(
			domain.TransactionSourceCSV,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			-11_00,
			time.Date(2026, time.June, 4, 13, 0, 0, 0, time.UTC),
			"",
			fmt.Sprintf("transfer-group-unmatched-%s", fake.Lorem().Word()),
		)
		pending := record(
			domain.TransactionSourceProvider,
			domain.TransactionStatusPending,
			domain.TransactionKindRegular,
			-20_00,
			time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC),
			"",
			"",
		)
		reconciliation := record(
			domain.TransactionSourceSystem,
			domain.TransactionStatusBooked,
			domain.TransactionKindReconciliation,
			5_00,
			time.Date(2026, time.June, 6, 9, 0, 0, 0, time.UTC),
			"",
			"",
		)

		providerTransaction, err := service.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -30_00,
			Currency:    "USD",
			Description: fmt.Sprintf("txn-provider-%s", fake.Lorem().Word()),
			EffectiveAt: time.Date(2026, time.June, 7, 9, 0, 0, 0, time.UTC),
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -32_00,
				Currency:    "USD",
				Description: fmt.Sprintf("provider-original-%s", fake.Lorem().Word()),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, expense.CategoryID)
		assert.Equal(t, category.ID, *expense.CategoryID)
		require.NotNil(t, transferOut.TransferGroupID)
		assert.Equal(t, *transferOut.TransferGroupID, *transferIn.TransferGroupID)

		updatedCategoryID := category.ID
		updatedProviderTransaction, err := service.UpdateTransaction(
			t.Context(),
			UpdateTransactionParams{
				ActorUserID:   ownerUserID,
				TenantID:      tenant.ID,
				TransactionID: providerTransaction.ID,
				Description:   fmt.Sprintf("txn-user-edited-%s", fake.Lorem().Word()),
				AmountMinor:   -31_00,
				EffectiveAt:   timePointer(time.Date(2026, time.June, 8, 11, 0, 0, 0, time.UTC)),
				CategoryID:    updatedCategoryID,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, updatedProviderTransaction.ProviderOriginal)
		require.NotNil(t, updatedProviderTransaction.CategoryID)
		assert.Equal(t, category.ID, *updatedProviderTransaction.CategoryID)
		assert.Equal(
			t,
			time.Date(2026, time.June, 8, 11, 0, 0, 0, time.UTC),
			updatedProviderTransaction.EffectiveAt,
		)
		assert.Equal(
			t,
			providerTransaction.ProviderOriginal.Description,
			updatedProviderTransaction.ProviderOriginal.Description,
		)
		assert.Equal(t, int64(-32_00), updatedProviderTransaction.ProviderOriginal.AmountMinor)

		preservedCategoryTransaction, err := service.UpdateTransaction(
			t.Context(),
			UpdateTransactionParams{
				ActorUserID:   ownerUserID,
				TenantID:      tenant.ID,
				TransactionID: providerTransaction.ID,
				Description:   fmt.Sprintf("txn-category-preserved-%s", fake.Lorem().Word()),
				AmountMinor:   updatedProviderTransaction.AmountMinor,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, preservedCategoryTransaction.CategoryID)
		assert.Equal(t, category.ID, *preservedCategoryTransaction.CategoryID)
		assert.Equal(t, updatedProviderTransaction.EffectiveAt, preservedCategoryTransaction.EffectiveAt)

		clearedCategoryTransaction, err := service.UpdateTransaction(
			t.Context(),
			UpdateTransactionParams{
				ActorUserID:   ownerUserID,
				TenantID:      tenant.ID,
				TransactionID: providerTransaction.ID,
				Description:   preservedCategoryTransaction.Description,
				AmountMinor:   preservedCategoryTransaction.AmountMinor,
				EffectiveAt:   timePointer(preservedCategoryTransaction.EffectiveAt),
				ClearCategory: true,
			},
		)
		require.NoError(t, err)
		assert.Nil(t, clearedCategoryTransaction.CategoryID)
		require.NotNil(t, clearedCategoryTransaction.ProviderOriginal)
		assert.Equal(
			t,
			providerTransaction.ProviderOriginal.Description,
			clearedCategoryTransaction.ProviderOriginal.Description,
		)

		loadedTransaction, err := service.GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID:   ownerUserID,
			TenantID:      tenant.ID,
			TransactionID: providerTransaction.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, clearedCategoryTransaction, loadedTransaction)

		balance, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(35_00), balance.BookedBalanceMinor)

		summary, err := service.SummarizeTransactions(t.Context(), SummarizeTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(24_00), summary.IncomeMinor)
		assert.Equal(t, int64(94_00), summary.ExpenseMinor)
		assert.Equal(t, int64(-70_00), summary.NetMinor)

		transactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 11)
		assert.Equal(t, providerTransaction.ID, transactions[0].ID)
		assert.Equal(t, reconciliation.ID, transactions[1].ID)
		assert.Equal(t, pending.ID, transactions[2].ID)

		providerOnly, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Source:      domain.TransactionSourceProvider,
		})
		require.NoError(t, err)
		require.Len(t, providerOnly, 3)

		bookedOnly, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Status:      domain.TransactionStatusBooked,
		})
		require.NoError(t, err)
		require.Len(t, bookedOnly, 10)

		require.NoError(t, service.HideTransaction(t.Context(), HideTransactionParams{
			ActorUserID:   ownerUserID,
			TenantID:      tenant.ID,
			TransactionID: providerTransaction.ID,
		}))

		visibleTransactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, visibleTransactions, 10)
		assert.NotContains(t, []string{
			visibleTransactions[0].ID,
			visibleTransactions[1].ID,
			visibleTransactions[2].ID,
			visibleTransactions[3].ID,
			visibleTransactions[4].ID,
			visibleTransactions[5].ID,
			visibleTransactions[6].ID,
			visibleTransactions[7].ID,
			visibleTransactions[8].ID,
			visibleTransactions[9].ID,
		}, providerTransaction.ID)

		hiddenTransactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID:   ownerUserID,
			TenantID:      tenant.ID,
			IncludeHidden: true,
		})
		require.NoError(t, err)
		require.Len(t, hiddenTransactions, 11)

		_, err = service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		assert.NotEmpty(t, opening.ID)
		assert.NotEmpty(t, expense.ID)
		assert.NotEmpty(t, refund.ID)
		assert.NotEmpty(t, transferOut.ID)
		assert.NotEmpty(t, transferIn.ID)
		assert.NotEmpty(t, externalTransferOut.ID)
		assert.NotEmpty(t, externalTransferIn.ID)
		assert.NotEmpty(t, groupedUnmatchedTransferOut.ID)
	})

	t.Run("lists and loads accounts with aggregate balances", func(t *testing.T) {
		service := makeService(t)
		fake := faker.New()
		ownerUserID := "user-owner-" + fake.UUID().V4()
		tenant := makeTenant(t, service, ownerUserID)

		checking, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "checking-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		savings, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "savings-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		record := func(accountID string, status domain.TransactionStatus, kind domain.TransactionKind, amount int64) {
			t.Helper()
			_, recordErr := service.RecordTransaction(t.Context(), RecordTransactionParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				AccountID:   accountID,
				Source:      domain.TransactionSourceManual,
				Status:      status,
				Kind:        kind,
				AmountMinor: amount,
				Currency:    "USD",
				Description: "txn-" + fake.Lorem().Word(),
				EffectiveAt: time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC),
			})
			require.NoError(t, recordErr)
		}

		record(checking.ID, domain.TransactionStatusBooked, domain.TransactionKindOpeningBalance, 100_00)
		record(checking.ID, domain.TransactionStatusBooked, domain.TransactionKindRegular, 25_00)
		record(checking.ID, domain.TransactionStatusPending, domain.TransactionKindRegular, -12_00)
		record(savings.ID, domain.TransactionStatusBooked, domain.TransactionKindTransfer, 30_00)
		record(savings.ID, domain.TransactionStatusBooked, domain.TransactionKindReconciliation, 5_00)

		accounts, err := service.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 2)

		assert.Equal(t, int64(125_00), accounts[0].BookedBalanceMinor)
		assert.Equal(t, int64(-12_00), accounts[0].PendingBalanceMinor)
		assert.Equal(t, int64(35_00), accounts[1].BookedBalanceMinor)
		assert.Equal(t, int64(0), accounts[1].PendingBalanceMinor)

		loadedChecking, err := service.GetAccount(t.Context(), GetAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   checking.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(125_00), loadedChecking.BookedBalanceMinor)
		assert.Equal(t, int64(-12_00), loadedChecking.PendingBalanceMinor)

		balance, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   checking.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(125_00), balance.BookedBalanceMinor)
		assert.Equal(t, int64(-12_00), balance.PendingBalanceMinor)
	})

	t.Run(
		"links recorded transfer pairs atomically, persists matched markers, and excludes only matched transfers from summary",
		func(t *testing.T) {
			fake := faker.New()
			now := time.Date(2026, time.June, 20, 17, 0, 0, 0, time.UTC)
			ids := []string{
				"tenant-id",
				"category-1",
				"category-2",
				"category-3",
				"category-4",
				"account-id",
				"transaction-out",
				"transaction-in",
				"transfer-group-linked",
				"transaction-unmatched",
			}
			service := NewService(
				func() *persistence.Store {
					database := openTestDatabase(t)
					store := persistence.NewStore(database)
					return store
				}(),
				WithNow(func() time.Time { return now }),
				WithIDGenerator(func() string {
					if len(ids) == 0 {
						return fake.UUID().V4()
					}
					value := ids[0]
					ids = ids[1:]
					return value
				}),
			)

			ownerUserID := fmt.Sprintf("user-owner-%s", fake.Lorem().Word())
			outsiderUserID := fmt.Sprintf("user-outsider-%s", fake.Lorem().Word())
			tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
				ActorUserID:     ownerUserID,
				Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
				DisplayCurrency: "USD",
				SeedDefaults:    true,
			})
			require.NoError(t, err)

			account, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("account-%s", fake.Lorem().Word()),
				Currency:    "USD",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)
			otherAccount, err := service.CreateAccount(t.Context(), CreateAccountParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				Name:        fmt.Sprintf("other-account-%s", fake.Lorem().Word()),
				Currency:    "PLN",
				Kind:        domain.AccountKindManual,
			})
			require.NoError(t, err)

			recordTransfer := func(
				accountID string,
				amountMinor int64,
				currency string,
				effectiveAt time.Time,
				transferGroupID string,
			) domain.Transaction {
				t.Helper()

				transaction, recordErr := service.RecordTransaction(
					t.Context(),
					RecordTransactionParams{
						ActorUserID:     ownerUserID,
						TenantID:        tenant.ID,
						AccountID:       accountID,
						Source:          domain.TransactionSourceCSV,
						Status:          domain.TransactionStatusBooked,
						Kind:            domain.TransactionKindTransfer,
						AmountMinor:     amountMinor,
						Currency:        currency,
						Description:     fmt.Sprintf("transfer-%s", fake.Lorem().Word()),
						EffectiveAt:     effectiveAt,
						TransferGroupID: transferGroupID,
					},
				)
				require.NoError(t, recordErr)
				return transaction
			}

			outgoingTransfer := recordTransfer(account.ID, -12_00, "USD", now.Add(-2*time.Hour), "")
			incomingTransfer := recordTransfer(otherAccount.ID, 9_00, "PLN", now.Add(-time.Hour), "")

			summaryBeforeLink, err := service.SummarizeTransactions(
				t.Context(),
				SummarizeTransactionsParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, int64(9_00), summaryBeforeLink.IncomeMinor)
			assert.Equal(t, int64(12_00), summaryBeforeLink.ExpenseMinor)
			outgoingBalanceBeforeLink, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				AccountID:   account.ID,
			})
			require.NoError(t, err)
			incomingBalanceBeforeLink, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				AccountID:   otherAccount.ID,
			})
			require.NoError(t, err)

			err = service.LinkTransfers(t.Context(), LinkTransfersParams{
				ActorUserID:         outsiderUserID,
				TenantID:            tenant.ID,
				FirstTransactionID:  outgoingTransfer.ID,
				SecondTransactionID: incomingTransfer.ID,
			})
			require.ErrorIs(t, err, ErrTenantAccessDenied)

			require.NoError(t, service.LinkTransfers(t.Context(), LinkTransfersParams{
				ActorUserID:         ownerUserID,
				TenantID:            tenant.ID,
				FirstTransactionID:  outgoingTransfer.ID,
				SecondTransactionID: incomingTransfer.ID,
			}))

			linkedOutgoing, err := service.store.GetTransaction(t.Context(), outgoingTransfer.ID)
			require.NoError(t, err)
			linkedIncoming, err := service.store.GetTransaction(t.Context(), incomingTransfer.ID)
			require.NoError(t, err)
			require.NotNil(t, linkedOutgoing.TransferGroupID)
			require.NotNil(t, linkedIncoming.TransferGroupID)
			require.NotNil(t, linkedOutgoing.TransferMatchedAt)
			require.NotNil(t, linkedIncoming.TransferMatchedAt)
			assert.NotEmpty(t, *linkedOutgoing.TransferGroupID)
			assert.Equal(t, *linkedOutgoing.TransferGroupID, *linkedIncoming.TransferGroupID)
			assert.Equal(t, now, *linkedOutgoing.TransferMatchedAt)
			assert.Equal(t, now, *linkedIncoming.TransferMatchedAt)
			assert.Equal(t, domain.TransactionKindTransfer, linkedOutgoing.Kind)
			assert.Equal(t, domain.TransactionKindTransfer, linkedIncoming.Kind)
			outgoingBalanceAfterLink, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				AccountID:   account.ID,
			})
			require.NoError(t, err)
			incomingBalanceAfterLink, err := service.GetAccountBalance(t.Context(), GetAccountBalanceParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
				AccountID:   otherAccount.ID,
			})
			require.NoError(t, err)
			assert.Equal(t, outgoingBalanceBeforeLink, outgoingBalanceAfterLink)
			assert.Equal(t, incomingBalanceBeforeLink, incomingBalanceAfterLink)

			groupedUnmatchedTransfer := recordTransfer(
				account.ID,
				-7_00,
				"USD",
				now.Add(-30*time.Minute),
				"lonely-transfer-group",
			)
			require.NotNil(t, groupedUnmatchedTransfer.TransferGroupID)
			assert.Nil(t, groupedUnmatchedTransfer.TransferMatchedAt)

			summaryAfterLink, err := service.SummarizeTransactions(
				t.Context(),
				SummarizeTransactionsParams{
					ActorUserID: ownerUserID,
					TenantID:    tenant.ID,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, int64(0), summaryAfterLink.IncomeMinor)
			assert.Equal(t, int64(7_00), summaryAfterLink.ExpenseMinor)
			assert.Equal(t, int64(-7_00), summaryAfterLink.NetMinor)

			require.NoError(t, service.UnlinkTransfers(t.Context(), UnlinkTransfersParams{
				ActorUserID:         ownerUserID,
				TenantID:            tenant.ID,
				FirstTransactionID:  outgoingTransfer.ID,
				SecondTransactionID: incomingTransfer.ID,
			}))
			unlinkedOutgoing, err := service.GetTransaction(t.Context(), GetTransactionParams{
				ActorUserID:   ownerUserID,
				TenantID:      tenant.ID,
				TransactionID: outgoingTransfer.ID,
			})
			require.NoError(t, err)
			unlinkedIncoming, err := service.GetTransaction(t.Context(), GetTransactionParams{
				ActorUserID:   ownerUserID,
				TenantID:      tenant.ID,
				TransactionID: incomingTransfer.ID,
			})
			require.NoError(t, err)
			assert.Equal(t, domain.TransactionKindRegular, unlinkedOutgoing.Kind)
			assert.Equal(t, domain.TransactionKindRegular, unlinkedIncoming.Kind)
			assert.Nil(t, unlinkedOutgoing.TransferGroupID)
			assert.Nil(t, unlinkedIncoming.TransferGroupID)
			assert.Nil(t, unlinkedOutgoing.TransferMatchedAt)
			assert.Nil(t, unlinkedIncoming.TransferMatchedAt)
			summaryAfterUnlink, err := service.SummarizeTransactions(t.Context(), SummarizeTransactionsParams{
				ActorUserID: ownerUserID,
				TenantID:    tenant.ID,
			})
			require.NoError(t, err)
			assert.Equal(t, int64(9_00), summaryAfterUnlink.IncomeMinor)
			assert.Equal(t, int64(19_00), summaryAfterUnlink.ExpenseMinor)
		},
	)

	t.Run(
		"does not leave one-sided transfer links when atomic persistence fails",
		func(t *testing.T) {
			now := time.Date(2026, time.June, 20, 17, 30, 0, 0, time.UTC)
			sentinel := errors.New("atomic link failed")
			firstTransfer := domain.Transaction{
				ID: "transaction-1", TenantID: "tenant-1", AccountID: "account-1",
				Status: domain.TransactionStatusBooked, AmountMinor: -1,
			}
			secondTransfer := domain.Transaction{
				ID: "transaction-2", TenantID: "tenant-1", AccountID: "account-2",
				Status: domain.TransactionStatusBooked, AmountMinor: 1,
			}
			var saveTransactionCalled bool
			var savedPairs [][2]domain.Transaction

			err := NewService(stubStore{
				isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
				getTransactionFn: func(_ context.Context, transactionID string) (*domain.Transaction, error) {
					if transactionID == firstTransfer.ID {
						return &firstTransfer, nil
					}
					return &secondTransfer, nil
				},
				saveTransactionFn: func(context.Context, domain.Transaction) (domain.Transaction, error) {
					saveTransactionCalled = true
					return domain.Transaction{}, nil
				},
				saveLinkedTransferPairFn: func(_ context.Context, first domain.Transaction, second domain.Transaction) error {
					savedPairs = append(savedPairs, [2]domain.Transaction{first, second})
					return sentinel
				},
			}, WithNow(func() time.Time { return now })).LinkTransfers(t.Context(), LinkTransfersParams{
				ActorUserID:         "user-1",
				TenantID:            "tenant-1",
				FirstTransactionID:  firstTransfer.ID,
				SecondTransactionID: secondTransfer.ID,
			})

			require.ErrorIs(t, err, sentinel)
			assert.False(t, saveTransactionCalled)
			require.Len(t, savedPairs, 1)
			require.NotNil(t, savedPairs[0][0].TransferGroupID)
			require.NotNil(t, savedPairs[0][1].TransferGroupID)
			require.NotNil(t, savedPairs[0][0].TransferMatchedAt)
			require.NotNil(t, savedPairs[0][1].TransferMatchedAt)
			assert.Equal(t, *savedPairs[0][0].TransferGroupID, *savedPairs[0][1].TransferGroupID)
			assert.Equal(t, now, *savedPairs[0][0].TransferMatchedAt)
			assert.Equal(t, now, *savedPairs[0][1].TransferMatchedAt)
		},
	)

	t.Run("rejects invalid transfer pairs without writing either transaction", func(t *testing.T) {
		fake := faker.New()
		base := func(id, accountID string, amountMinor int64) domain.Transaction {
			return domain.Transaction{
				ID:          id,
				TenantID:    "tenant-" + fake.UUID().V4(),
				AccountID:   accountID,
				Status:      domain.TransactionStatusBooked,
				AmountMinor: amountMinor,
			}
		}
		for _, tc := range []struct {
			name   string
			first  domain.Transaction
			second domain.Transaction
		}{
			{
				name:   "same transaction",
				first:  base("transaction-"+fake.UUID().V4(), "account-"+fake.UUID().V4(), -1),
				second: base("transaction-"+fake.UUID().V4(), "account-"+fake.UUID().V4(), 1),
			},
			{
				name:   "same account",
				first:  base("transaction-"+fake.UUID().V4(), "account-"+fake.UUID().V4(), -1),
				second: base("transaction-"+fake.UUID().V4(), "account-"+fake.UUID().V4(), 1),
			},
			{
				name:   "same direction",
				first:  base("transaction-"+fake.UUID().V4(), "account-first-"+fake.UUID().V4(), -1),
				second: base("transaction-"+fake.UUID().V4(), "account-second-"+fake.UUID().V4(), -2),
			},
			{
				name:   "zero amount",
				first:  base("transaction-"+fake.UUID().V4(), "account-first-"+fake.UUID().V4(), 0),
				second: base("transaction-"+fake.UUID().V4(), "account-second-"+fake.UUID().V4(), 1),
			},
			{
				name:   "not booked",
				first:  base("transaction-"+fake.UUID().V4(), "account-first-"+fake.UUID().V4(), -1),
				second: base("transaction-"+fake.UUID().V4(), "account-second-"+fake.UUID().V4(), 1),
			},
			{
				name:   "already linked",
				first:  base("transaction-"+fake.UUID().V4(), "account-first-"+fake.UUID().V4(), -1),
				second: base("transaction-"+fake.UUID().V4(), "account-second-"+fake.UUID().V4(), 1),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				first := tc.first
				second := tc.second
				first.TenantID = "tenant-" + fake.UUID().V4()
				second.TenantID = first.TenantID
				switch tc.name {
				case "same transaction":
					second.ID = first.ID
				case "same account":
					second.AccountID = first.AccountID
				case "not booked":
					second.Status = domain.TransactionStatusPending
				case "already linked":
					groupID := "group-" + fake.UUID().V4()
					first.TransferGroupID = &groupID
				}
				service := NewLedgerService(stubStore{
					isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
					getTransactionFn: func(_ context.Context, transactionID string) (*domain.Transaction, error) {
						if transactionID == first.ID {
							return &first, nil
						}
						return &second, nil
					},
					saveLinkedTransferPairFn: func(context.Context, domain.Transaction, domain.Transaction) error {
						t.Fatal("invalid pair must not be persisted")
						return nil
					},
				})

				err := service.LinkTransfers(t.Context(), LinkTransfersParams{
					ActorUserID:         "user-" + fake.UUID().V4(),
					TenantID:            first.TenantID,
					FirstTransactionID:  first.ID,
					SecondTransactionID: second.ID,
				})
				require.ErrorIs(t, err, ErrInvalidTransferPair)
			})
		}
	})

	t.Run("rejects unlinking transactions that are not one linked transfer pair", func(t *testing.T) {
		fake := faker.New()
		first := domain.Transaction{
			ID:          "transaction-first-" + fake.UUID().V4(),
			TenantID:    "tenant-" + fake.UUID().V4(),
			AccountID:   "account-first-" + fake.UUID().V4(),
			Status:      domain.TransactionStatusBooked,
			AmountMinor: -1,
		}
		second := domain.Transaction{
			ID:          "transaction-second-" + fake.UUID().V4(),
			TenantID:    first.TenantID,
			AccountID:   "account-second-" + fake.UUID().V4(),
			Status:      domain.TransactionStatusBooked,
			AmountMinor: 1,
		}
		service := NewLedgerService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTransactionFn: func(_ context.Context, transactionID string) (*domain.Transaction, error) {
				if transactionID == first.ID {
					return &first, nil
				}
				return &second, nil
			},
		})

		err := service.UnlinkTransfers(t.Context(), UnlinkTransfersParams{
			ActorUserID:         "user-" + fake.UUID().V4(),
			TenantID:            first.TenantID,
			FirstTransactionID:  first.ID,
			SecondTransactionID: second.ID,
		})
		require.ErrorIs(t, err, ErrTransferNotLinked)
	})

	t.Run("rejects cross-tenant entity access for tenant-scoped resources", func(t *testing.T) {
		service := NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn: func(context.Context, string) (*domain.Account, error) {
				return &domain.Account{ID: "account-1", TenantID: "tenant-other"}, nil
			},
			getCategoryFn: func(context.Context, string) (*domain.Category, error) {
				return &domain.Category{ID: "category-1", TenantID: "tenant-other"}, nil
			},
			getTagFn: func(context.Context, string) (*domain.Tag, error) {
				return &domain.Tag{ID: "tag-1", TenantID: "tenant-other"}, nil
			},
			getTransactionFn: func(context.Context, string) (*domain.Transaction, error) {
				return &domain.Transaction{ID: "transaction-1", TenantID: "tenant-other"}, nil
			},
		})

		_, err := service.UpdateAccount(t.Context(), UpdateAccountParams{
			ActorUserID: "user-1",
			TenantID:    "tenant-1",
			AccountID:   "account-1",
			Name:        "renamed",
		})
		require.ErrorIs(t, err, ErrAccountNotFound)

		err = service.HideCategory(t.Context(), HideCategoryParams{
			ActorUserID: "user-1",
			TenantID:    "tenant-1",
			CategoryID:  "category-1",
		})
		require.ErrorIs(t, err, ErrCategoryNotFound)

		_, err = service.UpdateTag(t.Context(), UpdateTagParams{
			ActorUserID: "user-1",
			TenantID:    "tenant-1",
			TagID:       "tag-1",
			Name:        "renamed",
		})
		require.ErrorIs(t, err, ErrTagNotFound)

		err = service.HideTransaction(t.Context(), HideTransactionParams{
			ActorUserID:   "user-1",
			TenantID:      "tenant-1",
			TransactionID: "transaction-1",
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})

	t.Run("surfaces service options and error paths", func(t *testing.T) {
		fake := faker.New()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)

		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		ids := []string{
			"tenant-id",
			"category-1",
			"category-2",
			"category-3",
			"category-4",
			"invite-id",
			"invite-code",
		}
		service := NewService(
			store,
			WithNow(func() time.Time { return now }),
			WithIDGenerator(func() string {
				if len(ids) == 0 {
					return fake.UUID().V4()
				}
				value := ids[0]
				ids = ids[1:]
				return value
			}),
		)

		_, err := service.CreateTenant(t.Context(), CreateTenantParams{SeedDefaults: false})
		require.Error(t, err)

		ownerUserID := fmt.Sprintf("user-owner-%s", fake.Lorem().Word())
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
			DisplayCurrency: "eur",
			SeedDefaults:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, "tenant-id", tenant.ID)
		assert.Equal(t, now, tenant.CreatedAt)
		assert.Equal(t, "EUR", tenant.DisplayCurrency)

		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: fmt.Sprintf("user-missing-%s", fake.Lorem().Word()),
			Code:        fmt.Sprintf("missing-code-%s", fake.Lorem().Word()),
		})
		require.ErrorIs(t, err, ErrInviteNotFound)

		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Recipient:   fmt.Sprintf("recipient-%s@example.com", fake.Internet().User()),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, invite.ID)
		assert.NotEmpty(t, invite.Code)

		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: fmt.Sprintf("user-accepted-%s", fake.Lorem().Word()),
			Code:        invite.Code,
		})
		require.NoError(t, err)

		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: fmt.Sprintf("user-second-%s", fake.Lorem().Word()),
			Code:        invite.Code,
		})
		require.ErrorIs(t, err, ErrInviteAccepted)

		_, err = service.UpdateAccount(t.Context(), UpdateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   fmt.Sprintf("missing-account-%s", fake.Lorem().Word()),
			Name:        fmt.Sprintf("name-%s", fake.Lorem().Word()),
		})
		require.ErrorIs(t, err, ErrAccountNotFound)

		_, err = service.UpdateCategory(t.Context(), UpdateCategoryParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			CategoryID:  fmt.Sprintf("missing-category-%s", fake.Lorem().Word()),
			Name:        fmt.Sprintf("name-%s", fake.Lorem().Word()),
			Kind:        domain.CategoryKindExpense,
		})
		require.ErrorIs(t, err, ErrCategoryNotFound)

		_, err = service.UpdateTag(t.Context(), UpdateTagParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			TagID:       fmt.Sprintf("missing-tag-%s", fake.Lorem().Word()),
			Name:        fmt.Sprintf("name-%s", fake.Lorem().Word()),
		})
		require.ErrorIs(t, err, ErrTagNotFound)

		_, err = service.UpdateTransaction(t.Context(), UpdateTransactionParams{
			ActorUserID:   ownerUserID,
			TenantID:      tenant.ID,
			TransactionID: fmt.Sprintf("missing-transaction-%s", fake.Lorem().Word()),
			Description:   fmt.Sprintf("name-%s", fake.Lorem().Word()),
			AmountMinor:   1,
			EffectiveAt:   timePointer(now),
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)

		_, err = service.GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID:   ownerUserID,
			TenantID:      tenant.ID,
			TransactionID: fmt.Sprintf("missing-transaction-%s", fake.Lorem().Word()),
		})
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})

	t.Run("wraps store failures across service methods", func(t *testing.T) {
		sentinel := errors.New("boom")
		effectiveAt := time.Date(2026, time.June, 20, 16, 0, 0, 0, time.UTC)
		baseAccount := domain.Account{ID: "account-1", TenantID: "tenant-1", Currency: "USD"}
		baseCategory := domain.Category{ID: "category-1", TenantID: "tenant-1"}
		baseTag := domain.Tag{ID: "tag-1", TenantID: "tenant-1"}
		baseTransaction := domain.Transaction{ID: "transaction-1", TenantID: "tenant-1"}

		_, err := NewService(stubStore{
			saveTenantFn: func(context.Context, domain.Tenant) (domain.Tenant, error) { return domain.Tenant{}, sentinel },
		}).CreateTenant(t.Context(), CreateTenantParams{ActorUserID: "user-1", Name: "tenant", DisplayCurrency: "USD", SeedDefaults: true})
		require.ErrorIs(t, err, sentinel)

		categoryID := "category-1"
		_, err = NewService(stubStore{
			saveTenantFn: func(context.Context, domain.Tenant) (domain.Tenant, error) { return domain.Tenant{}, nil },
			saveTenantMembershipFn: func(context.Context, domain.TenantMembership) (domain.TenantMembership, error) {
				return domain.TenantMembership{}, sentinel
			},
		}).CreateTenant(t.Context(), CreateTenantParams{ActorUserID: "user-1", Name: "tenant", DisplayCurrency: "USD", SeedDefaults: true})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			saveTenantFn: func(context.Context, domain.Tenant) (domain.Tenant, error) { return domain.Tenant{}, nil },
			saveTenantMembershipFn: func(context.Context, domain.TenantMembership) (domain.TenantMembership, error) {
				return domain.TenantMembership{}, nil
			},
			saveCategoryFn: func(context.Context, domain.Category) (domain.Category, error) {
				return domain.Category{}, sentinel
			},
		}).CreateTenant(t.Context(), CreateTenantParams{ActorUserID: "user-1", Name: "tenant", DisplayCurrency: "USD", SeedDefaults: true})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			listTenantsForUserFn: func(context.Context, string) ([]domain.TenantMembershipView, error) { return nil, sentinel },
		}).ListTenantsForUser(t.Context(), "user-1")
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			saveTenantInviteFn: func(context.Context, domain.TenantInvite) (domain.TenantInvite, error) {
				return domain.TenantInvite{}, sentinel
			},
		}).CreateTenantInvite(t.Context(), CreateTenantInviteParams{ActorUserID: "user-1", TenantID: "tenant-1", Recipient: "user@example.com"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			getTenantInviteByCodeFn: func(context.Context, string) (*domain.TenantInvite, error) { return nil, sentinel },
		}).AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{ActorUserID: "user-1", Code: "code-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			getTenantInviteByCodeFn: func(context.Context, string) (*domain.TenantInvite, error) {
				return &domain.TenantInvite{TenantID: "tenant-1"}, nil
			},
			saveTenantMembershipFn: func(context.Context, domain.TenantMembership) (domain.TenantMembership, error) {
				return domain.TenantMembership{}, sentinel
			},
		}).AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{ActorUserID: "user-1", Code: "code-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			getTenantInviteByCodeFn: func(context.Context, string) (*domain.TenantInvite, error) {
				return &domain.TenantInvite{TenantID: "tenant-1"}, nil
			},
			saveTenantMembershipFn: func(context.Context, domain.TenantMembership) (domain.TenantMembership, error) {
				return domain.TenantMembership{}, nil
			},
			updateTenantInviteFn: func(context.Context, domain.TenantInvite) (domain.TenantInvite, error) {
				return domain.TenantInvite{}, sentinel
			},
		}).AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{ActorUserID: "user-1", Code: "code-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTenantFn:      func(context.Context, string) (*domain.Tenant, error) { return nil, sentinel },
		}).UpdateTenant(t.Context(), UpdateTenantParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "tenant", DisplayCurrency: "USD"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTenantFn: func(context.Context, string) (*domain.Tenant, error) {
				return &domain.Tenant{ID: "tenant-1", Name: "tenant", DisplayCurrency: "USD"}, nil
			},
			saveTenantFn: func(context.Context, domain.Tenant) (domain.Tenant, error) { return domain.Tenant{}, sentinel },
		}).UpdateTenant(t.Context(), UpdateTenantParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "tenant-updated", DisplayCurrency: "EUR"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn:    func(context.Context, string, string) (bool, error) { return true, nil },
			listTenantMembersFn: func(context.Context, string) ([]domain.TenantMember, error) { return nil, sentinel },
		}).ListTenantMembers(t.Context(), ListTenantMembersParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			saveAccountFn:    func(context.Context, domain.Account) (domain.Account, error) { return domain.Account{}, sentinel },
		}).CreateAccount(t.Context(), CreateAccountParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "cash", Currency: "USD", Kind: domain.AccountKindManual})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return false, nil },
		}).CreateAccount(t.Context(), CreateAccountParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "cash", Currency: "USD", Kind: domain.AccountKindManual})
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn:     func(context.Context, string) (*domain.Account, error) { return &baseAccount, nil },
			saveAccountFn:    func(context.Context, domain.Account) (domain.Account, error) { return domain.Account{}, sentinel },
		}).UpdateAccount(t.Context(), UpdateAccountParams{ActorUserID: "user-1", TenantID: "tenant-1", AccountID: "account-1", Name: "cash-updated"})
		require.ErrorIs(t, err, sentinel)

		err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn:     func(context.Context, string) (*domain.Account, error) { return &baseAccount, nil },
			saveAccountFn:    func(context.Context, domain.Account) (domain.Account, error) { return domain.Account{}, sentinel },
		}).HideAccount(t.Context(), HideAccountParams{ActorUserID: "user-1", TenantID: "tenant-1", AccountID: "account-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return false, sentinel },
		}).ListAccounts(t.Context(), ListAccountsParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			listAccountsFn:   func(context.Context, string, bool) ([]domain.Account, error) { return nil, sentinel },
		}).ListAccounts(t.Context(), ListAccountsParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			saveCategoryFn:   func(context.Context, domain.Category) (domain.Category, error) { return domain.Category{}, sentinel },
		}).CreateCategory(t.Context(), CreateCategoryParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "food", Kind: domain.CategoryKindExpense})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getCategoryFn:    func(context.Context, string) (*domain.Category, error) { return &baseCategory, nil },
			saveCategoryFn:   func(context.Context, domain.Category) (domain.Category, error) { return domain.Category{}, sentinel },
		}).UpdateCategory(t.Context(), UpdateCategoryParams{ActorUserID: "user-1", TenantID: "tenant-1", CategoryID: "category-1", Name: "food", Kind: domain.CategoryKindExpense})
		require.ErrorIs(t, err, sentinel)

		err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getCategoryFn:    func(context.Context, string) (*domain.Category, error) { return &baseCategory, nil },
			saveCategoryFn:   func(context.Context, domain.Category) (domain.Category, error) { return domain.Category{}, sentinel },
		}).HideCategory(t.Context(), HideCategoryParams{ActorUserID: "user-1", TenantID: "tenant-1", CategoryID: "category-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			listCategoriesFn: func(context.Context, string, bool) ([]domain.Category, error) { return nil, sentinel },
		}).ListCategories(t.Context(), ListCategoriesParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			saveTagFn:        func(context.Context, domain.Tag) (domain.Tag, error) { return domain.Tag{}, sentinel },
		}).CreateTag(t.Context(), CreateTagParams{ActorUserID: "user-1", TenantID: "tenant-1", Name: "trip"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTagFn:         func(context.Context, string) (*domain.Tag, error) { return &baseTag, nil },
			saveTagFn:        func(context.Context, domain.Tag) (domain.Tag, error) { return domain.Tag{}, sentinel },
		}).UpdateTag(t.Context(), UpdateTagParams{ActorUserID: "user-1", TenantID: "tenant-1", TagID: "tag-1", Name: "trip"})
		require.ErrorIs(t, err, sentinel)

		err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTagFn:         func(context.Context, string) (*domain.Tag, error) { return &baseTag, nil },
			saveTagFn:        func(context.Context, domain.Tag) (domain.Tag, error) { return domain.Tag{}, sentinel },
		}).HideTag(t.Context(), HideTagParams{ActorUserID: "user-1", TenantID: "tenant-1", TagID: "tag-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			listTagsFn:       func(context.Context, string, bool) ([]domain.Tag, error) { return nil, sentinel },
		}).ListTags(t.Context(), ListTagsParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn:     func(context.Context, string) (*domain.Account, error) { return &baseAccount, nil },
			saveTransactionFn: func(context.Context, domain.Transaction) (domain.Transaction, error) {
				return domain.Transaction{}, sentinel
			},
		}).RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: "user-1",
			TenantID:    "tenant-1",
			AccountID:   "account-1",
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -1,
			Currency:    "USD",
			Description: "txn",
			EffectiveAt: effectiveAt,
		})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTransactionFn: func(context.Context, string) (*domain.Transaction, error) { return &baseTransaction, nil },
			saveTransactionFn: func(context.Context, domain.Transaction) (domain.Transaction, error) {
				return domain.Transaction{}, sentinel
			},
			getCategoryFn: func(context.Context, string) (*domain.Category, error) { return &baseCategory, nil },
		}).UpdateTransaction(t.Context(), UpdateTransactionParams{
			ActorUserID:   "user-1",
			TenantID:      "tenant-1",
			TransactionID: "transaction-1",
			Description:   "txn",
			AmountMinor:   -1,
			EffectiveAt:   timePointer(effectiveAt),
			CategoryID:    categoryID,
		})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTransactionFn: func(context.Context, string) (*domain.Transaction, error) { return nil, sentinel },
		}).GetTransaction(t.Context(), GetTransactionParams{
			ActorUserID:   "user-1",
			TenantID:      "tenant-1",
			TransactionID: "transaction-1",
		})
		require.ErrorIs(t, err, sentinel)

		err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTransactionFn: func(context.Context, string) (*domain.Transaction, error) { return &baseTransaction, nil },
			saveTransactionFn: func(context.Context, domain.Transaction) (domain.Transaction, error) {
				return domain.Transaction{}, sentinel
			},
		}).HideTransaction(t.Context(), HideTransactionParams{ActorUserID: "user-1", TenantID: "tenant-1", TransactionID: "transaction-1"})
		require.ErrorIs(t, err, sentinel)

		err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getTransactionFn: func(_ context.Context, transactionID string) (*domain.Transaction, error) {
				if transactionID == "transaction-2" {
					return &domain.Transaction{
						ID: "transaction-2", TenantID: "tenant-1", AccountID: "account-2",
						Status: domain.TransactionStatusBooked, AmountMinor: 1,
					}, nil
				}
				return &domain.Transaction{
					ID: "transaction-1", TenantID: "tenant-1", AccountID: "account-1",
					Status: domain.TransactionStatusBooked, AmountMinor: -1,
				}, nil
			},
			saveLinkedTransferPairFn: func(context.Context, domain.Transaction, domain.Transaction) error {
				return sentinel
			},
		}).LinkTransfers(t.Context(), LinkTransfersParams{
			ActorUserID:         "user-1",
			TenantID:            "tenant-1",
			FirstTransactionID:  "transaction-1",
			SecondTransactionID: "transaction-2",
		})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			listTransactionsFn: func(context.Context, string, string, domain.TransactionSource, domain.TransactionStatus, bool) ([]domain.Transaction, error) {
				return nil, sentinel
			},
		}).ListTransactions(t.Context(), ListTransactionsParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn:     func(context.Context, string) (*domain.Account, error) { return &baseAccount, nil },
			listTransactionsFn: func(context.Context, string, string, domain.TransactionSource, domain.TransactionStatus, bool) ([]domain.Transaction, error) {
				return nil, sentinel
			},
		}).GetAccountBalance(t.Context(), GetAccountBalanceParams{ActorUserID: "user-1", TenantID: "tenant-1", AccountID: "account-1"})
		require.ErrorIs(t, err, sentinel)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getAccountFn: func(context.Context, string) (*domain.Account, error) {
				return nil, persistence.ErrAccountNotFound
			},
		}).GetAccountBalance(t.Context(), GetAccountBalanceParams{ActorUserID: "user-1", TenantID: "tenant-1", AccountID: "account-1"})
		require.ErrorIs(t, err, ErrAccountNotFound)

		_, err = NewService(stubStore{
			isTenantMemberFn: func(context.Context, string, string) (bool, error) { return true, nil },
			listTransactionsFn: func(context.Context, string, string, domain.TransactionSource, domain.TransactionStatus, bool) ([]domain.Transaction, error) {
				return nil, sentinel
			},
		}).SummarizeTransactions(t.Context(), SummarizeTransactionsParams{ActorUserID: "user-1", TenantID: "tenant-1"})
		require.ErrorIs(t, err, sentinel)
	})
}

type stubStore struct {
	saveTenantFn             func(context.Context, domain.Tenant) (domain.Tenant, error)
	saveTenantMembershipFn   func(context.Context, domain.TenantMembership) (domain.TenantMembership, error)
	listTenantsForUserFn     func(context.Context, string) ([]domain.TenantMembershipView, error)
	isTenantMemberFn         func(context.Context, string, string) (bool, error)
	saveTenantInviteFn       func(context.Context, domain.TenantInvite) (domain.TenantInvite, error)
	getTenantInviteByCodeFn  func(context.Context, string) (*domain.TenantInvite, error)
	updateTenantInviteFn     func(context.Context, domain.TenantInvite) (domain.TenantInvite, error)
	listTenantMembersFn      func(context.Context, string) ([]domain.TenantMember, error)
	listTenantInvitesFn      func(context.Context, string) ([]domain.TenantInvite, error)
	saveAccountFn            func(context.Context, domain.Account) (domain.Account, error)
	getAccountFn             func(context.Context, string) (*domain.Account, error)
	listAccountsFn           func(context.Context, string, bool) ([]domain.Account, error)
	saveCategoryFn           func(context.Context, domain.Category) (domain.Category, error)
	getCategoryFn            func(context.Context, string) (*domain.Category, error)
	listCategoriesFn         func(context.Context, string, bool) ([]domain.Category, error)
	saveTagFn                func(context.Context, domain.Tag) (domain.Tag, error)
	getTagFn                 func(context.Context, string) (*domain.Tag, error)
	listTagsFn               func(context.Context, string, bool) ([]domain.Tag, error)
	saveTransactionFn        func(context.Context, domain.Transaction) (domain.Transaction, error)
	saveLinkedTransferPairFn func(context.Context, domain.Transaction, domain.Transaction) error
	getTransactionFn         func(context.Context, string) (*domain.Transaction, error)
	listTransactionsFn       func(context.Context, string, string, domain.TransactionSource, domain.TransactionStatus, bool) ([]domain.Transaction, error)
	getTenantFn              func(context.Context, string) (*domain.Tenant, error)
	saveCurrentFXRatesFn     func(context.Context, []domain.FXRate) error
	listCurrentFXRatesFn     func(context.Context, persistence.ListCurrentFXRatesParams) ([]domain.FXRate, error)
	saveCSVImportFn          func(context.Context, domain.CSVImportRecord) (domain.CSVImportRecord, error)
	getCSVImportFn           func(context.Context, string) (*domain.CSVImportRecord, error)
}

var errStubStoreNotConfigured = errors.New("stub store function not configured")

func (s stubStore) SaveTenant(ctx context.Context, tenant domain.Tenant) (domain.Tenant, error) {
	if s.saveTenantFn == nil {
		return tenant, nil
	}
	return s.saveTenantFn(ctx, tenant)
}

func (s stubStore) SaveTenantMembership(
	ctx context.Context,
	membership domain.TenantMembership,
) (domain.TenantMembership, error) {
	if s.saveTenantMembershipFn == nil {
		return membership, nil
	}
	return s.saveTenantMembershipFn(ctx, membership)
}

func (s stubStore) ListTenantsForUser(
	ctx context.Context,
	userID string,
) ([]domain.TenantMembershipView, error) {
	if s.listTenantsForUserFn == nil {
		return nil, nil
	}
	return s.listTenantsForUserFn(ctx, userID)
}

func (s stubStore) IsTenantMember(
	ctx context.Context,
	tenantID string,
	userID string,
) (bool, error) {
	if s.isTenantMemberFn == nil {
		return false, nil
	}
	return s.isTenantMemberFn(ctx, tenantID, userID)
}

func (s stubStore) SaveTenantInvite(
	ctx context.Context,
	invite domain.TenantInvite,
) (domain.TenantInvite, error) {
	if s.saveTenantInviteFn == nil {
		return invite, nil
	}
	return s.saveTenantInviteFn(ctx, invite)
}

func (s stubStore) GetTenantInviteByCode(
	ctx context.Context,
	code string,
) (*domain.TenantInvite, error) {
	if s.getTenantInviteByCodeFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getTenantInviteByCodeFn(ctx, code)
}

func (s stubStore) UpdateTenantInvite(
	ctx context.Context,
	invite domain.TenantInvite,
) (domain.TenantInvite, error) {
	if s.updateTenantInviteFn == nil {
		return invite, nil
	}
	return s.updateTenantInviteFn(ctx, invite)
}

func (s stubStore) ListTenantMembers(
	ctx context.Context,
	tenantID string,
) ([]domain.TenantMember, error) {
	if s.listTenantMembersFn == nil {
		return nil, nil
	}
	return s.listTenantMembersFn(ctx, tenantID)
}

func (s stubStore) ListTenantInvites(
	ctx context.Context,
	tenantID string,
) ([]domain.TenantInvite, error) {
	if s.listTenantInvitesFn == nil {
		return nil, nil
	}
	return s.listTenantInvitesFn(ctx, tenantID)
}

func (s stubStore) SaveAccount(
	ctx context.Context,
	account domain.Account,
) (domain.Account, error) {
	if s.saveAccountFn == nil {
		return account, nil
	}
	return s.saveAccountFn(ctx, account)
}

func (s stubStore) GetAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	if s.getAccountFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getAccountFn(ctx, accountID)
}

func (s stubStore) ListAccounts(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Account, error) {
	if s.listAccountsFn == nil {
		return nil, nil
	}
	return s.listAccountsFn(ctx, tenantID, includeHidden)
}

func (s stubStore) SaveCategory(
	ctx context.Context,
	category domain.Category,
) (domain.Category, error) {
	if s.saveCategoryFn == nil {
		return category, nil
	}
	return s.saveCategoryFn(ctx, category)
}

func (s stubStore) GetCategory(ctx context.Context, categoryID string) (*domain.Category, error) {
	if s.getCategoryFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getCategoryFn(ctx, categoryID)
}

func (s stubStore) ListCategories(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Category, error) {
	if s.listCategoriesFn == nil {
		return nil, nil
	}
	return s.listCategoriesFn(ctx, tenantID, includeHidden)
}

func (s stubStore) SaveTag(ctx context.Context, tag domain.Tag) (domain.Tag, error) {
	if s.saveTagFn == nil {
		return tag, nil
	}
	return s.saveTagFn(ctx, tag)
}

func (s stubStore) GetTag(ctx context.Context, tagID string) (*domain.Tag, error) {
	if s.getTagFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getTagFn(ctx, tagID)
}

func (s stubStore) ListTags(
	ctx context.Context,
	tenantID string,
	includeHidden bool,
) ([]domain.Tag, error) {
	if s.listTagsFn == nil {
		return nil, nil
	}
	return s.listTagsFn(ctx, tenantID, includeHidden)
}

func (s stubStore) SaveTransaction(
	ctx context.Context,
	transaction domain.Transaction,
) (domain.Transaction, error) {
	if s.saveTransactionFn == nil {
		return transaction, nil
	}
	return s.saveTransactionFn(ctx, transaction)
}

func (s stubStore) SaveLinkedTransferPair(
	ctx context.Context,
	firstTransaction domain.Transaction,
	secondTransaction domain.Transaction,
) error {
	if s.saveLinkedTransferPairFn == nil {
		return nil
	}
	return s.saveLinkedTransferPairFn(ctx, firstTransaction, secondTransaction)
}

func (s stubStore) GetTransaction(
	ctx context.Context,
	transactionID string,
) (*domain.Transaction, error) {
	if s.getTransactionFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getTransactionFn(ctx, transactionID)
}

func (s stubStore) ListTransactions(
	ctx context.Context,
	tenantID string,
	accountID string,
	source domain.TransactionSource,
	status domain.TransactionStatus,
	includeHidden bool,
	_ ...persistence.ListTransactionsPage,
) ([]domain.Transaction, error) {
	if s.listTransactionsFn == nil {
		return nil, nil
	}
	return s.listTransactionsFn(ctx, tenantID, accountID, source, status, includeHidden)
}

func (s stubStore) GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	if s.getTenantFn == nil {
		return &domain.Tenant{ID: tenantID}, nil
	}
	return s.getTenantFn(ctx, tenantID)
}

func (s stubStore) SaveCurrentFXRates(ctx context.Context, rates []domain.FXRate) error {
	if s.saveCurrentFXRatesFn == nil {
		return nil
	}
	return s.saveCurrentFXRatesFn(ctx, rates)
}

func (s stubStore) ListCurrentFXRates(
	ctx context.Context,
	params persistence.ListCurrentFXRatesParams,
) ([]domain.FXRate, error) {
	if s.listCurrentFXRatesFn == nil {
		return nil, nil
	}
	return s.listCurrentFXRatesFn(ctx, params)
}

func (s stubStore) SaveCSVImport(
	ctx context.Context,
	record domain.CSVImportRecord,
) (domain.CSVImportRecord, error) {
	if s.saveCSVImportFn == nil {
		return record, nil
	}
	return s.saveCSVImportFn(ctx, record)
}

func (s stubStore) GetCSVImport(ctx context.Context, importID string) (*domain.CSVImportRecord, error) {
	if s.getCSVImportFn == nil {
		return nil, errStubStoreNotConfigured
	}
	return s.getCSVImportFn(ctx, importID)
}
