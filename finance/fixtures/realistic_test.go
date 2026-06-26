package fixtures_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/gemyago/signal-foundry/finance/credentials"
	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/fixtures"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scenarioServiceSpy struct {
	createTenantCalls        int
	createTenantInviteCalls  int
	acceptTenantInviteCalls  int
	createAccountCalls       int
	listCategoriesCalls      int
	listTagsCalls            int
	previewCSVImportCalls    int
	recordTransactionCalls   int
	hideTransactionCalls     int
	linkTransfersCalls       int
	linkTokenConnectionCalls int
	upsertScheduleCalls      int
	syncFXRatesCalls         int
	createAccountFailAt      int
	recordTransactionFailAt  int
	createTenantErr          error
	inviteErr                error
	acceptInviteErr          error
	secondAccountErr         error
	listCategoriesErr        error
	listTagsErr              error
	previewCSVImportErr      error
	recordTransactionErr     error
	hideTransactionErr       error
	linkTransfersErr         error
	linkTokenErr             error
	upsertScheduleErr        error
	syncFXErr                error
	createdTenantID          string
	inviteCode               string
	checkingAccountID        string
	savingsAccountID         string
	importedAccountID        string
	reconciliationAccountID  string
	recordedTransactionID    string
	previewCSVImport         financepkg.CSVImportPreview
	previewCSVImportCSV      string
	categories               []domain.Category
	tags                     []domain.Tag
}

func (s *scenarioServiceSpy) CreateTenant(
	_ context.Context,
	_ financepkg.CreateTenantParams,
) (domain.Tenant, error) {
	s.createTenantCalls++
	if s.createTenantErr != nil {
		return domain.Tenant{}, s.createTenantErr
	}
	return domain.Tenant{ID: s.createdTenantID}, nil
}

func (s *scenarioServiceSpy) CreateTenantInvite(
	_ context.Context,
	_ financepkg.CreateTenantInviteParams,
) (domain.TenantInvite, error) {
	s.createTenantInviteCalls++
	if s.inviteErr != nil {
		return domain.TenantInvite{}, s.inviteErr
	}
	return domain.TenantInvite{Code: s.inviteCode}, nil
}

func (s *scenarioServiceSpy) AcceptTenantInvite(
	_ context.Context,
	_ financepkg.AcceptTenantInviteParams,
) (domain.TenantMembership, error) {
	s.acceptTenantInviteCalls++
	if s.acceptInviteErr != nil {
		return domain.TenantMembership{}, s.acceptInviteErr
	}
	return domain.TenantMembership{}, nil
}

func (s *scenarioServiceSpy) CreateAccount(
	_ context.Context,
	params financepkg.CreateAccountParams,
) (domain.Account, error) {
	s.createAccountCalls++
	if s.createAccountFailAt > 0 && s.createAccountCalls == s.createAccountFailAt {
		return domain.Account{}, errors.New("create account failed")
	}
	id := s.checkingAccountID
	switch {
	case params.Currency == "EUR":
		if s.secondAccountErr != nil {
			return domain.Account{}, s.secondAccountErr
		}
		id = s.savingsAccountID
	case params.Kind == domain.AccountKindImported:
		id = s.importedAccountID
	case params.Kind == domain.AccountKindReconciliation:
		id = s.reconciliationAccountID
	}
	return domain.Account{ID: id}, nil
}

