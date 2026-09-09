package persistence

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTransferPairStore(t *testing.T) {
	makeTransaction := func(fake faker.Faker, tenantID string, accountID string, now time.Time) domain.Transaction {
		categoryID := "category-" + fake.UUID().V4()
		return domain.Transaction{
			ID:          "transaction-" + fake.UUID().V4(),
			TenantID:    tenantID,
			AccountID:   accountID,
			Source:      domain.TransactionSourceProvider,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -int64(fake.IntBetween(1, 10_000)),
			Currency:    "USD",
			Description: "transaction-" + fake.Lorem().Word(),
			EffectiveAt: now,
			CategoryID:  &categoryID,
			CreatedAt:   now,
			UpdatedAt:   now,
			ProviderOriginal: &domain.ProviderTransactionOriginal{
				AmountMinor: -int64(fake.IntBetween(1, 10_000)),
				Currency:    "USD",
				Description: "provider-" + fake.Lorem().Word(),
				EffectiveAt: &now,
			},
		}
	}
	makeParams := func(fake faker.Faker, tenantID string, firstID string, secondID string, now time.Time) TransferPairLinkParams {
		return TransferPairLinkParams{
			TenantID:            tenantID,
			FirstTransactionID:  firstID,
			SecondTransactionID: secondID,
			TransferGroupID:     "group-" + fake.UUID().V4(),
			TransferMatchedAt:   now.Add(time.Minute),
			UpdatedAt:           now.Add(2 * time.Minute),
		}
	}
	makeStores := func(t *testing.T) (*Database, *Store, *TransactionTagStore, *TransferPairStore) {
		t.Helper()
		database := openTestDatabase(t)
		return database, NewStore(database), NewTransactionTagStore(database), NewTransferPairStore(database)
	}
	assertLedgerDataPreserved := func(t *testing.T, expected domain.Transaction, actual domain.Transaction) {
		t.Helper()
		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.TenantID, actual.TenantID)
		assert.Equal(t, expected.AccountID, actual.AccountID)
		assert.Equal(t, expected.Source, actual.Source)
		assert.Equal(t, expected.Status, actual.Status)
		assert.Equal(t, expected.AmountMinor, actual.AmountMinor)
		assert.Equal(t, expected.Currency, actual.Currency)
		assert.Equal(t, expected.Description, actual.Description)
		assert.True(t, expected.EffectiveAt.Equal(actual.EffectiveAt))
		assert.Equal(t, expected.CategoryID, actual.CategoryID)
		assert.ElementsMatch(t, expected.TagIDs, actual.TagIDs)
		require.NotNil(t, expected.ProviderOriginal)
		require.NotNil(t, actual.ProviderOriginal)
		assert.Equal(t, expected.ProviderOriginal.AmountMinor, actual.ProviderOriginal.AmountMinor)
		assert.Equal(t, expected.ProviderOriginal.Currency, actual.ProviderOriginal.Currency)
		assert.Equal(t, expected.ProviderOriginal.Description, actual.ProviderOriginal.Description)
		require.NotNil(t, expected.ProviderOriginal.EffectiveAt)
		require.NotNil(t, actual.ProviderOriginal.EffectiveAt)
		assert.True(t, expected.ProviderOriginal.EffectiveAt.Equal(*actual.ProviderOriginal.EffectiveAt))
	}

	t.Run("links and unlinks existing tenant rows without changing ledger data", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.FixedZone("pairs", 2*60*60))
		_, coreStore, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		firstTag := domain.Tag{
			ID:        "tag-first-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		secondTag := domain.Tag{
			ID:        "tag-second-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		for _, tag := range []domain.Tag{firstTag, secondTag} {
			_, err := coreStore.SaveTag(t.Context(), tag)
			require.NoError(t, err)
		}
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		second := makeTransaction(fake, tenantID, "account-second-"+fake.UUID().V4(), now.Add(time.Hour))
		first.TagIDs = []string{firstTag.ID}
		second.TagIDs = []string{secondTag.ID}
		second.AmountMinor = -first.AmountMinor + 1
		second.Currency = "EUR"
		first.TransferMatchingExcluded = true
		for _, transaction := range []domain.Transaction{first, second} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		params := makeParams(fake, tenantID, first.ID, second.ID, now)
		require.NoError(t, store.LinkTransferPair(t.Context(), params))
		linkedFirst, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		linkedSecond, err := transactions.GetTransaction(t.Context(), second.ID)
		require.NoError(t, err)
		for _, linked := range []*domain.Transaction{linkedFirst, linkedSecond} {
			assert.Equal(t, domain.TransactionKindTransfer, linked.Kind)
			assert.Equal(t, params.TransferGroupID, *linked.TransferGroupID)
			assert.True(t, params.TransferMatchedAt.Equal(*linked.TransferMatchedAt))
			assert.True(t, params.UpdatedAt.Equal(linked.UpdatedAt))
		}
		assert.True(t, linkedFirst.TransferMatchingExcluded)
		assert.False(t, linkedSecond.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *linkedFirst)
		assertLedgerDataPreserved(t, second, *linkedSecond)

		unlinkParams := TransferPairUnlinkParams{
			TenantID:            tenantID,
			FirstTransactionID:  first.ID,
			SecondTransactionID: second.ID,
			UpdatedAt:           now.Add(3 * time.Minute),
		}
		require.NoError(t, store.UnlinkTransferPair(t.Context(), unlinkParams))
		for _, transactionID := range []string{first.ID, second.ID} {
			unlinked, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.TransactionKindRegular, unlinked.Kind)
			assert.Nil(t, unlinked.TransferGroupID)
			assert.Nil(t, unlinked.TransferMatchedAt)
			assert.True(t, unlinked.TransferMatchingExcluded)
			assert.True(t, unlinkParams.UpdatedAt.Equal(unlinked.UpdatedAt))
			if transactionID == first.ID {
				assertLedgerDataPreserved(t, first, *unlinked)
			} else {
				assertLedgerDataPreserved(t, second, *unlinked)
			}
		}
	})

	t.Run("rolls back when either tenant-scoped row is missing", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.FixedZone("pairs", -3*60*60))
		_, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		_, err := transactions.SaveTransaction(t.Context(), first)
		require.NoError(t, err)

		params := makeParams(fake, tenantID, first.ID, "missing-"+fake.UUID().V4(), now)
		require.ErrorIs(t, store.LinkTransferPair(t.Context(), params), ErrTransferPairTransactionNotFound)
		stored, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		assert.Equal(t, first.Kind, stored.Kind)
		assert.Nil(t, stored.TransferGroupID)
		assert.Nil(t, stored.TransferMatchedAt)
		assert.Equal(t, first.TransferMatchingExcluded, stored.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *stored)
		_, err = transactions.GetTransaction(t.Context(), params.SecondTransactionID)
		require.ErrorIs(t, err, ErrTransactionNotFound)
	})

	t.Run("rejects duplicate transaction IDs without updating the row", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 13, 0, 0, 0, time.FixedZone("pairs", -2*60*60))
		_, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		transaction := makeTransaction(fake, tenantID, "account-"+fake.UUID().V4(), now)
		_, err := transactions.SaveTransaction(t.Context(), transaction)
		require.NoError(t, err)

		err = store.LinkTransferPair(t.Context(), makeParams(fake, tenantID, transaction.ID, transaction.ID, now))
		require.ErrorIs(t, err, ErrTransferPairTransactionIDsMustDiffer)
		stored, err := transactions.GetTransaction(t.Context(), transaction.ID)
		require.NoError(t, err)
		assert.Equal(t, transaction.Kind, stored.Kind)
		assert.Nil(t, stored.TransferGroupID)
		assert.Nil(t, stored.TransferMatchedAt)
		assert.Equal(t, transaction.TransferMatchingExcluded, stored.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, transaction, *stored)
	})

	t.Run("rolls back both legs when the second narrow update fails", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.July, 14, 14, 0, 0, 0, time.FixedZone("pairs", 5*60*60))
		database, _, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		first := makeTransaction(fake, tenantID, "account-first-"+fake.UUID().V4(), now)
		second := makeTransaction(fake, tenantID, "account-second-"+fake.UUID().V4(), now)
		for _, transaction := range []domain.Transaction{first, second} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		sentinel := errors.New("second narrow pair update failed")
		callbackName := "transfer-pair-" + fake.UUID().V4()
		var updates int
		require.NoError(
			t,
			database.db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table == (transactionModel{}).TableName() {
					updates++
					if updates == 2 {
						tx.AddError(sentinel)
					}
				}
			}),
		)
		defer database.db.Callback().Update().Remove(callbackName)

		require.ErrorIs(
			t,
			store.LinkTransferPair(t.Context(), makeParams(fake, tenantID, first.ID, second.ID, now)),
			sentinel,
		)
		storedFirst, err := transactions.GetTransaction(t.Context(), first.ID)
		require.NoError(t, err)
		storedSecond, err := transactions.GetTransaction(t.Context(), second.ID)
		require.NoError(t, err)
		assert.Equal(t, first.Kind, storedFirst.Kind)
		assert.Nil(t, storedFirst.TransferGroupID)
		assert.Nil(t, storedFirst.TransferMatchedAt)
		assert.Equal(t, first.TransferMatchingExcluded, storedFirst.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, first, *storedFirst)
		assert.Equal(t, second.Kind, storedSecond.Kind)
		assert.Nil(t, storedSecond.TransferGroupID)
		assert.Nil(t, storedSecond.TransferMatchedAt)
		assert.Equal(t, second.TransferMatchingExcluded, storedSecond.TransferMatchingExcluded)
		assertLedgerDataPreserved(t, second, *storedSecond)
	})

	t.Run("loads one complete compact eligible transfer-matching projection", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.FixedZone("matching", 2*60*60))
		rangeStart := now.Add(-24 * time.Hour)
		rangeEndExclusive := now.Add(24 * time.Hour)
		_, coreStore, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		otherTenantID := "tenant-other-" + fake.UUID().V4()
		categoryID := "category-" + fake.UUID().V4()
		visibleFirst := domain.Account{
			ID: "account-first-" + fake.UUID().V4(), TenantID: tenantID,
			Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
			CreatedAt: now, UpdatedAt: now,
		}
		visibleSecond := domain.Account{
			ID: "account-second-" + fake.UUID().V4(), TenantID: tenantID,
			Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindLinked,
			CreatedAt: now, UpdatedAt: now,
		}
		hiddenAt := now
		hiddenAccount := domain.Account{
			ID: "account-hidden-" + fake.UUID().V4(), TenantID: tenantID,
			Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
			HiddenAt: &hiddenAt, CreatedAt: now, UpdatedAt: now,
		}
		otherAccount := domain.Account{
			ID: "account-other-" + fake.UUID().V4(), TenantID: otherTenantID,
			Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
			CreatedAt: now, UpdatedAt: now,
		}
		for _, account := range []domain.Account{visibleFirst, visibleSecond, hiddenAccount, otherAccount} {
			_, err := coreStore.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}

		makeTransaction := func(id string, accountID string, source domain.TransactionSource, status domain.TransactionStatus, kind domain.TransactionKind, amount int64, effectiveAt time.Time) domain.Transaction {
			return domain.Transaction{
				ID: id, TenantID: tenantID, AccountID: accountID, Source: source, Status: status, Kind: kind,
				AmountMinor: amount, Currency: "USD", Description: "transaction-" + fake.Lorem().Word(),
				EffectiveAt: effectiveAt, CategoryID: &categoryID, CreatedAt: now, UpdatedAt: now,
			}
		}
		eligible := []domain.Transaction{
			makeTransaction(
				"transaction-manual-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-101,
				rangeStart,
			),
			makeTransaction(
				"transaction-csv-"+fake.UUID().V4(),
				visibleSecond.ID,
				domain.TransactionSourceCSV,
				domain.TransactionStatusBooked,
				domain.TransactionKindExpense,
				202,
				rangeEndExclusive.Add(-time.Microsecond),
			),
			makeTransaction(
				"transaction-provider-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceProvider,
				domain.TransactionStatusBooked,
				domain.TransactionKindIncome,
				-303,
				rangeStart.Add(-144*time.Hour),
			),
			makeTransaction(
				"transaction-transfer-"+fake.UUID().V4(),
				visibleSecond.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindTransfer,
				404,
				rangeEndExclusive.Add(144*time.Hour-time.Microsecond),
			),
		}
		for index := range 201 {
			accountID := visibleFirst.ID
			if index%2 == 1 {
				accountID = visibleSecond.ID
			}
			eligible = append(eligible, makeTransaction(
				"transaction-uncapped-"+fake.UUID().V4(), accountID, domain.TransactionSourceManual,
				domain.TransactionStatusBooked, domain.TransactionKindRegular, int64(index+500), now,
			))
		}
		ineligible := []domain.Transaction{
			makeTransaction(
				"transaction-pending-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceProvider,
				domain.TransactionStatusPending,
				domain.TransactionKindRegular,
				-1,
				now,
			),
			makeTransaction(
				"transaction-zero-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				0,
				now,
			),
			makeTransaction(
				"transaction-refund-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRefund,
				-1,
				now,
			),
			makeTransaction(
				"transaction-reconciliation-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindReconciliation,
				-1,
				now,
			),
			makeTransaction(
				"transaction-opening-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindOpeningBalance,
				-1,
				now,
			),
			makeTransaction(
				"transaction-system-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceSystem,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-1,
				now,
			),
			makeTransaction(
				"transaction-hidden-account-"+fake.UUID().V4(),
				hiddenAccount.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-1,
				now,
			),
			makeTransaction(
				"transaction-missing-account-"+fake.UUID().V4(),
				"account-missing-"+fake.UUID().V4(),
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-1,
				now,
			),
			makeTransaction(
				"transaction-before-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-1,
				rangeStart.Add(-144*time.Hour-time.Microsecond),
			),
			makeTransaction(
				"transaction-end-"+fake.UUID().V4(),
				visibleFirst.ID,
				domain.TransactionSourceManual,
				domain.TransactionStatusBooked,
				domain.TransactionKindRegular,
				-1,
				rangeEndExclusive.Add(144*time.Hour),
			),
		}
		excluded := makeTransaction(
			"transaction-excluded-"+fake.UUID().V4(),
			visibleFirst.ID,
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			domain.TransactionKindRegular,
			-1,
			now,
		)
		excluded.TransferMatchingExcluded = true
		ineligible = append(ineligible, excluded)
		paired := makeTransaction(
			"transaction-paired-"+fake.UUID().V4(),
			visibleFirst.ID,
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			domain.TransactionKindTransfer,
			-1,
			now,
		)
		groupID := "group-" + fake.UUID().V4()
		paired.TransferGroupID = &groupID
		paired.TransferMatchedAt = &now
		ineligible = append(ineligible, paired)
		hidden := makeTransaction(
			"transaction-hidden-"+fake.UUID().V4(),
			visibleFirst.ID,
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			domain.TransactionKindRegular,
			-1,
			now,
		)
		hidden.HiddenAt = &hiddenAt
		ineligible = append(ineligible, hidden)
		otherTenant := makeTransaction(
			"transaction-other-"+fake.UUID().V4(),
			otherAccount.ID,
			domain.TransactionSourceManual,
			domain.TransactionStatusBooked,
			domain.TransactionKindRegular,
			-1,
			now,
		)
		otherTenant.TenantID = otherTenantID
		ineligible = append(ineligible, otherTenant)
		for _, transaction := range append(eligible, ineligible...) {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}

		actual, err := store.ListEligibleTransferMatchingTransactions(
			t.Context(),
			ListEligibleTransferMatchingTransactionsParams{
				TenantID: tenantID, RangeStart: rangeStart, RangeEndExclusive: rangeEndExclusive,
			},
		)
		require.NoError(t, err)
		expected := make(map[string]TransferMatchingTransaction, len(eligible))
		for _, transaction := range eligible {
			expected[transaction.ID] = TransferMatchingTransaction{
				ID: transaction.ID, AccountID: transaction.AccountID, Currency: transaction.Currency,
				AmountMinor: transaction.AmountMinor, EffectiveAt: transaction.EffectiveAt,
			}
		}
		actualByID := make(map[string]TransferMatchingTransaction, len(actual))
		for _, transaction := range actual {
			actualByID[transaction.ID] = transaction
		}
		require.Len(t, actualByID, len(expected))
		for transactionID, expectedTransaction := range expected {
			actualTransaction, found := actualByID[transactionID]
			require.True(t, found)
			assert.Equal(t, expectedTransaction.ID, actualTransaction.ID)
			assert.Equal(t, expectedTransaction.AccountID, actualTransaction.AccountID)
			assert.Equal(t, expectedTransaction.Currency, actualTransaction.Currency)
			assert.Equal(t, expectedTransaction.AmountMinor, actualTransaction.AmountMinor)
			assert.Equal(t, expectedTransaction.EffectiveAt.UnixNano(), actualTransaction.EffectiveAt.UnixNano())
		}
	})

	t.Run("loads descriptions and only unambiguous connection provenance", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.FixedZone("matching", -3*60*60))
		_, coreStore, transactions, store := makeStores(t)
		tenantID := "tenant-" + fake.UUID().V4()
		accounts := []domain.Account{
			{
				ID: "account-first-" + fake.UUID().V4(), TenantID: tenantID,
				Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
				CreatedAt: now, UpdatedAt: now,
			},
			{
				ID: "account-second-" + fake.UUID().V4(), TenantID: tenantID,
				Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindLinked,
				CreatedAt: now, UpdatedAt: now,
			},
		}
		for _, account := range accounts {
			_, err := coreStore.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}
		makeTransaction := func(id string, accountID string, status domain.TransactionStatus) domain.Transaction {
			return domain.Transaction{
				ID: id, TenantID: tenantID, AccountID: accountID, Source: domain.TransactionSourceProvider,
				Status: status, Kind: domain.TransactionKindRegular, AmountMinor: -int64(fake.IntBetween(1, 10_000)),
				Currency: "USD", Description: "description-" + fake.Lorem().Word(), EffectiveAt: now,
				CreatedAt: now, UpdatedAt: now,
			}
		}
		oneMapping := makeTransaction(
			"transaction-one-"+fake.UUID().V4(), accounts[0].ID, domain.TransactionStatusBooked,
		)
		noMapping := makeTransaction(
			"transaction-none-"+fake.UUID().V4(), accounts[1].ID, domain.TransactionStatusBooked,
		)
		repeatedMapping := makeTransaction(
			"transaction-repeated-"+fake.UUID().V4(), accounts[0].ID, domain.TransactionStatusBooked,
		)
		conflictingMapping := makeTransaction(
			"transaction-conflicting-"+fake.UUID().V4(), accounts[1].ID, domain.TransactionStatusBooked,
		)
		ineligible := makeTransaction(
			"transaction-pending-"+fake.UUID().V4(), accounts[0].ID, domain.TransactionStatusPending,
		)
		for _, transaction := range []domain.Transaction{oneMapping, noMapping, repeatedMapping, conflictingMapping, ineligible} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		connectionOneID := "connection-one-" + fake.UUID().V4()
		connectionTwoID := "connection-two-" + fake.UUID().V4()
		makeMatch := func(transactionID string, connectionID string) domain.ProviderTransactionMatch {
			return domain.ProviderTransactionMatch{
				ID: "match-" + fake.UUID().V4(), ConnectionID: connectionID,
				ProviderAccountID:     "provider-account-" + fake.UUID().V4(),
				ProviderTransactionID: "provider-transaction-" + fake.UUID().V4(),
				Fingerprint:           "fingerprint-" + fake.UUID().V4(), TransactionID: transactionID,
				Status: domain.TransactionStatusBooked, CreatedAt: now, UpdatedAt: now,
			}
		}
		for _, match := range []domain.ProviderTransactionMatch{
			makeMatch(oneMapping.ID, connectionOneID),
			makeMatch(repeatedMapping.ID, connectionOneID),
			makeMatch(repeatedMapping.ID, connectionOneID),
			makeMatch(conflictingMapping.ID, connectionOneID),
			makeMatch(conflictingMapping.ID, connectionTwoID),
		} {
			_, err := coreStore.SaveProviderTransactionMatch(t.Context(), match)
			require.NoError(t, err)
		}

		actual, err := store.ListEligibleTransferMatchingTransactions(
			t.Context(),
			ListEligibleTransferMatchingTransactionsParams{
				TenantID: tenantID, RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
			},
		)

		require.NoError(t, err)
		expected := map[string]TransferMatchingTransaction{
			oneMapping.ID: {
				ID:           oneMapping.ID,
				AccountID:    oneMapping.AccountID,
				Currency:     oneMapping.Currency,
				AmountMinor:  oneMapping.AmountMinor,
				EffectiveAt:  oneMapping.EffectiveAt,
				Description:  oneMapping.Description,
				ConnectionID: &connectionOneID,
			},
			noMapping.ID: {
				ID:          noMapping.ID,
				AccountID:   noMapping.AccountID,
				Currency:    noMapping.Currency,
				AmountMinor: noMapping.AmountMinor,
				EffectiveAt: noMapping.EffectiveAt,
				Description: noMapping.Description,
			},
			repeatedMapping.ID: {
				ID:           repeatedMapping.ID,
				AccountID:    repeatedMapping.AccountID,
				Currency:     repeatedMapping.Currency,
				AmountMinor:  repeatedMapping.AmountMinor,
				EffectiveAt:  repeatedMapping.EffectiveAt,
				Description:  repeatedMapping.Description,
				ConnectionID: &connectionOneID,
			},
			conflictingMapping.ID: {
				ID:          conflictingMapping.ID,
				AccountID:   conflictingMapping.AccountID,
				Currency:    conflictingMapping.Currency,
				AmountMinor: conflictingMapping.AmountMinor,
				EffectiveAt: conflictingMapping.EffectiveAt,
				Description: conflictingMapping.Description,
			},
		}
		require.Len(t, actual, len(expected))
		actualByID := make(map[string]TransferMatchingTransaction, len(actual))
		for _, transaction := range actual {
			actualByID[transaction.ID] = transaction
		}
		require.Len(t, actualByID, len(expected))
		for transactionID, expectedTransaction := range expected {
			actualTransaction, found := actualByID[transactionID]
			require.True(t, found)
			assert.Equal(t, expectedTransaction.ID, actualTransaction.ID)
			assert.Equal(t, expectedTransaction.AccountID, actualTransaction.AccountID)
			assert.Equal(t, expectedTransaction.Currency, actualTransaction.Currency)
			assert.Equal(t, expectedTransaction.AmountMinor, actualTransaction.AmountMinor)
			assert.Equal(t, expectedTransaction.Description, actualTransaction.Description)
			assert.Equal(t, expectedTransaction.ConnectionID, actualTransaction.ConnectionID)
			assert.Equal(t, expectedTransaction.EffectiveAt.UnixNano(), actualTransaction.EffectiveAt.UnixNano())
		}
	})
}
