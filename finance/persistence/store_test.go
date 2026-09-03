package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStore(t *testing.T) {
	makeStore := func(t *testing.T) *Store {
		t.Helper()
		database := openTestDatabase(t)
		store := NewStore(database)
		return store
	}

	t.Run("uses provided database handle", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewStore(database)
		require.NotNil(t, store)
		assert.Same(t, database.db, store.db)
	})

	t.Run("keeps domain and persistence models separate", func(t *testing.T) {
		assert.NotEqual(
			t,
			fmt.Sprintf("%T", domain.ConnectionSecret{}),
			fmt.Sprintf("%T", connectionSecretModel{}),
		)
	})

	t.Run("orders core entity canonical timestamps", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		earlier := time.Date(2025, time.December, 31, 23, 30, 0, 123000, time.UTC)
		later := time.Date(2026, time.January, 1, 0, 0, 0, 456000, time.FixedZone("zero", 0))
		require.True(t, earlier.Before(later))

		userID := "user-" + fake.UUID().V4()
		earlierTenant := domain.Tenant{
			ID: "tenant-earlier-" + fake.UUID().V4(), Name: fake.Company().Name(), DisplayCurrency: "USD",
			CreatedAt: earlier, UpdatedAt: earlier,
		}
		laterTenant := domain.Tenant{
			ID: "tenant-later-" + fake.UUID().V4(), Name: fake.Company().Name(), DisplayCurrency: "USD",
			CreatedAt: later, UpdatedAt: later,
		}
		for _, tenant := range []domain.Tenant{laterTenant, earlierTenant} {
			_, err := store.SaveTenant(t.Context(), tenant)
			require.NoError(t, err)
			_, err = store.SaveTenantMembership(t.Context(), domain.TenantMembership{
				TenantID: tenant.ID, UserID: userID, JoinedAt: tenant.CreatedAt, CreatedAt: tenant.CreatedAt,
			})
			require.NoError(t, err)
		}
		views, err := store.ListTenantsForUser(t.Context(), userID)
		require.NoError(t, err)
		require.Equal(t, []string{earlierTenant.ID, laterTenant.ID}, []string{views[0].Tenant.ID, views[1].Tenant.ID})

		tenantID := earlierTenant.ID
		earlierInvite := domain.TenantInvite{
			ID: "invite-earlier-" + fake.UUID().V4(), TenantID: tenantID, Code: "code-earlier-" + fake.UUID().V4(),
			Recipient: fake.Internet().Email(), CreatedByUserID: userID, CreatedAt: earlier,
		}
		laterInvite := domain.TenantInvite{
			ID: "invite-later-" + fake.UUID().V4(), TenantID: tenantID, Code: "code-later-" + fake.UUID().V4(),
			Recipient: fake.Internet().Email(), CreatedByUserID: userID, CreatedAt: later,
		}
		for _, invite := range []domain.TenantInvite{laterInvite, earlierInvite} {
			_, err = store.SaveTenantInvite(t.Context(), invite)
			require.NoError(t, err)
		}
		invites, err := store.ListTenantInvites(t.Context(), tenantID)
		require.NoError(t, err)
		require.Equal(t, []string{earlierInvite.ID, laterInvite.ID}, []string{invites[0].ID, invites[1].ID})

		earlierMember := domain.TenantMembership{
			TenantID: tenantID, UserID: "member-earlier-" + fake.UUID().V4(), JoinedAt: earlier, CreatedAt: earlier,
		}
		laterMember := domain.TenantMembership{
			TenantID: tenantID, UserID: "member-later-" + fake.UUID().V4(), JoinedAt: later, CreatedAt: later,
		}
		for _, membership := range []domain.TenantMembership{laterMember, earlierMember} {
			_, err = store.SaveTenantMembership(t.Context(), membership)
			require.NoError(t, err)
		}
		members, err := store.ListTenantMembers(t.Context(), tenantID)
		require.NoError(t, err)
		require.Equal(t, laterMember.UserID, members[len(members)-1].UserID)
		earlierMemberIndex := -1
		laterMemberIndex := -1
		for index, member := range members {
			if member.UserID == earlierMember.UserID {
				earlierMemberIndex = index
			}
			if member.UserID == laterMember.UserID {
				laterMemberIndex = index
			}
		}
		require.GreaterOrEqual(t, earlierMemberIndex, 0)
		require.Greater(t, laterMemberIndex, earlierMemberIndex)

		earlierAccount := domain.Account{
			ID: "account-earlier-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(), Currency: "USD",
			Kind: domain.AccountKindManual, CreatedAt: earlier, UpdatedAt: earlier,
		}
		laterAccount := domain.Account{
			ID: "account-later-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(), Currency: "USD",
			Kind: domain.AccountKindManual, CreatedAt: later, UpdatedAt: later,
		}
		for _, account := range []domain.Account{laterAccount, earlierAccount} {
			_, err = store.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}
		accounts, err := store.ListAccounts(t.Context(), tenantID, true)
		require.NoError(t, err)
		require.Equal(t, []string{earlierAccount.ID, laterAccount.ID}, []string{accounts[0].ID, accounts[1].ID})

		earlierCategory := domain.Category{
			ID: "category-earlier-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense, CreatedAt: earlier, UpdatedAt: earlier,
		}
		laterCategory := domain.Category{
			ID: "category-later-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense, CreatedAt: later, UpdatedAt: later,
		}
		for _, category := range []domain.Category{laterCategory, earlierCategory} {
			_, err = store.SaveCategory(t.Context(), category)
			require.NoError(t, err)
		}
		categories, err := store.ListCategories(t.Context(), tenantID, true)
		require.NoError(t, err)
		require.Equal(t, []string{earlierCategory.ID, laterCategory.ID}, []string{categories[0].ID, categories[1].ID})

		earlierTag := domain.Tag{
			ID: "tag-earlier-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(),
			CreatedAt: earlier, UpdatedAt: earlier,
		}
		laterTag := domain.Tag{
			ID: "tag-later-" + fake.UUID().V4(), TenantID: tenantID, Name: fake.Lorem().Word(),
			CreatedAt: later, UpdatedAt: later,
		}
		for _, tag := range []domain.Tag{laterTag, earlierTag} {
			_, err = store.SaveTag(t.Context(), tag)
			require.NoError(t, err)
		}
		tags, err := store.ListTags(t.Context(), tenantID, true)
		require.NoError(t, err)
		require.Equal(t, []string{earlierTag.ID, laterTag.ID}, []string{tags[0].ID, tags[1].ID})

		makeTransaction := func(id string, at time.Time) domain.Transaction {
			return domain.Transaction{
				ID: id, TenantID: tenantID, AccountID: earlierAccount.ID, Source: domain.TransactionSourceManual,
				Status: domain.TransactionStatusBooked, Kind: domain.TransactionKindRegular, AmountMinor: 1,
				Currency: "USD", Description: fake.Lorem().Sentence(3), EffectiveAt: at, CreatedAt: at, UpdatedAt: at,
			}
		}
		earlierTransaction := makeTransaction("transaction-earlier-"+fake.UUID().V4(), earlier)
		laterTransaction := makeTransaction("transaction-later-"+fake.UUID().V4(), later)
		for _, transaction := range []domain.Transaction{earlierTransaction, laterTransaction} {
			_, err = store.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		transactions, err := store.ListTransactions(t.Context(), tenantID, "", "", "", true)
		require.NoError(t, err)
		require.Equal(
			t,
			[]string{laterTransaction.ID, earlierTransaction.ID},
			[]string{transactions[0].ID, transactions[1].ID},
		)
		require.True(t, later.Equal(transactions[0].EffectiveAt))
	})

	t.Run("enforces invite code uniqueness", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.June, 21, 10, 0, 0, 0, time.UTC)
		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		inviteCode := "code-" + fake.UUID().V4()
		_, err = store.SaveTenantInvite(t.Context(), domain.TenantInvite{
			ID: "invite-1-" + fake.UUID().V4(), TenantID: tenant.ID, Code: inviteCode,
			Recipient: fake.Internet().Email(), CreatedByUserID: "user-1-" + fake.UUID().V4(), CreatedAt: now,
		})
		require.NoError(t, err)
		_, err = store.SaveTenantInvite(t.Context(), domain.TenantInvite{
			ID: "invite-2-" + fake.UUID().V4(), TenantID: tenant.ID, Code: inviteCode,
			Recipient: fake.Internet().Email(), CreatedByUserID: "user-2-" + fake.UUID().V4(), CreatedAt: now,
		})
		require.Error(t, err)
	})

	t.Run("persists csv import records and reports missing imports", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.June, 20, 15, 0, 0, 0, time.UTC)
		autoStampedRecord := domain.CSVImportRecord{
			ID:        "import-auto-" + fake.UUID().V4(),
			TenantID:  "tenant-auto-" + fake.UUID().V4(),
			Type:      domain.CSVImportTypeTransactions,
			Status:    domain.CSVImportStatusPreviewed,
			FileName:  "transactions-auto.csv",
			RawCSV:    "accountName,currency\nwallet,USD\n",
			Headers:   []string{"accountName", "currency"},
			Mapping:   map[string]string{"accountName": "accountName"},
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		}
		autoStampedSaved, err := store.SaveCSVImport(t.Context(), autoStampedRecord)
		require.NoError(t, err)
		assert.False(t, autoStampedSaved.CreatedAt.IsZero())
		assert.Equal(t, autoStampedSaved.CreatedAt, autoStampedSaved.UpdatedAt)

		record := domain.CSVImportRecord{
			ID:                    "import-" + fake.UUID().V4(),
			TenantID:              "tenant-" + fake.UUID().V4(),
			Type:                  domain.CSVImportTypeTransactions,
			Status:                domain.CSVImportStatusPreviewed,
			FileName:              "transactions.csv",
			RawCSV:                "accountName,currency\nwallet,USD\n",
			Headers:               []string{"accountName", "currency"},
			Mapping:               map[string]string{"accountName": "accountName"},
			DuplicateRows:         []domain.CSVImportRejectedRow{{RowNumber: 2, Reason: "duplicate"}},
			RejectedRows:          []domain.CSVImportRejectedRow{{RowNumber: 3, Reason: "invalid"}},
			ImportableCount:       1,
			WouldCreateAccounts:   []string{"wallet"},
			WouldCreateCategories: []string{"groceries"},
			WouldCreateTags:       []string{"team"},
			AccountOptions: []domain.CSVImportAccountOption{{
				Name:           "wallet",
				SourceRowCount: 2,
				Selected:       true,
			}},
			SelectedAccountNames: []string{"wallet"},
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		saved, err := store.SaveCSVImport(t.Context(), record)
		require.NoError(t, err)
		assert.Equal(t, record, saved)

		loaded, err := store.GetCSVImport(t.Context(), record.ID)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		expectedLoaded := *loaded
		expectedLoaded.CreatedAt = record.CreatedAt
		expectedLoaded.UpdatedAt = record.UpdatedAt
		assert.Equal(t, expectedLoaded, record)
		assert.True(t, record.CreatedAt.Equal(loaded.CreatedAt))
		assert.True(t, record.UpdatedAt.Equal(loaded.UpdatedAt))

		loaded, err = store.GetCSVImport(t.Context(), "missing-"+fake.UUID().V4())
		require.ErrorIs(t, err, ErrCSVImportNotFound)
		assert.Nil(t, loaded)
	})

	t.Run("persists and filters finance core entities with separate models", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.June, 20, 15, 0, 0, 0, time.UTC)

		tenant := domain.Tenant{
			ID:              fmt.Sprintf("tenant-%s", fake.Lorem().Word()),
			Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		savedTenant, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)
		assert.NotEqual(t, fmt.Sprintf("%T", domain.Tenant{}), fmt.Sprintf("%T", tenantModel{}))

		membership := domain.TenantMembership{
			TenantID:  tenant.ID,
			UserID:    fmt.Sprintf("user-%s", fake.Lorem().Word()),
			JoinedAt:  now,
			CreatedAt: now,
		}
		_, err = store.SaveTenantMembership(t.Context(), membership)
		require.NoError(t, err)

		views, err := store.ListTenantsForUser(t.Context(), membership.UserID)
		require.NoError(t, err)
		require.Len(t, views, 1)
		assert.Equal(t, savedTenant.ID, views[0].Tenant.ID)

		isMember, err := store.IsTenantMember(t.Context(), tenant.ID, membership.UserID)
		require.NoError(t, err)
		assert.True(t, isMember)

		invite := domain.TenantInvite{
			ID:              fmt.Sprintf("invite-%s", fake.Lorem().Word()),
			TenantID:        tenant.ID,
			Code:            fmt.Sprintf("code-%s", fake.Lorem().Word()),
			Recipient:       fmt.Sprintf("recipient-%s@example.com", fake.Internet().User()),
			CreatedByUserID: membership.UserID,
			CreatedAt:       now,
		}
		_, err = store.SaveTenantInvite(t.Context(), invite)
		require.NoError(t, err)

		loadedInvite, err := store.GetTenantInviteByCode(t.Context(), invite.Code)
		require.NoError(t, err)
		require.NotNil(t, loadedInvite)
		assert.Equal(t, invite.ID, loadedInvite.ID)

		acceptedByUserID := fmt.Sprintf("user-accepted-%s", fake.Lorem().Word())
		acceptedAt := now.Add(time.Minute)
		invite.AcceptedByUserID = &acceptedByUserID
		invite.AcceptedAt = &acceptedAt
		updatedInvite, err := store.UpdateTenantInvite(t.Context(), invite)
		require.NoError(t, err)
		require.NotNil(t, updatedInvite.AcceptedAt)

		members, err := store.ListTenantMembers(t.Context(), tenant.ID)
		require.NoError(t, err)
		require.Len(t, members, 1)

		account := domain.Account{
			ID:        fmt.Sprintf("account-%s", fake.Lorem().Word()),
			TenantID:  tenant.ID,
			Name:      fmt.Sprintf("account-%s", fake.Lorem().Word()),
			Currency:  "USD",
			Kind:      domain.AccountKindLinked,
			CreatedAt: now,
			UpdatedAt: now,
			LinkedAccount: &domain.LinkedAccount{
				Provider:          fmt.Sprintf("provider-%s", fake.Lorem().Word()),
				ProviderAccountID: fmt.Sprintf("provider-account-%s", fake.Lorem().Word()),
			},
		}
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)

		loadedAccount, err := store.GetAccount(t.Context(), account.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedAccount)
		require.NotNil(t, loadedAccount.LinkedAccount)

		hiddenAt := now.Add(2 * time.Minute)
		account.HiddenAt = &hiddenAt
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)

		visibleAccounts, err := store.ListAccounts(t.Context(), tenant.ID, false)
		require.NoError(t, err)
		assert.Empty(t, visibleAccounts)

		allAccounts, err := store.ListAccounts(t.Context(), tenant.ID, true)
		require.NoError(t, err)
		require.Len(t, allAccounts, 1)

		category := domain.Category{
			ID:            fmt.Sprintf("category-%s", fake.Lorem().Word()),
			TenantID:      tenant.ID,
			Name:          fmt.Sprintf("category-%s", fake.Lorem().Word()),
			Kind:          domain.CategoryKindExpense,
			SeededDefault: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_, err = store.SaveCategory(t.Context(), category)
		require.NoError(t, err)

		loadedCategory, err := store.GetCategory(t.Context(), category.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedCategory)

		category.HiddenAt = &hiddenAt
		_, err = store.SaveCategory(t.Context(), category)
		require.NoError(t, err)

		visibleCategories, err := store.ListCategories(t.Context(), tenant.ID, false)
		require.NoError(t, err)
		assert.Empty(t, visibleCategories)

		tag := domain.Tag{
			ID:        fmt.Sprintf("tag-%s", fake.Lorem().Word()),
			TenantID:  tenant.ID,
			Name:      fmt.Sprintf("tag-%s", fake.Lorem().Word()),
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err = store.SaveTag(t.Context(), tag)
		require.NoError(t, err)

		loadedTag, err := store.GetTag(t.Context(), tag.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedTag)

		tag.HiddenAt = &hiddenAt
		_, err = store.SaveTag(t.Context(), tag)
		require.NoError(t, err)

		visibleTags, err := store.ListTags(t.Context(), tenant.ID, false)
		require.NoError(t, err)
		assert.Empty(t, visibleTags)

		originalEffectiveAt := now.Add(-time.Hour)
		transactionOne := domain.Transaction{
			ID:          fmt.Sprintf("transaction-%s", fake.Lorem().Word()),
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -10_00,
			Currency:    "USD",
			Description: fmt.Sprintf("transaction-%s", fake.Lorem().Word()),
			EffectiveAt: now,
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -11_00,
				Currency:    "USD",
				Description: fmt.Sprintf("provider-original-%s", fake.Lorem().Word()),
				EffectiveAt: &originalEffectiveAt,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		transactionTwo := domain.Transaction{
			ID:          fmt.Sprintf("transaction-second-%s", fake.Lorem().Word()),
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusPending,
			Kind:        domain.TransactionKindRefund,
			AmountMinor: 5_00,
			Currency:    "USD",
			Description: fmt.Sprintf("transaction-second-%s", fake.Lorem().Word()),
			EffectiveAt: now.Add(-time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		_, err = store.SaveTransaction(t.Context(), transactionOne)
		require.NoError(t, err)
		_, err = store.SaveTransaction(t.Context(), transactionTwo)
		require.NoError(t, err)

		loadedTransaction, err := store.GetTransaction(t.Context(), transactionOne.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedTransaction)
		require.NotNil(t, loadedTransaction.ProviderOriginal)
		assert.Nil(t, loadedTransaction.TransferMatchedAt)

		providerTransactions, err := store.ListTransactions(
			t.Context(),
			tenant.ID,
			account.ID,
			domain.TransactionSourceProvider,
			"",
			true,
		)
		require.NoError(t, err)
		require.Len(t, providerTransactions, 1)

		pendingTransactions, err := store.ListTransactions(
			t.Context(),
			tenant.ID,
			"",
			"",
			domain.TransactionStatusPending,
			true,
		)
		require.NoError(t, err)
		require.Len(t, pendingTransactions, 1)

		pagedTransactions, err := store.ListTransactions(
			t.Context(),
			tenant.ID,
			"",
			"",
			"",
			true,
			ListTransactionsPage{Limit: 1, Offset: 1},
		)
		require.NoError(t, err)
		require.Len(t, pagedTransactions, 1)
		assert.Equal(t, transactionTwo.ID, pagedTransactions[0].ID)

		transactionOne.HiddenAt = &hiddenAt
		_, err = store.SaveTransaction(t.Context(), transactionOne)
		require.NoError(t, err)

		visibleTransactions, err := store.ListTransactions(
			t.Context(),
			tenant.ID,
			"",
			"",
			"",
			false,
		)
		require.NoError(t, err)
		require.Len(t, visibleTransactions, 1)
	})

	t.Run("aggregates account balances for one or many accounts", func(t *testing.T) {
		store := makeStore(t)
		balanceStore := NewAccountBalanceStore(&Database{db: store.db})
		fake := faker.New()
		now := time.Date(2026, time.July, 3, 10, 0, 0, 0, time.UTC)

		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		checking := domain.Account{
			ID:        "account-checking-" + fake.UUID().V4(),
			TenantID:  tenant.ID,
			Name:      "checking-" + fake.Lorem().Word(),
			Currency:  "USD",
			Kind:      domain.AccountKindManual,
			CreatedAt: now,
			UpdatedAt: now,
		}
		savings := domain.Account{
			ID:        "account-savings-" + fake.UUID().V4(),
			TenantID:  tenant.ID,
			Name:      "savings-" + fake.Lorem().Word(),
			Currency:  "USD",
			Kind:      domain.AccountKindManual,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err = store.SaveAccount(t.Context(), checking)
		require.NoError(t, err)
		_, err = store.SaveAccount(t.Context(), savings)
		require.NoError(t, err)

		saveTransaction := func(
			accountID string,
			status domain.TransactionStatus,
			kind domain.TransactionKind,
			amount int64,
			effectiveAt time.Time,
			hidden bool,
		) {
			t.Helper()
			transaction := domain.Transaction{
				ID:          "transaction-" + fake.UUID().V4(),
				TenantID:    tenant.ID,
				AccountID:   accountID,
				Source:      domain.TransactionSourceManual,
				Status:      status,
				Kind:        kind,
				AmountMinor: amount,
				Currency:    "USD",
				Description: "transaction-" + fake.Lorem().Word(),
				EffectiveAt: effectiveAt,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if hidden {
				hiddenAt := now.Add(time.Minute)
				transaction.HiddenAt = &hiddenAt
			}
			_, saveErr := store.SaveTransaction(t.Context(), transaction)
			require.NoError(t, saveErr)
		}

		saveTransaction(
			checking.ID,
			domain.TransactionStatusBooked,
			domain.TransactionKindOpeningBalance,
			100_00,
			now,
			false,
		)
		saveTransaction(checking.ID, domain.TransactionStatusBooked, domain.TransactionKindRefund, 10_00, now, false)
		saveTransaction(
			checking.ID,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			-30_00,
			now,
			false,
		)
		saveTransaction(checking.ID, domain.TransactionStatusPending, domain.TransactionKindRegular, -12_00, now, false)
		saveTransaction(checking.ID, domain.TransactionStatusBooked, domain.TransactionKindRegular, 99_00, now, true)
		saveTransaction(savings.ID, domain.TransactionStatusBooked, domain.TransactionKindTransfer, 30_00, now, false)
		saveTransaction(
			savings.ID,
			domain.TransactionStatusBooked,
			domain.TransactionKindReconciliation,
			5_00,
			now,
			false,
		)
		saveTransaction(
			checking.ID,
			domain.TransactionStatusBooked,
			domain.TransactionKindRegular,
			20_00,
			time.Date(
				now.Year(), now.Month(), now.Day(), 23, 30, 0, 0,
				time.FixedZone("UTC-10", -10*60*60),
			),
			false,
		)

		balances, err := balanceStore.ListAccountBalances(t.Context(), ListAccountBalancesParams{
			TenantID:   tenant.ID,
			AccountIDs: []string{checking.ID, savings.ID},
		})
		require.NoError(t, err)
		require.Len(t, balances, 2)
		assert.Equal(
			t,
			domain.AccountBalance{AccountID: checking.ID, BookedBalanceMinor: 100_00, PendingBalanceMinor: -12_00},
			balances[0],
		)
		assert.Equal(
			t,
			domain.AccountBalance{AccountID: savings.ID, BookedBalanceMinor: 35_00, PendingBalanceMinor: 0},
			balances[1],
		)

		checkingOnly, err := balanceStore.ListAccountBalances(t.Context(), ListAccountBalancesParams{
			TenantID:   tenant.ID,
			AccountIDs: []string{checking.ID},
		})
		require.NoError(t, err)
		require.Len(t, checkingOnly, 1)
		assert.Equal(
			t,
			domain.AccountBalance{AccountID: checking.ID, BookedBalanceMinor: 100_00, PendingBalanceMinor: -12_00},
			checkingOnly[0],
		)

		allBalances, err := balanceStore.ListAccountBalances(t.Context(), ListAccountBalancesParams{
			TenantID: tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, allBalances, 2)

		cutoff := now
		cutoffBalances, err := balanceStore.ListAccountBalances(t.Context(), ListAccountBalancesParams{
			TenantID:              tenant.ID,
			AccountIDs:            []string{checking.ID},
			EffectiveAtOnOrBefore: &cutoff,
		})
		require.NoError(t, err)
		require.Len(t, cutoffBalances, 1)
		assert.Equal(t, int64(80_00), cutoffBalances[0].BookedBalanceMinor)

		emptyBalances, err := balanceStore.ListAccountBalances(t.Context(), ListAccountBalancesParams{
			TenantID:   tenant.ID,
			AccountIDs: []string{},
		})
		require.NoError(t, err)
		assert.Empty(t, emptyBalances)
	})

	t.Run("persists tenant archive state without deleting tenant-owned data", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.July, 3, 8, 0, 0, 0, time.UTC)
		archivedAt := now.Add(15 * time.Minute)

		tenant := domain.Tenant{
			ID:              "tenant-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err := store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		membership := domain.TenantMembership{
			TenantID:  tenant.ID,
			UserID:    "user-" + fake.UUID().V4(),
			JoinedAt:  now,
			CreatedAt: now,
		}
		_, err = store.SaveTenantMembership(t.Context(), membership)
		require.NoError(t, err)

		account := domain.Account{
			ID:        "account-" + fake.UUID().V4(),
			TenantID:  tenant.ID,
			Name:      "account-" + fake.Lorem().Word(),
			Currency:  "USD",
			Kind:      domain.AccountKindManual,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err = store.SaveAccount(t.Context(), account)
		require.NoError(t, err)

		tenant.ArchivedAt = &archivedAt
		tenant.UpdatedAt = archivedAt
		_, err = store.SaveTenant(t.Context(), tenant)
		require.NoError(t, err)

		loadedTenant, err := store.GetTenant(t.Context(), tenant.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedTenant)
		require.NotNil(t, loadedTenant.ArchivedAt)
		assert.True(t, archivedAt.Equal(*loadedTenant.ArchivedAt))

		views, err := store.ListTenantsForUser(t.Context(), membership.UserID)
		require.NoError(t, err)
		assert.Empty(t, views)

		loadedAccount, err := store.GetAccount(t.Context(), account.ID)
		require.NoError(t, err)
		require.NotNil(t, loadedAccount)
		assert.Equal(t, account.ID, loadedAccount.ID)
	})

	t.Run(
		"stores encrypted secrets without changing supplied timestamp offsets",
		func(t *testing.T) {
			store := makeStore(t)
			fake := faker.New()

			cipher, err := credentials.NewAESGCMCipher(
				[]byte("0123456789abcdef0123456789abcdef"),
				"fixture-key",
			)
			require.NoError(t, err)

			plaintext := fmt.Sprintf("secret-%s-%d", fake.Lorem().Word(), fake.Int())
			envelope, err := cipher.SealString(plaintext)
			require.NoError(t, err)

			createdAt := time.Date(
				2026,
				time.June,
				20,
				14,
				30,
				0,
				0,
				time.FixedZone("offset", 2*60*60),
			)
			secret, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
				ID:        fmt.Sprintf("secret-%s", fake.Lorem().Word()),
				Provider:  fmt.Sprintf("provider-%s", fake.Lorem().Word()),
				Reference: fmt.Sprintf("reference-%s", fake.Lorem().Word()),
				Envelope:  envelope,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			})
			require.NoError(t, err)

			assert.True(t, createdAt.Equal(secret.CreatedAt))
			assert.True(t, createdAt.Equal(secret.UpdatedAt))

			sqlDB, err := store.db.DB()
			require.NoError(t, err)

			var ciphertext string
			err = sqlDB.QueryRowContext(
				t.Context(),
				"SELECT ciphertext FROM finance_connection_secrets WHERE id = $1",
				secret.ID,
			).Scan(&ciphertext)
			require.NoError(t, err)
			assert.NotContains(t, ciphertext, plaintext)

			loaded, err := store.GetConnectionSecret(t.Context(), secret.ID)
			require.NoError(t, err)
			require.NotNil(t, loaded)
			opened, err := cipher.OpenString(loaded.Envelope)
			require.NoError(t, err)
			assert.Equal(t, plaintext, opened)

			zeroTimeSecret, err := store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
				ID:        fmt.Sprintf("secret-zero-%s", fake.Lorem().Word()),
				Provider:  fmt.Sprintf("provider-zero-%s", fake.Lorem().Word()),
				Reference: fmt.Sprintf("reference-zero-%s", fake.Lorem().Word()),
				Envelope:  envelope,
			})
			require.NoError(t, err)
			assert.False(t, zeroTimeSecret.CreatedAt.IsZero())
			assert.False(t, zeroTimeSecret.UpdatedAt.IsZero())
		},
	)

	t.Run(
		"returns not found for unknown secrets and persists fixture bootstrap records",
		func(t *testing.T) {
			store := makeStore(t)
			fake := faker.New()

			missingID := fmt.Sprintf("missing-%s", fake.Lorem().Word())
			loaded, err := store.GetConnectionSecret(t.Context(), missingID)
			require.ErrorIs(t, err, ErrConnectionSecretNotFound)
			assert.Nil(t, loaded)

			runID := fmt.Sprintf("run-%s", fake.Lorem().Word())
			require.NoError(
				t,
				store.CreateFixtureBootstrapRun(t.Context(), domain.FixtureBootstrapRun{
					ID:       runID,
					Seed:     5,
					Scenario: fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
					StartedAt: time.Date(
						2026,
						time.June,
						20,
						18,
						0,
						0,
						0,
						time.FixedZone("fixture", 5*60*60),
					),
				}),
			)
			require.NoError(
				t,
				store.CreateFixtureBootstrapRun(t.Context(), domain.FixtureBootstrapRun{
					ID:       fmt.Sprintf("run-zero-%s", fake.Lorem().Word()),
					Seed:     6,
					Scenario: fmt.Sprintf("scenario-zero-%s", fake.Lorem().Word()),
				}),
			)
			require.NoError(
				t,
				store.CreateFixtureScenarioRecord(t.Context(), runID, domain.FixtureScenarioRecord{
					Name:     fmt.Sprintf("record-%s", fake.Lorem().Word()),
					StableID: fmt.Sprintf("stable-%s", fake.Lorem().Word()),
					OccurredAt: time.Date(
						2026,
						time.June,
						20,
						18,
						5,
						0,
						0,
						time.FixedZone("fixture", 5*60*60),
					),
				}),
			)
			firstStableID := fmt.Sprintf("stable-zero-%s", fake.Lorem().Word())
			require.NoError(
				t,
				store.CreateFixtureScenarioRecord(t.Context(), runID, domain.FixtureScenarioRecord{
					Name:     fmt.Sprintf("record-zero-%s", fake.Lorem().Word()),
					StableID: firstStableID,
				}),
			)

			sqlDB, err := store.db.DB()
			require.NoError(t, err)

			var startedAt time.Time
			err = sqlDB.QueryRowContext(
				t.Context(),
				"SELECT started_at FROM finance_fixture_bootstrap_runs WHERE id = $1",
				runID,
			).Scan(&startedAt)
			require.NoError(t, err)
			expectedStartedAt := time.Date(
				2026,
				time.June,
				20,
				18,
				0,
				0,
				0,
				time.FixedZone("fixture", 5*60*60),
			)
			assert.True(t, expectedStartedAt.Equal(startedAt))

			var recordID string
			var occurredAt time.Time
			err = sqlDB.QueryRowContext(
				t.Context(),
				"SELECT id, occurred_at FROM finance_fixture_scenario_records WHERE run_id = $1 AND stable_id = $2",
				runID,
				firstStableID,
			).Scan(&recordID, &occurredAt)
			require.NoError(t, err)
			assert.NotEmpty(t, recordID)
			assert.False(t, occurredAt.IsZero())
		},
	)

	t.Run(
		"persists deterministic fixture scenario record ids for identical payloads",
		func(t *testing.T) {
			fake := faker.New()
			runID := fmt.Sprintf("run-%s", fake.Lorem().Word())
			record := domain.FixtureScenarioRecord{
				Name:     fmt.Sprintf("record-%s", fake.Lorem().Word()),
				StableID: fmt.Sprintf("stable-%s", fake.Lorem().Word()),
				OccurredAt: time.Date(
					2026,
					time.June,
					20,
					18,
					10,
					0,
					0,
					time.FixedZone("fixture", -3*60*60),
				),
			}

			persistRecordID := func(t *testing.T) string {
				t.Helper()

				store := makeStore(t)
				require.NoError(
					t,
					store.CreateFixtureBootstrapRun(t.Context(), domain.FixtureBootstrapRun{
						ID:        runID,
						Seed:      7,
						Scenario:  fmt.Sprintf("scenario-%s", fake.Lorem().Word()),
						StartedAt: time.Date(2026, time.June, 20, 18, 0, 0, 0, time.UTC),
					}),
				)
				require.NoError(t, store.CreateFixtureScenarioRecord(t.Context(), runID, record))

				sqlDB, err := store.db.DB()
				require.NoError(t, err)

				var recordID string
				err = sqlDB.QueryRowContext(
					t.Context(),
					"SELECT id FROM finance_fixture_scenario_records WHERE run_id = $1 AND stable_id = $2",
					runID,
					record.StableID,
				).Scan(&recordID)
				require.NoError(t, err)
				require.NoError(t, store.DB().Table((fixtureScenarioRecordModel{}).TableName()).
					Where("run_id = ?", runID).Delete(&fixtureScenarioRecordModel{}).Error)
				require.NoError(t, store.DB().Table((fixtureBootstrapRunModel{}).TableName()).
					Where("id = ?", runID).Delete(&fixtureBootstrapRunModel{}).Error)
				return recordID
			}

			firstRecordID := persistRecordID(t)
			secondRecordID := persistRecordID(t)

			assert.NotEmpty(t, firstRecordID)
			assert.Equal(t, firstRecordID, secondRecordID)
		},
	)

	t.Run("saves linked transfer pairs atomically and persists matched marker", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.June, 20, 19, 0, 0, 0, time.UTC)
		tenantID := fmt.Sprintf("tenant-%s", fake.Lorem().Word())
		accountID := fmt.Sprintf("account-%s", fake.Lorem().Word())
		groupID := fmt.Sprintf("group-%s", fake.Lorem().Word())

		_, err := store.SaveTenant(t.Context(), domain.Tenant{
			ID:              tenantID,
			Name:            fmt.Sprintf("tenant-%s", fake.Company().Name()),
			DisplayCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
		})
		require.NoError(t, err)

		_, err = store.SaveAccount(t.Context(), domain.Account{
			ID:        accountID,
			TenantID:  tenantID,
			Name:      fmt.Sprintf("account-%s", fake.Lorem().Word()),
			Currency:  "USD",
			Kind:      domain.AccountKindManual,
			CreatedAt: now,
			UpdatedAt: now,
		})
		require.NoError(t, err)

		firstTransfer := domain.Transaction{
			ID:          fmt.Sprintf("transaction-first-%s", fake.Lorem().Word()),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindTransfer,
			AmountMinor: -12_00,
			Currency:    "USD",
			Description: fmt.Sprintf("transfer-first-%s", fake.Lorem().Word()),
			EffectiveAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		secondTransfer := domain.Transaction{
			ID:          fmt.Sprintf("transaction-second-%s", fake.Lorem().Word()),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindTransfer,
			AmountMinor: 9_00,
			Currency:    "USD",
			Description: fmt.Sprintf("transfer-second-%s", fake.Lorem().Word()),
			EffectiveAt: now.Add(time.Minute),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		_, err = store.SaveTransaction(t.Context(), firstTransfer)
		require.NoError(t, err)
		_, err = store.SaveTransaction(t.Context(), secondTransfer)
		require.NoError(t, err)

		firstTransfer.TransferGroupID = &groupID
		secondTransfer.TransferGroupID = &groupID
		firstTransfer.TransferMatchedAt = &now
		secondTransfer.TransferMatchedAt = &now
		firstTransfer.UpdatedAt = now.Add(2 * time.Minute)
		secondTransfer.UpdatedAt = now.Add(2 * time.Minute)

		err = store.SaveLinkedTransferPair(t.Context(), firstTransfer, secondTransfer)
		require.NoError(t, err)

		storedMatchedFirst, err := store.GetTransaction(t.Context(), firstTransfer.ID)
		require.NoError(t, err)
		storedMatchedSecond, err := store.GetTransaction(t.Context(), secondTransfer.ID)
		require.NoError(t, err)
		require.NotNil(t, storedMatchedFirst.TransferMatchedAt)
		require.NotNil(t, storedMatchedSecond.TransferMatchedAt)
		assert.True(t, now.Equal(*storedMatchedFirst.TransferMatchedAt))
		assert.True(t, now.Equal(*storedMatchedSecond.TransferMatchedAt))

		firstTransfer.TransferMatchedAt = nil
		secondTransfer.TransferMatchedAt = nil
		firstTransfer.TransferGroupID = nil
		secondTransfer.TransferGroupID = nil
		firstTransfer.UpdatedAt = now.Add(3 * time.Minute)
		secondTransfer.UpdatedAt = now.Add(3 * time.Minute)
		_, err = store.SaveTransaction(t.Context(), firstTransfer)
		require.NoError(t, err)
		_, err = store.SaveTransaction(t.Context(), secondTransfer)
		require.NoError(t, err)

		callbackName := fmt.Sprintf("test:fail-second-linked-transfer-%s", fake.Lorem().Word())
		var createCalls int
		sentinel := errors.New("second write failed")
		require.NoError(
			t,
			store.db.Callback().
				Create().
				Before("gorm:create").
				Register(callbackName, func(tx *gorm.DB) {
					if tx.Statement.Table != (transactionModel{}).TableName() {
						return
					}
					createCalls++
					if createCalls == 2 {
						tx.AddError(sentinel)
					}
				}),
		)
		defer func() {
			store.db.Callback().Create().Remove(callbackName)
		}()

		err = store.SaveLinkedTransferPair(t.Context(), firstTransfer, secondTransfer)
		require.ErrorIs(t, err, sentinel)

		storedFirst, err := store.GetTransaction(t.Context(), firstTransfer.ID)
		require.NoError(t, err)
		storedSecond, err := store.GetTransaction(t.Context(), secondTransfer.ID)
		require.NoError(t, err)
		assert.Nil(t, storedFirst.TransferGroupID)
		assert.Nil(t, storedSecond.TransferGroupID)
	})

	t.Run("returns persistence errors when tables are missing", func(t *testing.T) {
		fake := faker.New()
		store := makeStore(t)
		sqlDB, err := store.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		cipher, err := credentials.NewAESGCMCipher(
			[]byte("0123456789abcdef0123456789abcdef"),
			"fixture-key",
		)
		require.NoError(t, err)
		envelope, err := cipher.SealString(fmt.Sprintf("secret-%s", fake.Lorem().Word()))
		require.NoError(t, err)

		_, err = store.SaveConnectionSecret(t.Context(), domain.ConnectionSecret{
			ID:        fmt.Sprintf("secret-%s", fake.Lorem().Word()),
			Provider:  fmt.Sprintf("provider-%s", fake.Lorem().Word()),
			Reference: fmt.Sprintf("reference-%s", fake.Lorem().Word()),
			Envelope:  envelope,
		})
		require.Error(t, err)

		_, err = store.GetConnectionSecret(
			t.Context(),
			fmt.Sprintf("missing-%s", fake.Lorem().Word()),
		)
		require.Error(t, err)

		err = store.CreateFixtureBootstrapRun(
			t.Context(),
			domain.FixtureBootstrapRun{ID: fmt.Sprintf("run-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		err = store.CreateFixtureScenarioRecord(
			t.Context(),
			fmt.Sprintf("run-%s", fake.Lorem().Word()),
			domain.FixtureScenarioRecord{Name: fmt.Sprintf("record-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.SaveTenant(
			t.Context(),
			domain.Tenant{ID: fmt.Sprintf("tenant-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.SaveTenantMembership(
			t.Context(),
			domain.TenantMembership{TenantID: fmt.Sprintf("tenant-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.ListTenantsForUser(t.Context(), fmt.Sprintf("user-%s", fake.Lorem().Word()))
		require.Error(t, err)

		_, err = store.IsTenantMember(
			t.Context(),
			fmt.Sprintf("tenant-%s", fake.Lorem().Word()),
			fmt.Sprintf("user-%s", fake.Lorem().Word()),
		)
		require.Error(t, err)

		_, err = store.SaveTenantInvite(
			t.Context(),
			domain.TenantInvite{ID: fmt.Sprintf("invite-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.GetTenantInviteByCode(
			t.Context(),
			fmt.Sprintf("code-%s", fake.Lorem().Word()),
		)
		require.Error(t, err)

		_, err = store.UpdateTenantInvite(
			t.Context(),
			domain.TenantInvite{ID: fmt.Sprintf("invite-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.ListTenantMembers(t.Context(), fmt.Sprintf("tenant-%s", fake.Lorem().Word()))
		require.Error(t, err)

		_, err = store.SaveAccount(
			t.Context(),
			domain.Account{ID: fmt.Sprintf("account-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.GetAccount(t.Context(), fmt.Sprintf("account-%s", fake.Lorem().Word()))
		require.Error(t, err)

		_, err = store.ListAccounts(
			t.Context(),
			fmt.Sprintf("tenant-%s", fake.Lorem().Word()),
			false,
		)
		require.Error(t, err)

		_, err = store.SaveCategory(
			t.Context(),
			domain.Category{ID: fmt.Sprintf("category-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.GetCategory(t.Context(), fmt.Sprintf("category-%s", fake.Lorem().Word()))
		require.Error(t, err)

		_, err = store.ListCategories(
			t.Context(),
			fmt.Sprintf("tenant-%s", fake.Lorem().Word()),
			false,
		)
		require.Error(t, err)

		_, err = store.SaveTag(
			t.Context(),
			domain.Tag{ID: fmt.Sprintf("tag-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.GetTag(t.Context(), fmt.Sprintf("tag-%s", fake.Lorem().Word()))
		require.Error(t, err)

		_, err = store.ListTags(t.Context(), fmt.Sprintf("tenant-%s", fake.Lorem().Word()), false)
		require.Error(t, err)

		_, err = store.SaveTransaction(
			t.Context(),
			domain.Transaction{ID: fmt.Sprintf("transaction-%s", fake.Lorem().Word())},
		)
		require.Error(t, err)

		_, err = store.GetTransaction(
			t.Context(),
			fmt.Sprintf("transaction-%s", fake.Lorem().Word()),
		)
		require.Error(t, err)

		_, err = store.ListTransactions(
			t.Context(),
			fmt.Sprintf("tenant-%s", fake.Lorem().Word()),
			"",
			"",
			"",
			false,
		)
		require.Error(t, err)
	})

	t.Run("persists one current fx rate per provider pair", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		frankfurterProvider := "frankfurter-" + fake.UUID().V4()
		ecbProvider := "ecb-" + fake.UUID().V4()
		firstDate := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
		secondDate := time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC)

		require.NoError(t, store.SaveFXRates(t.Context(), []domain.FXRate{
			{
				Provider:      frankfurterProvider,
				BaseCurrency:  "USD",
				QuoteCurrency: "PLN",
				RateDate:      firstDate,
				Rate:          4.10,
			},
			{
				Provider:      frankfurterProvider,
				BaseCurrency:  "USD",
				QuoteCurrency: "PLN",
				RateDate:      secondDate,
				Rate:          4.12,
			},
			{
				Provider:      ecbProvider,
				BaseCurrency:  "USD",
				QuoteCurrency: "PLN",
				RateDate:      firstDate,
				Rate:          4.11,
			},
		}))

		require.NoError(t, store.SaveFXRates(t.Context(), []domain.FXRate{{
			Provider:      frankfurterProvider,
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			RateDate:      firstDate,
			Rate:          4.15,
		}}))

		frankfurterRates, err := store.ListFXRates(t.Context(), ListFXRatesParams{
			Provider:      frankfurterProvider,
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
		})
		require.NoError(t, err)
		require.Len(t, frankfurterRates, 1)
		assert.InDelta(t, 4.15, frankfurterRates[0].Rate, 0.00001)
		assert.True(t, firstDate.Equal(frankfurterRates[0].RateDate))

		windowRates, err := store.ListFXRates(t.Context(), ListFXRatesParams{
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			StartDate:     secondDate,
			EndDate:       secondDate,
		})
		require.NoError(t, err)
		ratesByProvider := make(map[string]domain.FXRate, 2)
		for _, rate := range windowRates {
			if rate.Provider == frankfurterProvider || rate.Provider == ecbProvider {
				ratesByProvider[rate.Provider] = rate
			}
		}
		require.Len(t, ratesByProvider, 2)
		assert.InDelta(t, 4.11, ratesByProvider[ecbProvider].Rate, 0.00001)
		assert.InDelta(t, 4.15, ratesByProvider[frankfurterProvider].Rate, 0.00001)

		_, err = store.ListFXRates(t.Context(), ListFXRatesParams{
			StartDate: secondDate,
			EndDate:   firstDate,
		})
		require.NoError(t, err)

		err = store.SaveFXRates(t.Context(), []domain.FXRate{{
			Provider:      frankfurterProvider,
			BaseCurrency:  "USD",
			QuoteCurrency: "PLN",
			Rate:          4.15,
		}})
		require.Error(t, err)
	})

	t.Run("discovers active tenant account and transaction FX pairs", func(t *testing.T) {
		store := makeStore(t)
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 11, 0, 0, 0, time.UTC)
		currency := func(prefix string) string { return strings.ToUpper(prefix + fake.UUID().V4()[:8]) }
		initialQuoteCurrency := currency("q")
		updatedQuoteCurrency := currency("r")
		accountCurrency := currency("a")
		transactionCurrency := currency("t")
		secondTenantCurrency := currency("s")
		activeTenant := domain.Tenant{
			ID:              "tenant-active-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: initialQuoteCurrency,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		secondActiveTenant := domain.Tenant{
			ID:              "tenant-second-active-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: initialQuoteCurrency,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		archivedTenant := domain.Tenant{
			ID:              "tenant-archived-" + fake.UUID().V4(),
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "PLN",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		archivedAt := now.Add(time.Minute)
		archivedTenant.ArchivedAt = &archivedAt
		require.NoError(t, func() error {
			_, err := store.SaveTenant(t.Context(), activeTenant)
			return err
		}())
		require.NoError(t, func() error {
			_, err := store.SaveTenant(t.Context(), archivedTenant)
			return err
		}())
		require.NoError(t, func() error {
			_, err := store.SaveTenant(t.Context(), secondActiveTenant)
			return err
		}())
		t.Cleanup(func() {
			cleanupAt := now.Add(3 * time.Minute)
			for _, tenant := range []domain.Tenant{activeTenant, secondActiveTenant} {
				tenant.ArchivedAt = &cleanupAt
				tenant.UpdatedAt = cleanupAt
				_, cleanupErr := store.SaveTenant(context.WithoutCancel(t.Context()), tenant)
				require.NoError(t, cleanupErr)
			}
		})
		saveAccount := func(tenantID string, currency string) string {
			t.Helper()
			account := domain.Account{
				ID:        "account-" + fake.UUID().V4(),
				TenantID:  tenantID,
				Name:      "account-" + fake.Lorem().Word(),
				Currency:  currency,
				Kind:      domain.AccountKindManual,
				CreatedAt: now,
				UpdatedAt: now,
			}
			_, err := store.SaveAccount(t.Context(), account)
			require.NoError(t, err)
			return account.ID
		}
		accountID := saveAccount(activeTenant.ID, accountCurrency)
		_ = saveAccount(activeTenant.ID, initialQuoteCurrency)
		_ = saveAccount(archivedTenant.ID, currency("x"))
		_ = saveAccount(secondActiveTenant.ID, secondTenantCurrency)
		saveTransaction := func(currency string) {
			t.Helper()
			_, err := store.SaveTransaction(t.Context(), domain.Transaction{
				ID:          "transaction-" + fake.UUID().V4(),
				TenantID:    activeTenant.ID,
				AccountID:   accountID,
				Source:      domain.TransactionSourceManual,
				Status:      domain.TransactionStatusBooked,
				Kind:        domain.TransactionKindRegular,
				AmountMinor: -100,
				Currency:    currency,
				Description: "transaction-" + fake.Lorem().Word(),
				EffectiveAt: now,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			require.NoError(t, err)
		}
		saveTransaction(transactionCurrency)
		saveTransaction(transactionCurrency)

		discovery := NewFXPairDiscoveryStore(&Database{db: store.db})
		pairs, err := discovery.ListRequiredFXPairs(t.Context())
		require.NoError(t, err)
		initialPairs := make([]RequiredFXPair, 0, len(pairs))
		for _, pair := range pairs {
			if pair.QuoteCurrency == initialQuoteCurrency {
				initialPairs = append(initialPairs, pair)
			}
		}
		assert.Equal(t, []RequiredFXPair{
			{BaseCurrency: accountCurrency, QuoteCurrency: initialQuoteCurrency},
			{BaseCurrency: secondTenantCurrency, QuoteCurrency: initialQuoteCurrency},
			{BaseCurrency: transactionCurrency, QuoteCurrency: initialQuoteCurrency},
		}, initialPairs)

		activeTenant.DisplayCurrency = updatedQuoteCurrency
		activeTenant.UpdatedAt = now.Add(2 * time.Minute)
		_, err = store.SaveTenant(t.Context(), activeTenant)
		require.NoError(t, err)
		pairs, err = discovery.ListRequiredFXPairs(t.Context())
		require.NoError(t, err)
		updatedPairs := make([]RequiredFXPair, 0, len(pairs))
		for _, pair := range pairs {
			if pair.QuoteCurrency == updatedQuoteCurrency {
				updatedPairs = append(updatedPairs, pair)
			}
		}
		assert.Equal(t, []RequiredFXPair{
			{BaseCurrency: accountCurrency, QuoteCurrency: updatedQuoteCurrency},
			{BaseCurrency: initialQuoteCurrency, QuoteCurrency: updatedQuoteCurrency},
			{BaseCurrency: transactionCurrency, QuoteCurrency: updatedQuoteCurrency},
		}, updatedPairs)
	})
}