func (s *scenarioServiceSpy) ListCategories(
	_ context.Context,
	_ financepkg.ListCategoriesParams,
) ([]domain.Category, error) {
	s.listCategoriesCalls++
	if s.listCategoriesErr != nil {
		return nil, s.listCategoriesErr
	}
	if s.categories != nil {
		return s.categories, nil
	}
	return []domain.Category{
		{ID: "cat-paycheck", Name: "Paycheck", Kind: domain.CategoryKindIncome},
		{ID: "cat-bonus", Name: "Bonus", Kind: domain.CategoryKindIncome},
		{ID: "cat-interest", Name: "Interest & Dividends", Kind: domain.CategoryKindIncome},
		{ID: "cat-housing", Name: "Housing", Kind: domain.CategoryKindExpense},
		{ID: "cat-utilities", Name: "Utilities", Kind: domain.CategoryKindExpense},
		{ID: "cat-groceries", Name: "Groceries", Kind: domain.CategoryKindExpense},
		{ID: "cat-dining", Name: "Dining & Coffee", Kind: domain.CategoryKindExpense},
		{ID: "cat-transport", Name: "Transportation", Kind: domain.CategoryKindExpense},
		{ID: "cat-health", Name: "Health & Medical", Kind: domain.CategoryKindExpense},
		{ID: "cat-personal", Name: "Personal Care", Kind: domain.CategoryKindExpense},
		{ID: "cat-entertainment", Name: "Entertainment", Kind: domain.CategoryKindExpense},
		{ID: "cat-shopping", Name: "Shopping", Kind: domain.CategoryKindExpense},
		{ID: "cat-travel", Name: "Travel & Vacation", Kind: domain.CategoryKindExpense},
		{ID: "cat-gifts", Name: "Gifts & Donations", Kind: domain.CategoryKindExpense},
		{ID: "cat-misc", Name: "Miscellaneous", Kind: domain.CategoryKindExpense},
	}, nil
}

func (s *scenarioServiceSpy) ListTags(
	_ context.Context,
	_ financepkg.ListTagsParams,
) ([]domain.Tag, error) {
	s.listTagsCalls++
	if s.listTagsErr != nil {
		return nil, s.listTagsErr
	}
	if s.tags != nil {
		return s.tags, nil
	}
	return []domain.Tag{{ID: "tag-travel", Name: "Travel"}}, nil
}

func (s *scenarioServiceSpy) PreviewCSVImport(
	_ context.Context,
	params financepkg.PreviewCSVImportParams,
) (financepkg.CSVImportPreview, error) {
	s.previewCSVImportCalls++
	s.previewCSVImportCSV = params.CSV
	if s.previewCSVImportErr != nil {
		return financepkg.CSVImportPreview{}, s.previewCSVImportErr
	}
	if s.previewCSVImport.ImportID != "" {
		return s.previewCSVImport, nil
	}
	return financepkg.CSVImportPreview{ImportID: "import-preview"}, nil
}

func (s *scenarioServiceSpy) RecordTransaction(
	_ context.Context,
	_ financepkg.RecordTransactionParams,
) (domain.Transaction, error) {
	s.recordTransactionCalls++
	if s.recordTransactionFailAt > 0 && s.recordTransactionCalls == s.recordTransactionFailAt {
		return domain.Transaction{}, errors.New("record transaction failed")
	}
	if s.recordTransactionErr != nil {
		return domain.Transaction{}, s.recordTransactionErr
	}
	return domain.Transaction{ID: s.recordedTransactionID}, nil
}

func (s *scenarioServiceSpy) HideTransaction(
	_ context.Context,
	_ financepkg.HideTransactionParams,
) error {
	s.hideTransactionCalls++
	return s.hideTransactionErr
}

func (s *scenarioServiceSpy) LinkTransfers(
	_ context.Context,
	_ financepkg.LinkTransfersParams,
) error {
	s.linkTransfersCalls++
	return s.linkTransfersErr
}

func (s *scenarioServiceSpy) LinkTokenBankConnection(
	_ context.Context,
	_ financepkg.LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	s.linkTokenConnectionCalls++
	if s.linkTokenErr != nil {
		return domain.BankConnection{}, s.linkTokenErr
	}
	return domain.BankConnection{ID: "connection-1"}, nil
}

func (s *scenarioServiceSpy) UpsertBankConnectionSchedule(
	_ context.Context,
	_ financepkg.UpsertBankConnectionScheduleParams,
) (domain.BankConnectionSchedule, error) {
	s.upsertScheduleCalls++
	if s.upsertScheduleErr != nil {
		return domain.BankConnectionSchedule{}, s.upsertScheduleErr
	}
	return domain.BankConnectionSchedule{}, nil
}

func (s *scenarioServiceSpy) SyncFXRates(
	_ context.Context,
	_ financepkg.SyncFXRatesParams,
) (financepkg.SyncFXRatesResult, error) {
	s.syncFXRatesCalls++
	if s.syncFXErr != nil {
		return financepkg.SyncFXRatesResult{}, s.syncFXErr
	}
	return financepkg.SyncFXRatesResult{}, nil
}

