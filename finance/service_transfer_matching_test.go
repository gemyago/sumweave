package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransferMatchingService(t *testing.T) {
	makeRow := func(id string, accountID string, amount int64, effectiveAt time.Time) persistence.TransferMatchingTransaction {
		return persistence.TransferMatchingTransaction{
			ID: id, AccountID: accountID, Currency: "USD", AmountMinor: amount, EffectiveAt: effectiveAt,
		}
	}
	makeFXRow := func(
		id string,
		accountID string,
		currency string,
		amount int64,
		description string,
		effectiveAt time.Time,
		connectionID *string,
	) persistence.TransferMatchingTransaction {
		return persistence.TransferMatchingTransaction{
			ID: id, AccountID: accountID, Currency: currency, AmountMinor: amount, Description: description,
			EffectiveAt: effectiveAt, ConnectionID: connectionID,
		}
	}
	makeMonobankRow := func(
		id string,
		accountID string,
		currency string,
		amount int64,
		operationAmount int64,
		operationCurrencyCode int,
		effectiveAt time.Time,
		connectionID *string,
	) persistence.TransferMatchingTransaction {
		connectorID := "monobank"
		snapshot := fmt.Sprintf(
			`{"amount":%d,"operationAmount":%d,"currencyCode":%d,"mcc":4829}`,
			amount,
			operationAmount,
			operationCurrencyCode,
		)
		return persistence.TransferMatchingTransaction{
			ID: id, AccountID: accountID, Currency: currency, AmountMinor: amount, EffectiveAt: effectiveAt,
			ConnectionID: connectionID, ConnectorID: &connectorID, ProviderOriginalAmountMinor: &amount,
			ProviderOriginalCurrency: &currency, SnapshotJSON: &snapshot,
		}
	}
	makeService := func(t *testing.T, pairs transferMatchingPairStore) *TransferMatchingService {
		t.Helper()
		service, err := NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: newMockaccessGuardStore(t), Pairs: pairs, Logger: slog.New(slog.DiscardHandler),
			Now: func() time.Time {
				return time.Date(2026, time.September, 7, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
			},
			NewID: func() string { return "group-test" },
		})
		require.NoError(t, err)
		return service
	}

	t.Run("matches deterministic mutually unique pairs from one immutable load", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := TransferMatchingParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now, RangeEndExclusive: now.Add(3 * time.Hour),
			MessageID: "message-" + fake.UUID().V4(), SourceSyncMessageID: "sync-" + fake.UUID().V4(),
		}
		rows := []persistence.TransferMatchingTransaction{
			makeRow("accepted-first", "account-first", -100, now),
			makeRow("accepted-second", "account-second", 100, now.Add(72*time.Hour)),
			makeRow("unmatched", "account-unmatched", -200, now),
			makeRow("minimum", "account-minimum", math.MinInt64, now.Add(time.Minute)),
			makeRow("ambiguous-first", "account-ambiguous-first", -300, now.Add(2*time.Minute)),
			makeRow("ambiguous-second", "account-ambiguous-second", 300, now.Add(3*time.Minute)),
			makeRow("ambiguous-third", "account-ambiguous-third", 300, now.Add(4*time.Minute)),
			makeRow("outside-first", "account-outside-first", -400, now.Add(-time.Hour)),
			makeRow("outside-second", "account-outside-second", 400, now.Add(-30*time.Minute)),
		}
		slices.Reverse(rows)
		pairs := newMocktransferMatchingPairStore(t)
		pairs.EXPECT().
			ListEligibleTransferMatchingTransactions(t.Context(), persistence.ListEligibleTransferMatchingTransactionsParams{
				TenantID: params.TenantID, RangeStart: params.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
			}).
			Return(rows, nil).
			Once()
		var saved persistence.TransferPairLinkParams
		pairs.EXPECT().
			LinkTransferPair(t.Context(), mock.Anything).
			Run(func(_ context.Context, value persistence.TransferPairLinkParams) {
				saved = value
			}).
			Return(nil).
			Once()

		counts, err := makeService(t, pairs).Match(t.Context(), params)

		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1, Unmatched: 2, Ambiguous: 3}, counts)
		assert.Equal(t, params.TenantID, saved.TenantID)
		assert.Equal(t, "accepted-first", saved.FirstTransactionID)
		assert.Equal(t, "accepted-second", saved.SecondTransactionID)
		assert.Equal(t, "group-test", saved.TransferGroupID)
		assert.True(t, now.Equal(saved.TransferMatchedAt))
		assert.True(t, now.Equal(saved.UpdatedAt))
	})

	t.Run("combines same-currency and FX candidates before mutual uniqueness", func(t *testing.T) {
		fake := faker.New()
		stringPointer := func(value string) *string { return &value }
		now := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
		connectionID := "connection-" + fake.UUID().V4()
		params := TransferMatchingParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		}
		baseDescription := "FX123 USD/PLN 4.125"
		quoteDescription := "FX123 USD/PLN 4,12500"
		base := makeFXRow(
			"base-"+fake.UUID().V4(), "account-base-"+fake.UUID().V4(), "USD", -10_000,
			baseDescription, now, &connectionID,
		)
		quote := makeFXRow(
			"quote-"+fake.UUID().V4(), "account-quote-"+fake.UUID().V4(), "PLN", 41_250,
			quoteDescription, now.Add(time.Hour), &connectionID,
		)

		t.Run("matches the illustrated USD PLN exchange", func(t *testing.T) {
			pairs := newMocktransferMatchingPairStore(t)
			pairs.EXPECT().
				ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).
				Return([]persistence.TransferMatchingTransaction{base, quote}, nil).
				Once()
			pairs.EXPECT().
				LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
					return value.FirstTransactionID == base.ID && value.SecondTransactionID == quote.ID
				})).
				Return(nil).
				Once()

			counts, err := makeService(t, pairs).Match(t.Context(), params)

			require.NoError(t, err)
			assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
		})

		t.Run("rejects mismatched FX evidence and unavailable provenance", func(t *testing.T) {
			cases := []struct {
				name   string
				second persistence.TransferMatchingTransaction
			}{
				{
					name: "connection scope",
					second: makeFXRow(
						"quote-connection-"+fake.UUID().V4(),
						"account-quote-connection-"+fake.UUID().V4(), "PLN", 41_250,
						quoteDescription, now.Add(time.Hour), stringPointer("connection-"+fake.UUID().V4()),
					),
				},
				{
					name: "reference",
					second: makeFXRow(
						"quote-reference-"+fake.UUID().V4(),
						"account-quote-reference-"+fake.UUID().V4(), "PLN", 41_250,
						"FX456 USD/PLN 4.125", now.Add(time.Hour), &connectionID,
					),
				},
				{
					name: "rate",
					second: makeFXRow(
						"quote-rate-"+fake.UUID().V4(),
						"account-quote-rate-"+fake.UUID().V4(), "PLN", 41_250,
						"FX123 USD/PLN 4.126", now.Add(time.Hour), &connectionID,
					),
				},
				{
					name: "ordered currency pair",
					second: makeFXRow(
						"quote-pair-"+fake.UUID().V4(),
						"account-quote-pair-"+fake.UUID().V4(), "PLN", 41_250,
						"FX123 PLN/USD 4.125", now.Add(time.Hour), &connectionID,
					),
				},
				{
					name: "converted value",
					second: makeFXRow(
						"quote-value-"+fake.UUID().V4(),
						"account-quote-value-"+fake.UUID().V4(), "PLN", 41_249,
						quoteDescription, now.Add(time.Hour), &connectionID,
					),
				},
				{
					name: "missing provenance",
					second: makeFXRow(
						"quote-missing-"+fake.UUID().V4(),
						"account-quote-missing-"+fake.UUID().V4(), "PLN", 41_250,
						quoteDescription, now.Add(time.Hour), nil,
					),
				},
				{
					name: "conflicting provenance",
					second: makeFXRow(
						"quote-conflicting-"+fake.UUID().V4(),
						"account-quote-conflicting-"+fake.UUID().V4(), "PLN", 41_250,
						quoteDescription, now.Add(time.Hour), nil,
					),
				},
			}
			for _, testCase := range cases {
				t.Run(testCase.name, func(t *testing.T) {
					pairs := newMocktransferMatchingPairStore(t)
					pairs.EXPECT().
						ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).
						Return([]persistence.TransferMatchingTransaction{base, testCase.second}, nil).
						Once()

					counts, err := makeService(t, pairs).Match(t.Context(), params)

					require.NoError(t, err)
					assert.Equal(t, TransferMatchingAttemptCounts{Unmatched: 1}, counts)
				})
			}
		})

		t.Run("keeps candidates ambiguous across both rules and outside the requested range", func(t *testing.T) {
			sameCurrency := makeFXRow(
				"same-currency-"+fake.UUID().V4(), "account-same-currency-"+fake.UUID().V4(), "USD", 10_000,
				"unrelated", now.Add(time.Minute), &connectionID,
			)
			pairs := newMocktransferMatchingPairStore(t)
			pairs.EXPECT().
				ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).
				Return([]persistence.TransferMatchingTransaction{base, quote, sameCurrency}, nil).
				Once()

			counts, err := makeService(t, pairs).Match(t.Context(), params)

			require.NoError(t, err)
			assert.Equal(t, TransferMatchingAttemptCounts{Ambiguous: 2}, counts)

			outside := makeFXRow(
				"outside-"+fake.UUID().V4(), "account-outside-"+fake.UUID().V4(), "USD", -10_000,
				baseDescription, now.Add(73*time.Hour), &connectionID,
			)
			pairs = newMocktransferMatchingPairStore(t)
			pairs.EXPECT().
				ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).
				Return([]persistence.TransferMatchingTransaction{base, quote, outside}, nil).
				Once()

			counts, err = makeService(t, pairs).Match(t.Context(), params)

			require.NoError(t, err)
			assert.Equal(t, TransferMatchingAttemptCounts{Ambiguous: 1}, counts)
		})

		t.Run("keeps FX decisions stable, range-scoped, and extracts evidence once per row", func(t *testing.T) {
			rows := []persistence.TransferMatchingTransaction{base, quote}
			forward, forwardCounts, err := decideTransferMatchingPairs(
				t.Context(), rows, params.RangeStart, params.RangeEndExclusive,
			)
			require.NoError(t, err)
			require.Equal(t, TransferMatchingAttemptCounts{}, forwardCounts)
			slices.Reverse(rows)
			shuffled, shuffledCounts, err := decideTransferMatchingPairs(
				t.Context(), rows, params.RangeStart, params.RangeEndExclusive,
			)
			require.NoError(t, err)
			assert.Equal(t, forward, shuffled)
			assert.Equal(t, forwardCounts, shuffledCounts)

			outsideRange, outsideCounts, err := decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{base, quote},
				now.Add(2*time.Hour),
				now.Add(3*time.Hour),
			)
			require.NoError(t, err)
			assert.Empty(t, outsideRange)
			assert.Equal(t, TransferMatchingAttemptCounts{}, outsideCounts)

			extractions := 0
			transferMatchingIndex(
				rows,
				params.RangeStart,
				params.RangeEndExclusive,
				func(description string) (fxEvidence, bool) {
					extractions++
					return extractFXEvidence(description)
				},
			)
			assert.Equal(t, len(rows), extractions)
		})
	})

	t.Run("matches only reciprocal Monobank snapshots", func(t *testing.T) {
		fake := faker.New()
		stringPointer := func(value string) *string { return &value }
		now := time.Date(2026, time.September, 11, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
		connectionID := "connection-" + fake.UUID().V4()
		params := TransferMatchingParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		}
		first := makeMonobankRow(
			"transaction-uah-"+fake.UUID().V4(), "account-uah-"+fake.UUID().V4(), "UAH", 508_300, 10_000, 978,
			now, &connectionID,
		)
		second := makeMonobankRow(
			"transaction-eur-"+fake.UUID().V4(), "account-eur-"+fake.UUID().V4(), "EUR", -10_000, -508_300, 980,
			now, &connectionID,
		)

		t.Run("accepts either input order and persists the illustrated pair atomically", func(t *testing.T) {
			forward, forwardCounts, err := decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{first, second},
				params.RangeStart,
				params.RangeEndExclusive,
			)
			require.NoError(t, err)
			require.Equal(t, TransferMatchingAttemptCounts{}, forwardCounts)
			require.Len(t, forward, 1)
			reversed, reversedCounts, err := decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{second, first},
				params.RangeStart,
				params.RangeEndExclusive,
			)
			require.NoError(t, err)
			assert.Equal(t, forward, reversed)
			assert.Equal(t, forwardCounts, reversedCounts)

			pairs := newMocktransferMatchingPairStore(t)
			pairs.EXPECT().
				ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).
				Return([]persistence.TransferMatchingTransaction{second, first}, nil).
				Once()
			pairs.EXPECT().
				LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
					return value.TenantID == params.TenantID &&
						((value.FirstTransactionID == first.ID && value.SecondTransactionID == second.ID) ||
							(value.FirstTransactionID == second.ID && value.SecondTransactionID == first.ID))
				})).
				Return(nil).
				Once()
			counts, err := makeService(t, pairs).Match(t.Context(), params)
			require.NoError(t, err)
			assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
		})

		t.Run("finds the reciprocal pair when either leg starts the requested range", func(t *testing.T) {
			secondOutsideRange := second
			secondOutsideRange.EffectiveAt = now.Add(time.Hour)
			pairs, counts, err := decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{first, secondOutsideRange},
				now,
				now.Add(time.Minute),
			)
			require.NoError(t, err)
			assert.Len(t, pairs, 1)
			assert.Equal(t, TransferMatchingAttemptCounts{}, counts)
			firstOutsideRange := first
			firstOutsideRange.EffectiveAt = now.Add(-time.Hour)
			pairs, counts, err = decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{firstOutsideRange, second},
				now,
				now.Add(time.Minute),
			)
			require.NoError(t, err)
			assert.Len(t, pairs, 1)
			assert.Equal(t, TransferMatchingAttemptCounts{}, counts)
		})

		t.Run("rejects incomplete or incompatible Monobank candidates", func(t *testing.T) {
			outside := second
			outside.EffectiveAt = now.Add(transferMatchingWindow + time.Nanosecond)
			sameAccount := second
			sameAccount.AccountID = first.AccountID
			differentConnection := second
			differentConnection.ConnectionID = stringPointer("connection-" + fake.UUID().V4())
			nonreciprocal := second
			snapshot := `{"amount":-10000,"operationAmount":-508299,"currencyCode":980,"mcc":4829}`
			nonreciprocal.SnapshotJSON = &snapshot
			oneSided := second
			oneSided.SnapshotJSON = nil
			sameSign := second
			sameSignSnapshot := `{"amount":-10000,"operationAmount":508300,"currencyCode":980,"mcc":4829}`
			sameSign.SnapshotJSON = &sameSignSnapshot
			for _, testCase := range []struct {
				name   string
				second persistence.TransferMatchingTransaction
			}{
				{name: "one sided", second: oneSided},
				{name: "nonreciprocal", second: nonreciprocal},
				{name: "same sign", second: sameSign},
				{name: "different connection", second: differentConnection},
				{name: "same account", second: sameAccount},
				{name: "outside window", second: outside},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					pairs, _, err := decideTransferMatchingPairs(
						t.Context(),
						[]persistence.TransferMatchingTransaction{first, testCase.second},
						params.RangeStart,
						params.RangeEndExclusive,
					)
					require.NoError(t, err)
					assert.Empty(t, pairs)
				})
			}
		})

		t.Run("keeps two reciprocal snapshots and cross-rule candidates ambiguous", func(t *testing.T) {
			secondCopy := second
			secondCopy.ID = "transaction-eur-copy-" + fake.UUID().V4()
			secondCopy.AccountID = "account-eur-copy-" + fake.UUID().V4()
			pairs, _, err := decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{first, second, secondCopy},
				params.RangeStart,
				params.RangeEndExclusive,
			)
			require.NoError(t, err)
			assert.Empty(t, pairs)

			sameCurrency := persistence.TransferMatchingTransaction{
				ID: "transaction-uah-same-" + fake.UUID().V4(), AccountID: "account-uah-same-" + fake.UUID().V4(),
				Currency: "UAH", AmountMinor: -508_300, EffectiveAt: now,
			}
			pairs, _, err = decideTransferMatchingPairs(
				t.Context(),
				[]persistence.TransferMatchingTransaction{first, second, sameCurrency},
				params.RangeStart,
				params.RangeEndExclusive,
			)
			require.NoError(t, err)
			assert.Empty(t, pairs)
		})
	})

	t.Run("rejects invalid ranges before loading and requires constructor dependencies", func(t *testing.T) {
		fake := faker.New()
		pairs := newMocktransferMatchingPairStore(t)
		service := makeService(t, pairs)
		invalidAt := time.Now()
		invalid := []TransferMatchingParams{
			{TenantID: "tenant-" + fake.UUID().V4()},
			{TenantID: "tenant-" + fake.UUID().V4(), RangeStart: invalidAt, RangeEndExclusive: invalidAt},
		}
		for _, params := range invalid {
			_, err := service.Match(t.Context(), params)
			require.ErrorIs(t, err, ErrInvalidTimestampRange)
		}
		_, err := NewTransferMatchingService(TransferMatchingServiceArgs{})
		require.ErrorContains(t, err, "access store is required")
		access := newMockaccessGuardStore(t)
		_, err = NewTransferMatchingService(TransferMatchingServiceArgs{Access: access})
		require.ErrorContains(t, err, "pair store is required")
		_, err = NewTransferMatchingService(TransferMatchingServiceArgs{Access: access, Pairs: pairs})
		require.ErrorContains(t, err, "logger is required")
		_, err = NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: access, Pairs: pairs, Logger: slog.New(slog.DiscardHandler),
		})
		require.ErrorContains(t, err, "clock is required")
		_, err = NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: access, Pairs: pairs, Logger: slog.New(slog.DiscardHandler), Now: time.Now,
		})
		require.ErrorContains(t, err, "ID generator is required")
	})

	t.Run("skips missing pairs, retains earlier commits, and reloads on retry", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 15, 0, 0, 0, time.FixedZone("test", -3*60*60))
		params := TransferMatchingParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		}
		rows := []persistence.TransferMatchingTransaction{
			makeRow("first-a", "account-first-a", -100, now),
			makeRow("first-b", "account-first-b", 100, now.Add(time.Minute)),
			makeRow("second-a", "account-second-a", -200, now),
			makeRow("second-b", "account-second-b", 200, now.Add(time.Minute)),
			makeRow("third-a", "account-third-a", -300, now),
			makeRow("third-b", "account-third-b", 300, now.Add(time.Minute)),
		}
		pairs := newMocktransferMatchingPairStore(t)
		pairs.EXPECT().ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).Return(rows, nil).Once()
		pairs.EXPECT().
			LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
				return value.FirstTransactionID == "first-a" && value.SecondTransactionID == "first-b"
			})).
			Return(persistence.ErrTransferPairTransactionNotFound).
			Once()
		pairs.EXPECT().
			LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
				return value.FirstTransactionID == "second-a" && value.SecondTransactionID == "second-b"
			})).
			Return(nil).
			Once()
		writeErr := errors.New("later write failed")
		pairs.EXPECT().
			LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
				return value.FirstTransactionID == "third-a" && value.SecondTransactionID == "third-b"
			})).
			Return(writeErr).
			Once()
		service := makeService(t, pairs)
		counts, err := service.Match(t.Context(), params)
		require.ErrorIs(t, err, writeErr)
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1, Skipped: 1}, counts)

		pairs.EXPECT().ListEligibleTransferMatchingTransactions(t.Context(), mock.Anything).Return(nil, nil).Once()
		counts, err = service.Match(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{}, counts)
	})

	t.Run("retains an earlier PostgreSQL pair when a later pair save fails", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 16, 0, 0, 0, time.FixedZone("test", -3*60*60))
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		transactions := persistence.NewTransactionTagStore(database)
		pairStore := persistence.NewTransferPairStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		makeAccount := func() domain.Account {
			return domain.Account{
				ID: "account-" + fake.UUID().V4(), TenantID: tenantID,
				Name: "account-" + fake.Lorem().Word(), Currency: "USD", Kind: domain.AccountKindManual,
				CreatedAt: now, UpdatedAt: now,
			}
		}
		makeTransaction := func(accountID string, amount int64, description string) domain.Transaction {
			return domain.Transaction{
				ID: description + "-" + fake.UUID().V4(), TenantID: tenantID, AccountID: accountID,
				Source: domain.TransactionSourceManual, Status: domain.TransactionStatusBooked,
				Kind: domain.TransactionKindRegular, AmountMinor: amount, Currency: "USD",
				Description: description, EffectiveAt: now, CreatedAt: now, UpdatedAt: now,
			}
		}
		accounts := []domain.Account{makeAccount(), makeAccount(), makeAccount(), makeAccount()}
		for _, account := range accounts {
			_, err := store.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}
		committedFirst := makeTransaction(accounts[0].ID, -100, "committed-first")
		committedSecond := makeTransaction(accounts[1].ID, 100, "committed-second")
		failingFirst := makeTransaction(accounts[2].ID, -200, "failing-first")
		failingSecond := makeTransaction(accounts[3].ID, 200, "failing-second")
		for _, transaction := range []domain.Transaction{committedFirst, committedSecond, failingFirst, failingSecond} {
			_, err := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, err)
		}
		committedGroupID := "group-" + fake.UUID().V4()
		generatedIDs := []string{committedGroupID, "\x00"}
		matchingService, err := NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: store,
			Pairs:  pairStore,
			Logger: slog.New(slog.DiscardHandler),
			Now:    func() time.Time { return now },
			NewID: func() string {
				id := generatedIDs[0]
				generatedIDs = generatedIDs[1:]
				return id
			},
		})
		require.NoError(t, err)

		counts, err := matchingService.Match(t.Context(), TransferMatchingParams{
			TenantID: tenantID, RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		})

		require.Error(t, err)
		require.ErrorContains(t, err, "invalid byte sequence")
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
		for _, transactionID := range []string{committedFirst.ID, committedSecond.ID} {
			stored, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.TransactionKindTransfer, stored.Kind)
			require.NotNil(t, stored.TransferGroupID)
			assert.Equal(t, committedGroupID, *stored.TransferGroupID)
			require.NotNil(t, stored.TransferMatchedAt)
		}
		for _, transactionID := range []string{failingFirst.ID, failingSecond.ID} {
			stored, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.TransactionKindRegular, stored.Kind)
			assert.Nil(t, stored.TransferGroupID)
			assert.Nil(t, stored.TransferMatchedAt)
		}
	})

	t.Run("keeps outer hour-boundary evidence ambiguous and uses elapsed DST time", func(t *testing.T) {
		fake := faker.New()
		hourZero := time.Date(2026, time.March, 6, 12, 0, 0, 0, time.FixedZone("test", -5*60*60))
		boundaryParams := TransferMatchingParams{
			TenantID:   "tenant-" + fake.UUID().V4(),
			RangeStart: hourZero.Add(72 * time.Hour), RangeEndExclusive: hourZero.Add(73 * time.Hour),
		}
		boundaryRows := []persistence.TransferMatchingTransaction{
			makeRow("left-start", "account-left-start", -100, hourZero.Add(72*time.Hour)),
			makeRow("left-hour-zero", "account-left-hour-zero", 100, hourZero),
			makeRow("left-hour-144", "account-left-hour-144", 100, hourZero.Add(144*time.Hour)),
			makeRow("right-start", "account-right-start", 200, hourZero.Add(72*time.Hour)),
			makeRow("right-hour-zero", "account-right-hour-zero", -200, hourZero),
			makeRow("right-hour-144", "account-right-hour-144", -200, hourZero.Add(144*time.Hour)),
		}
		pairs := newMocktransferMatchingPairStore(t)
		pairs.EXPECT().
			ListEligibleTransferMatchingTransactions(t.Context(), persistence.ListEligibleTransferMatchingTransactionsParams{
				TenantID:   boundaryParams.TenantID,
				RangeStart: boundaryParams.RangeStart, RangeEndExclusive: boundaryParams.RangeEndExclusive,
			}).
			Return(boundaryRows, nil).
			Once()

		counts, err := makeService(t, pairs).Match(t.Context(), boundaryParams)

		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{Ambiguous: 2}, counts)

		dstStart, err := time.Parse(time.RFC3339, "2026-03-07T12:00:00-05:00")
		require.NoError(t, err)
		dstPartner, err := time.Parse(time.RFC3339, "2026-03-10T13:00:00-04:00")
		require.NoError(t, err)
		dstParams := TransferMatchingParams{
			TenantID:   "tenant-" + fake.UUID().V4(),
			RangeStart: dstStart, RangeEndExclusive: dstStart.Add(time.Hour),
		}
		pairs.EXPECT().
			ListEligibleTransferMatchingTransactions(t.Context(), persistence.ListEligibleTransferMatchingTransactionsParams{
				TenantID:          dstParams.TenantID,
				RangeStart:        dstParams.RangeStart,
				RangeEndExclusive: dstParams.RangeEndExclusive,
			}).
			Return([]persistence.TransferMatchingTransaction{
				makeRow("dst-start", "account-dst-start", -500, dstStart),
				makeRow("dst-partner", "account-dst-partner", 500, dstPartner),
			}, nil).
			Once()
		pairs.EXPECT().
			LinkTransferPair(t.Context(), mock.MatchedBy(func(value persistence.TransferPairLinkParams) bool {
				return value.FirstTransactionID == "dst-partner" && value.SecondTransactionID == "dst-start"
			})).
			Return(nil).
			Once()

		counts, err = makeService(t, pairs).Match(t.Context(), dstParams)

		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
	})

	t.Run("does not evaluate or save after cancellation", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 17, 0, 0, 0, time.UTC)
		pairs := newMocktransferMatchingPairStore(t)
		service := makeService(t, pairs)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := service.Match(ctx, TransferMatchingParams{
			TenantID: "tenant-" + fake.UUID().V4(), RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("submits only through an optional finance-owned publication port", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 19, 0, 0, 0, time.FixedZone("test", 2*60*60))
		params := TransferMatchingSubmission{
			ActorUserID: "user-" + fake.UUID().V4(), TenantID: "tenant-" + fake.UUID().V4(),
			RangeStart: now, RangeEndExclusive: now.Add(time.Hour),
		}
		access := newMockaccessGuardStore(t)
		pairs := newMocktransferMatchingPairStore(t)
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		service, err := NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: access, Pairs: pairs, Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
			NewID: func() string { return "group-" + fake.UUID().V4() },
		})
		require.NoError(t, err)
		_, err = service.Submit(t.Context(), params)
		require.ErrorContains(t, err, "submission publisher is required")

		publisher := NewMockSemanticCommandPublisher(t)
		reference := TransferMatchingJobRef{ID: "message-" + fake.UUID().V4()}
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		publisher.EXPECT().
			PublishSemanticCommand(t.Context(), mock.Anything).
			Return(DispatchReference{MessageID: reference.ID}, nil).
			Once()
		service, err = NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: access, Pairs: pairs, Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
			NewID: func() string { return "group-" + fake.UUID().V4() },
		}, WithTransferMatchingServiceSubmissionPublisher(publisher))
		require.NoError(t, err)
		actual, err := service.Submit(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, reference, actual)

		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(false, nil).Once()
		_, err = service.Submit(t.Context(), params)
		require.ErrorIs(t, err, ErrTenantAccessDenied)

		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		invalid := params
		invalid.RangeEndExclusive = invalid.RangeStart
		_, err = service.Submit(t.Context(), invalid)
		require.ErrorIs(t, err, ErrInvalidTimestampRange)

		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		invalid = params
		invalid.RangeStart = time.Time{}
		_, err = service.Submit(t.Context(), invalid)
		require.ErrorIs(t, err, ErrInvalidTimestampRange)
		_, err = service.Match(t.Context(), TransferMatchingParams{
			TenantID: params.TenantID, RangeStart: invalid.RangeStart, RangeEndExclusive: params.RangeEndExclusive,
		})
		require.ErrorIs(t, err, ErrInvalidTimestampRange)
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, _, err = decideTransferMatchingPairs(canceled, []persistence.TransferMatchingTransaction{{
			ID: fake.UUID().V4(), AmountMinor: 1, Currency: "USD", EffectiveAt: params.RangeStart,
		}}, params.RangeStart, params.RangeEndExclusive)
		require.ErrorIs(t, err, context.Canceled)

		publishErr := errors.New("publication failed")
		access.EXPECT().IsTenantMember(t.Context(), params.TenantID, params.ActorUserID).Return(true, nil).Once()
		publisher.EXPECT().
			PublishSemanticCommand(t.Context(), mock.Anything).
			Return(DispatchReference{}, publishErr).
			Once()
		_, err = service.Submit(t.Context(), params)
		require.ErrorIs(t, err, publishErr)
	})

	t.Run("preserves classification, tags, balances, reporting, and fresh pending eligibility", func(t *testing.T) {
		fake := faker.New()
		now := time.Date(2026, time.September, 7, 20, 0, 0, 0, time.FixedZone("test", 2*60*60))
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		transactions := persistence.NewTransactionTagStore(database)
		pairStore := persistence.NewTransferPairStore(database)
		tenantID := "tenant-" + fake.UUID().V4()
		actorUserID := "user-" + fake.UUID().V4()
		require.NoError(t, func() error {
			_, err := store.SaveTenantMembership(t.Context(), domain.TenantMembership{
				TenantID: tenantID, UserID: actorUserID, JoinedAt: now,
			})
			return err
		}())
		accounts := []domain.Account{
			{
				ID:        "account-first-" + fake.UUID().V4(),
				TenantID:  tenantID,
				Name:      "account-" + fake.Lorem().Word(),
				Currency:  "USD",
				Kind:      domain.AccountKindManual,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "account-second-" + fake.UUID().V4(),
				TenantID:  tenantID,
				Name:      "account-" + fake.Lorem().Word(),
				Currency:  "USD",
				Kind:      domain.AccountKindManual,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		for _, account := range accounts {
			_, err := store.SaveAccount(t.Context(), account)
			require.NoError(t, err)
		}
		category := domain.Category{
			ID: "category-" + fake.UUID().V4(), TenantID: tenantID, Name: "category-" + fake.Lorem().Word(),
			Kind: domain.CategoryKindExpense, CreatedAt: now, UpdatedAt: now,
		}
		tag := domain.Tag{
			ID:        "tag-" + fake.UUID().V4(),
			TenantID:  tenantID,
			Name:      "tag-" + fake.Lorem().Word(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := store.SaveCategory(t.Context(), category)
		require.NoError(t, err)
		_, err = store.SaveTag(t.Context(), tag)
		require.NoError(t, err)
		makeTransaction := func(id string, accountID string, amount int64, description string, effectiveAt time.Time, status domain.TransactionStatus) domain.Transaction {
			return domain.Transaction{
				ID: id, TenantID: tenantID, AccountID: accountID, Source: domain.TransactionSourceManual,
				Status: status, Kind: domain.TransactionKindRegular, AmountMinor: amount, Currency: "USD",
				Description: description, EffectiveAt: effectiveAt, CategoryID: &category.ID, TagIDs: []string{tag.ID},
				CreatedAt: now, UpdatedAt: now,
			}
		}
		firstRangeStart := now.Add(-2 * time.Hour)
		first := makeTransaction(
			"transaction-first-"+fake.UUID().V4(),
			accounts[0].ID,
			-100,
			"already-matched",
			now,
			domain.TransactionStatusBooked,
		)
		second := makeTransaction(
			"transaction-second-"+fake.UUID().V4(),
			accounts[1].ID,
			100,
			"already-matched",
			now.Add(time.Hour),
			domain.TransactionStatusBooked,
		)
		pending := makeTransaction(
			"transaction-pending-"+fake.UUID().V4(),
			accounts[0].ID,
			-200,
			"classifiable",
			now.Add(3*time.Hour),
			domain.TransactionStatusPending,
		)
		bookedPartner := makeTransaction(
			"transaction-booked-"+fake.UUID().V4(),
			accounts[1].ID,
			200,
			"classifiable",
			now.Add(4*time.Hour),
			domain.TransactionStatusBooked,
		)
		pending.CategoryID = nil
		bookedPartner.CategoryID = nil
		excludedFromMatching := makeTransaction(
			"transaction-excluded-"+fake.UUID().V4(),
			accounts[0].ID,
			-33,
			"reportable",
			now.Add(9*time.Hour),
			domain.TransactionStatusBooked,
		)
		excludedFromMatching.TransferMatchingExcluded = true
		for _, transaction := range []domain.Transaction{first, second, pending, bookedPartner, excludedFromMatching} {
			_, saveErr := transactions.SaveTransaction(t.Context(), transaction)
			require.NoError(t, saveErr)
		}
		matchingService, err := NewTransferMatchingService(TransferMatchingServiceArgs{
			Access: store,
			Pairs:  pairStore,
			Logger: slog.New(slog.DiscardHandler),
			Now:    func() time.Time { return now },
			NewID:  func() string { return "group-" + fake.UUID().V4() },
		})
		require.NoError(t, err)
		counts, err := matchingService.Match(t.Context(), TransferMatchingParams{
			TenantID: tenantID, RangeStart: firstRangeStart, RangeEndExclusive: now.Add(2 * time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
		for _, transactionID := range []string{first.ID, second.ID} {
			stored, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			require.NotNil(t, stored.CategoryID)
			assert.Equal(t, category.ID, *stored.CategoryID)
			assert.Equal(t, []string{tag.ID}, stored.TagIDs)
			assert.Equal(t, domain.TransactionKindTransfer, stored.Kind)
		}

		ruleStore := persistence.NewClassificationRuleStore(database)
		_, err = ruleStore.AppendClassificationRule(t.Context(), domain.ClassificationRule{
			ID: "rule-" + fake.UUID().V4(), TenantID: tenantID, MatchType: domain.ClassificationMatchTypeExact,
			Condition: "classifiable", CategoryID: category.ID, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, err)
		classificationService, err := NewClassificationService(ClassificationServiceArgs{
			Access:       store,
			Rules:        ruleStore,
			Transactions: persistence.NewClassificationTransactionStore(database),
			Categories:   store,
			Tags:         store,
			Logger:       slog.New(slog.DiscardHandler),
			Now:          func() time.Time { return now },
		})
		require.NoError(t, err)
		skipCounts, err := classificationService.Classify(t.Context(), ClassificationParams{
			TenantID: tenantID, RangeStart: firstRangeStart, RangeEndExclusive: now.Add(2 * time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{}, skipCounts)
		classificationCounts, err := classificationService.Classify(t.Context(), ClassificationParams{
			TenantID: tenantID, RangeStart: now.Add(2 * time.Hour), RangeEndExclusive: now.Add(5 * time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, ClassificationAttemptCounts{Classified: 1}, classificationCounts)

		pending.Status = domain.TransactionStatusBooked
		_, err = transactions.SaveTransaction(t.Context(), pending)
		require.NoError(t, err)
		counts, err = matchingService.Match(t.Context(), TransferMatchingParams{
			TenantID: tenantID, RangeStart: now.Add(2 * time.Hour), RangeEndExclusive: now.Add(5 * time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, TransferMatchingAttemptCounts{MatchedPairs: 1}, counts)
		for _, transactionID := range []string{pending.ID, bookedPartner.ID} {
			stored, getErr := transactions.GetTransaction(t.Context(), transactionID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.TransactionKindTransfer, stored.Kind)
			assert.Equal(t, []string{tag.ID}, stored.TagIDs)
			if transactionID == bookedPartner.ID {
				require.NotNil(t, stored.CategoryID)
				assert.Equal(t, category.ID, *stored.CategoryID)
			} else {
				assert.Nil(t, stored.CategoryID)
			}
		}

		ledger := NewLedgerService(store)
		summary, err := ledger.SummarizeTransactions(
			t.Context(),
			SummarizeTransactionsParams{ActorUserID: actorUserID, TenantID: tenantID},
		)
		require.NoError(t, err)
		assert.Equal(t, domain.TransactionSummary{ExpenseMinor: 33, NetMinor: -33}, summary)
		firstBalance, err := ledger.GetAccountBalance(
			t.Context(),
			GetAccountBalanceParams{ActorUserID: actorUserID, TenantID: tenantID, AccountID: accounts[0].ID},
		)
		require.NoError(t, err)
		secondBalance, err := ledger.GetAccountBalance(
			t.Context(),
			GetAccountBalanceParams{ActorUserID: actorUserID, TenantID: tenantID, AccountID: accounts[1].ID},
		)
		require.NoError(t, err)
		assert.Equal(t, int64(-333), firstBalance.BookedBalanceMinor)
		assert.Equal(t, int64(300), secondBalance.BookedBalanceMinor)
	})
}