func TestRealisticScenario(t *testing.T) {
	makeSpy := func(fake faker.Faker) *scenarioServiceSpy {
		return &scenarioServiceSpy{
			createdTenantID:         "tenant-" + fake.UUID().V4(),
			inviteCode:              "invite-" + fake.UUID().V4(),
			checkingAccountID:       "checking-" + fake.UUID().V4(),
			savingsAccountID:        "savings-" + fake.UUID().V4(),
			importedAccountID:       "imported-" + fake.UUID().V4(),
			reconciliationAccountID: "reconciliation-" + fake.UUID().V4(),
			recordedTransactionID:   "transaction-" + fake.UUID().V4(),
		}
	}
	defaultCategories := func(t *testing.T, spy *scenarioServiceSpy) []domain.Category {
		t.Helper()
		categories, err := spy.ListCategories(t.Context(), financepkg.ListCategoriesParams{})
		require.NoError(t, err)
		return categories
	}

	t.Run("service-backed scenario invokes finance APIs and records stable ids", func(t *testing.T) {
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		bootstrap := fixtures.NewBootstrapper(
			fixtures.NewService(fixtures.NewPersistenceRepository(store)),
		)
		spy := makeSpy(fake)
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)

		summary, err := fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			spy,
			fixtures.Config{Seed: 23, Now: now, Scenario: "realistic"},
		)
		require.NoError(t, err)
		assert.Equal(t, int64(23), summary.Seed)
		assert.Equal(t, []string{"realistic-core"}, summary.ScenarioIDs)
		assert.Equal(t, 1, spy.createTenantCalls)
		assert.Equal(t, 1, spy.createTenantInviteCalls)
		assert.Equal(t, 1, spy.acceptTenantInviteCalls)
		assert.Equal(t, 4, spy.createAccountCalls)
		assert.Equal(t, 1, spy.listCategoriesCalls)
		assert.Equal(t, 1, spy.listTagsCalls)
		assert.Equal(t, 1, spy.previewCSVImportCalls)
		assert.Equal(t, 402, spy.recordTransactionCalls)
		assert.Equal(t, 12, spy.hideTransactionCalls)
		assert.Equal(t, 12, spy.linkTransfersCalls)
		assert.Equal(t, 1, spy.linkTokenConnectionCalls)
		assert.Equal(t, 1, spy.upsertScheduleCalls)
		assert.Equal(t, 1, spy.syncFXRatesCalls)
		assert.Contains(t, spy.previewCSVImportCSV, ",Travel,")
	})

	t.Run("returns service errors without direct-table fallback", func(t *testing.T) {
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		bootstrap := fixtures.NewBootstrapper(
			fixtures.NewService(fixtures.NewPersistenceRepository(store)),
		)
		wantErr := errors.New("link failed")
		spy := makeSpy(fake)
		spy.linkTokenErr = wantErr

		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			spy,
			fixtures.Config{
				Seed:     24,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, 1, spy.linkTokenConnectionCalls)
		assert.Zero(t, spy.upsertScheduleCalls)
		assert.Zero(t, spy.syncFXRatesCalls)
	})

	t.Run("returns early service errors from earlier scenario steps", func(t *testing.T) {
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		bootstrap := fixtures.NewBootstrapper(
			fixtures.NewService(fixtures.NewPersistenceRepository(store)),
		)

		createTenantErr := errors.New("tenant create failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			&scenarioServiceSpy{createTenantErr: createTenantErr},
			fixtures.Config{
				Seed:     25,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, createTenantErr)

		inviteErr := errors.New("invite create failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			func() *scenarioServiceSpy {
				spy := makeSpy(fake)
				spy.inviteErr = inviteErr
				return spy
			}(),
			fixtures.Config{
				Seed:     26,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, inviteErr)

		acceptInviteErr := errors.New("invite accept failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			func() *scenarioServiceSpy {
				spy := makeSpy(fake)
				spy.acceptInviteErr = acceptInviteErr
				return spy
			}(),
			fixtures.Config{
				Seed:     27,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, acceptInviteErr)

		upsertScheduleErr := errors.New("schedule create failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			func() *scenarioServiceSpy {
				spy := makeSpy(fake)
				spy.upsertScheduleErr = upsertScheduleErr
				return spy
			}(),
			fixtures.Config{
				Seed:     28,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, upsertScheduleErr)

		recordTransactionErr := errors.New("transaction write failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			func() *scenarioServiceSpy {
				spy := makeSpy(fake)
				spy.recordTransactionErr = recordTransactionErr
				return spy
			}(),
			fixtures.Config{
				Seed:     29,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, recordTransactionErr)

		listTagsErr := errors.New("list tags failed")
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			func() *scenarioServiceSpy {
				spy := makeSpy(fake)
				spy.listTagsErr = listTagsErr
				return spy
			}(),
			fixtures.Config{
				Seed:     30,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, listTagsErr)

		syncFXErr := errors.New("fx sync failed")
		spy := makeSpy(fake)
		spy.syncFXErr = syncFXErr
		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			spy,
			fixtures.Config{
				Seed:     31,
				Now:      time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario: "realistic",
			},
		)
		require.ErrorIs(t, err, syncFXErr)
		assert.Equal(t, 1, spy.syncFXRatesCalls)
	})

	t.Run("returns targeted branch errors across seeded scenario phases", func(t *testing.T) {
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		bootstrap := fixtures.NewBootstrapper(
			fixtures.NewService(fixtures.NewPersistenceRepository(store)),
		)
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		seed := int64(33)

		runScenario := func(t *testing.T, spy *scenarioServiceSpy) error {
			t.Helper()
			seed++
			_, runErr := fixtures.GenerateRealisticScenario(
				t.Context(),
				bootstrap,
				spy,
				fixtures.Config{Seed: seed, Now: now, Scenario: "realistic"},
			)
			return runErr
		}

		t.Run("fails when categories cannot be listed", func(t *testing.T) {
			wantErr := errors.New("list categories failed")
			spy := makeSpy(fake)
			spy.listCategoriesErr = wantErr

			runErr := runScenario(t, spy)
			require.ErrorIs(t, runErr, wantErr)
		})

		t.Run("fails when seeded tags are missing", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.tags = []domain.Tag{}

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "realistic scenario requires seeded default tags")
		})

		t.Run("fails when seeded travel tag is not reusable in csv preview", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.previewCSVImport = financepkg.CSVImportPreview{
				ImportID:        "import-preview",
				WouldCreateTags: []string{"Travel"},
			}

			runErr := runScenario(t, spy)
			require.EqualError(
				t,
				runErr,
				"realistic scenario expected seeded tag to be reusable in csv preview: Travel",
			)
		})

		t.Run("fails account creation in each later account slot", func(t *testing.T) {
			for _, failAt := range []int{1, 3, 4} {
				t.Run(fmt.Sprintf("account-%d", failAt), func(t *testing.T) {
					spy := makeSpy(fake)
					spy.createAccountFailAt = failAt

					runErr := runScenario(t, spy)
					require.EqualError(t, runErr, "create account failed")
				})
			}
		})

		t.Run("fails when required seeded category is absent", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.categories = slices.DeleteFunc(
				append([]domain.Category(nil), defaultCategories(t, spy)...),
				func(category domain.Category) bool { return category.Name == "Bonus" },
			)

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "realistic scenario missing seeded category: Bonus")
		})

		t.Run("fails opening balance write after first opening entry", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 2

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails quarterly bonus write", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 4

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails regular monthly fixture transaction", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 6

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails outbound matched transfer write", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 33

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails inbound matched transfer write", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 34

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails transfer linking", func(t *testing.T) {
			wantErr := errors.New("link transfers failed")
			spy := makeSpy(fake)
			spy.linkTransfersErr = wantErr

			runErr := runScenario(t, spy)
			require.ErrorIs(t, runErr, wantErr)
		})

		t.Run("fails unmatched transfer write", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 35

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails hidden duplicate write", func(t *testing.T) {
			spy := makeSpy(fake)
			spy.recordTransactionFailAt = 36

			runErr := runScenario(t, spy)
			require.EqualError(t, runErr, "record transaction failed")
		})

		t.Run("fails hide transaction", func(t *testing.T) {
			wantErr := errors.New("hide transaction failed")
			spy := makeSpy(fake)
			spy.hideTransactionErr = wantErr

			runErr := runScenario(t, spy)
			require.ErrorIs(t, runErr, wantErr)
		})
	})

	t.Run("allows auth-aligned owner and member overrides", func(t *testing.T) {
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		bootstrap := fixtures.NewBootstrapper(
			fixtures.NewService(fixtures.NewPersistenceRepository(store)),
		)
		spy := makeSpy(fake)

		_, err = fixtures.GenerateRealisticScenario(
			t.Context(),
			bootstrap,
			spy,
			fixtures.Config{
				Seed:         32,
				Now:          time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC),
				Scenario:     "realistic",
				OwnerUserID:  "owner-override-" + fake.UUID().V4(),
				MemberUserID: "member-override-" + fake.UUID().V4(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, 1, spy.createTenantCalls)
		assert.Equal(t, 1, spy.acceptTenantInviteCalls)
	})

	fake := faker.New()
	store, err := persistence.NewStore(fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()))
	require.NoError(t, err)
	require.NoError(t, store.Migrate(t.Context()))
	repo := fixtures.NewPersistenceRepository(store)
	bootstrap := fixtures.NewBootstrapper(fixtures.NewService(repo))
	now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
	cipher, err := credentials.NewAESGCMCipher(
		[]byte("12345678901234567890123456789012"),
		"fixture-key",
	)
	require.NoError(t, err)
	financeService := financepkg.NewService(
		store,
		financepkg.WithNow(func() time.Time { return now }),
		financepkg.WithConnectionSecretCipher(cipher),
		financepkg.WithBankProviders(realisticScenarioProvider{}),
		financepkg.WithFXProviders(financepkg.NewStaticFXProvider(
			financepkg.FXProviderFrankfurter,
			fixtures.RealisticScenarioStaticFXRates(financepkg.FXProviderFrankfurter, now),
		)),
	)

	summary, err := fixtures.GenerateRealisticScenario(
		t.Context(),
		bootstrap,
		financeService,
		fixtures.Config{
			Seed:     17,
			Now:      now,
			Scenario: "realistic",
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(17), summary.Seed)
	assert.NotEmpty(t, summary.ScenarioIDs)

	ownerTenants, err := financeService.ListTenantsForUser(
		t.Context(),
		fixtures.RealisticScenarioOwnerUserID(17),
	)
	require.NoError(t, err)
	require.NotEmpty(t, ownerTenants)
	tenantID := ownerTenants[0].Tenant.ID

	accounts, err := financeService.ListAccounts(t.Context(), financepkg.ListAccountsParams{
		ActorUserID: fixtures.RealisticScenarioOwnerUserID(17),
		TenantID:    tenantID,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(accounts), 2)

	transactions, err := financeService.ListTransactions(
		t.Context(),
		financepkg.ListTransactionsParams{
			ActorUserID:   fixtures.RealisticScenarioOwnerUserID(17),
			TenantID:      tenantID,
			IncludeHidden: true,
		},
	)
	require.NoError(t, err)

	monthCounts := map[string]int{}
	accountKinds := map[domain.AccountKind]int{}
	currencies := map[string]struct{}{}
	sawPending := false
	sawRefund := false
	sawMatchedTransfer := false
	sawUnmatchedTransfer := false
	sawReconciliation := false
	sawOpeningBalance := false
	sawHidden := false
	sawCSV := false
	sawProvider := false
	sawProviderOriginal := false
	for _, account := range accounts {
		accountKinds[account.Kind]++
		currencies[account.Currency] = struct{}{}
	}
	for _, transaction := range transactions {
		monthCounts[transaction.EffectiveAt.Format("2006-01")]++
		currencies[transaction.Currency] = struct{}{}
		switch transaction.Status {
		case domain.TransactionStatusPending:
			sawPending = true
		case domain.TransactionStatusBooked:
		}
		switch transaction.Kind {
		case domain.TransactionKindRefund:
			sawRefund = true
		case domain.TransactionKindTransfer:
			if transaction.TransferMatchedAt != nil {
				sawMatchedTransfer = true
			} else {
				sawUnmatchedTransfer = true
			}
		case domain.TransactionKindReconciliation:
			sawReconciliation = true
		case domain.TransactionKindOpeningBalance:
			sawOpeningBalance = true
		case domain.TransactionKindRegular:
		}
		if transaction.HiddenAt != nil {
			sawHidden = true
		}
		switch transaction.Source {
		case domain.TransactionSourceCSV:
			sawCSV = true
		case domain.TransactionSourceProvider:
			sawProvider = true
		case domain.TransactionSourceManual, domain.TransactionSourceSystem:
		}
		if transaction.ProviderOriginal != nil {
			sawProviderOriginal = true
		}
	}
	months := make([]string, 0, len(monthCounts))
	for monthKey := range monthCounts {
		months = append(months, monthKey)
	}
	slices.Sort(months)

	require.Len(t, months, 12)
	assert.Equal(t, "2025-07", months[0])
	assert.Equal(t, "2026-06", months[len(months)-1])
	for _, monthKey := range months {
		assert.GreaterOrEqual(t, monthCounts[monthKey], 30, monthKey)
		assert.LessOrEqual(t, monthCounts[monthKey], 40, monthKey)
	}
	assert.GreaterOrEqual(t, len(transactions), 360)
	assert.LessOrEqual(t, len(transactions), 480)
	assert.GreaterOrEqual(t, accountKinds[domain.AccountKindManual], 1)
	assert.GreaterOrEqual(t, accountKinds[domain.AccountKindImported], 1)
	assert.GreaterOrEqual(t, accountKinds[domain.AccountKindReconciliation], 1)
	assert.GreaterOrEqual(t, len(currencies), 2)
	assert.True(t, sawPending)
	assert.True(t, sawRefund)
	assert.True(t, sawMatchedTransfer)
	assert.True(t, sawUnmatchedTransfer)
	assert.True(t, sawReconciliation)
	assert.True(t, sawOpeningBalance)
	assert.True(t, sawHidden)
	assert.True(t, sawCSV)
	assert.True(t, sawProvider)
	assert.True(t, sawProviderOriginal)

	connections, err := financeService.ListBankConnections(
		t.Context(),
		financepkg.ListBankConnectionsParams{
			ActorUserID: fixtures.RealisticScenarioOwnerUserID(17),
			TenantID:    tenantID,
		},
	)
	require.NoError(t, err)
	assert.Len(t, connections, 1)

	dashboard, err := financeService.GetDashboard(t.Context(), financepkg.DashboardParams{
		ActorUserID: fixtures.RealisticScenarioOwnerUserID(17),
		TenantID:    tenantID,
	})
	require.NoError(t, err)
	assert.NotZero(t, dashboard.Settled.TransactionCount)

	summaryAgain, err := fixtures.GenerateRealisticScenario(
		t.Context(),
		bootstrap,
		financeService,
		fixtures.Config{
			Seed:     17,
			Now:      now.Add(time.Hour),
			Scenario: "realistic-second",
		},
	)
	require.NoError(t, err)
	assert.Equal(t, summary.ScenarioIDs, summaryAgain.ScenarioIDs)
}

type realisticScenarioProvider struct{}

func (realisticScenarioProvider) Name() string { return "monobank" }

func (realisticScenarioProvider) StartLink(
	_ context.Context,
	_ financepkg.ProviderStartLinkParams,
) (financepkg.ProviderLinkStart, error) {
	return financepkg.ProviderLinkStart{}, nil
}

func (realisticScenarioProvider) FinishLink(
	_ context.Context,
	_ financepkg.ProviderFinishLinkParams,
) (financepkg.ProviderLinkResult, error) {
	return financepkg.ProviderLinkResult{}, nil
}

func (realisticScenarioProvider) LinkToken(
	_ context.Context,
	_ financepkg.ProviderTokenLinkParams,
) (financepkg.ProviderTokenLinkResult, error) {
	return financepkg.ProviderTokenLinkResult{
		DisplayName:       "Scenario Connection",
		ProviderReference: "scenario-ref",
		ExternalID:        "ext-1",
		Secret:            "secret",
		State:             "active",
	}, nil
}

func (realisticScenarioProvider) Sync(
	_ context.Context,
	_ financepkg.ProviderSyncParams,
) (financepkg.ProviderSyncResult, error) {
	return financepkg.ProviderSyncResult{}, nil
}
