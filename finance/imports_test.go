package finance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingCSVJobEnqueuer struct {
	requests []CSVImportJobRequest
	jobType  string
	jobID    string
	err      error
}

func (r *recordingCSVJobEnqueuer) EnqueueCSVImport(
	_ context.Context,
	request CSVImportJobRequest,
) (CSVImportJobRef, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return CSVImportJobRef{}, r.err
	}
	return CSVImportJobRef{ID: r.jobID, JobType: r.jobType}, nil
}

type failingCSVImportStore struct {
	serviceStore

	saveErr error
	getErr  error
}

func (s failingCSVImportStore) SaveCSVImport(
	_ context.Context,
	_ domain.CSVImportRecord,
) (domain.CSVImportRecord, error) {
	return domain.CSVImportRecord{}, s.saveErr
}

func (s failingCSVImportStore) GetCSVImport(
	_ context.Context,
	_ string,
) (*domain.CSVImportRecord, error) {
	return nil, s.getErr
}

func TestCSVImports(t *testing.T) {
	makeService := func(t *testing.T, opts ...ServiceOption) *Service {
		t.Helper()
		fake := faker.New()
		store, err := persistence.NewStore(
			fmt.Sprintf("%s/%s.db", t.TempDir(), fake.Lorem().Word()),
		)
		require.NoError(t, err)
		require.NoError(t, store.Migrate(t.Context()))
		return NewService(store, opts...)
	}

	makeTenant := func(t *testing.T, service *Service, actorUserID string) domain.Tenant {
		t.Helper()
		fake := faker.New()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)
		return tenant
	}

	makeMembership := func(t *testing.T, service *Service, ownerUserID, memberUserID, tenantID string) {
		t.Helper()
		invite, err := service.CreateTenantInvite(t.Context(), CreateTenantInviteParams{
			ActorUserID: ownerUserID,
			TenantID:    tenantID,
			Recipient:   "member-" + faker.New().Internet().Email(),
		})
		require.NoError(t, err)
		_, err = service.AcceptTenantInvite(t.Context(), AcceptTenantInviteParams{
			ActorUserID: memberUserID,
			Code:        invite.Code,
		})
		require.NoError(t, err)
	}

	t.Run("preview confirm and import transaction csv with audit metadata", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeTransactions,
		}
		service := makeService(
			t,
			WithNow(func() time.Time { return now }),
			WithCSVImportJobEnqueuer(enqueuer),
		)
		tenant := makeTenant(t, service, actorUserID)

		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "checking-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		category, err := service.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "groceries-" + fake.Lorem().Word(),
			Kind:        domain.CategoryKindExpense,
		})
		require.NoError(t, err)
		_, err = service.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -1250,
			Currency:    "USD",
			Description: "coffee duplicate",
			EffectiveAt: now.Add(-24 * time.Hour),
			CategoryID:  category.ID,
		})
		require.NoError(t, err)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			FileName:    "transactions.csv",
			CSV: "accountName,currency,effectiveAt,amountMinor,description,category,tag,status\n" +
				account.Name + ",USD," + now.Add(-24*time.Hour).Format(time.DateOnly) + ",-1250,coffee duplicate," + category.Name + ",morning,booked\n" +
				"wallet-" + fake.Lorem().Word() + ",EUR," + now.Format(time.DateOnly) + ",-5000,lunch," + "meals-" + fake.Lorem().Word() + ",team,booked\n" +
				account.Name + ",USD," + now.Format(time.DateOnly) + ",oops,bad row," + category.Name + ",,booked\n",
		})
		require.NoError(t, err)
		assert.Equal(t, CSVImportTypeTransactions, preview.ImportType)
		assert.Equal(
			t,
			[]string{
				"accountName",
				"currency",
				"effectiveAt",
				"amountMinor",
				"description",
				"category",
				"tag",
				"status",
			},
			preview.Headers,
		)
		assert.Len(t, preview.DuplicateRows, 1)
		assert.Len(t, preview.RejectedRows, 1)
		require.Len(t, preview.WouldCreateAccounts, 1)
		assert.Contains(t, preview.WouldCreateAccounts[0], "wallet-")
		assert.Len(t, preview.WouldCreateCategories, 1)
		assert.Equal(t, []string{"morning", "team"}, preview.WouldCreateTags)

		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.NoError(t, err)
		assert.Equal(t, enqueuer.jobID, confirmed.JobID)
		require.Len(t, enqueuer.requests, 1)
		assert.Equal(t, CSVImportJobTypeTransactions, enqueuer.requests[0].JobType)

		result, err := service.RunCSVImportJob(
			t.Context(),
			RunCSVImportJobParams{ImportID: preview.ImportID, JobID: confirmed.JobID},
		)
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedCount)
		assert.Len(t, result.RejectedRows, 2)

		audit, err := service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, confirmed.JobID, audit.JobID)
		assert.Equal(t, actorUserID, audit.ConfirmedByUserID)
		assert.Equal(t, CSVImportStatusCompleted, audit.Status)
	})

	t.Run("reordered transaction csv uses header mapping across preview confirm and run", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeTransactions,
		}
		service := makeService(
			t,
			WithNow(func() time.Time { return now }),
			WithCSVImportJobEnqueuer(enqueuer),
		)
		tenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			FileName:    "transactions-reordered.csv",
			CSV: "description,effectiveAt,status,accountName,tag,amountMinor,currency,category\n" +
				"lunch," + now.Format(time.DateOnly) + ",booked,wallet-eur,team,-5000,EUR,meals\n",
		})
		require.NoError(t, err)
		assert.Empty(t, preview.RejectedRows)
		assert.Equal(t, []string{"wallet-eur"}, preview.WouldCreateAccounts)
		assert.Equal(t, []string{"meals"}, preview.WouldCreateCategories)
		assert.Equal(t, []string{"team"}, preview.WouldCreateTags)

		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.NoError(t, err)

		result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID,
			JobID:    confirmed.JobID,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedCount)
		assert.Empty(t, result.RejectedRows)

		accounts, err := service.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "wallet-eur", accounts[0].Name)
		assert.Equal(t, "EUR", accounts[0].Currency)

		transactions, err := service.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		assert.Equal(t, int64(-5000), transactions[0].AmountMinor)
		assert.Equal(t, "lunch", transactions[0].Description)
		assert.Equal(t, domain.TransactionStatusBooked, transactions[0].Status)
	})

	t.Run("account import confirms finance.account_import job type", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeAccounts,
		}
		service := makeService(t, WithCSVImportJobEnqueuer(enqueuer))
		tenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.NoError(t, err)
		assert.Equal(t, CSVImportJobTypeAccounts, confirmed.JobType)
	})

	t.Run("preview rejects empty csv input", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		service := makeService(t)
		tenant := makeTenant(t, service, actorUserID)

		_, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			FileName:    "empty.csv",
			CSV:         "",
		})
		require.EqualError(t, err, "csv import requires at least one row")
	})

	t.Run("confirm requires configured csv import enqueuer", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		service := makeService(t)
		tenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.EqualError(t, err, "csv import job enqueuer is required")
	})

	t.Run("account import job imports valid rows and reports invalid ones", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeAccounts,
		}
		service := makeService(
			t,
			WithNow(func() time.Time { return now }),
			WithCSVImportJobEnqueuer(enqueuer),
		)
		tenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\ninvalid,,\n",
		})
		require.NoError(t, err)
		require.Len(t, preview.RejectedRows, 1)

		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.NoError(t, err)

		result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID,
			JobID:    confirmed.JobID,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedCount)
		require.Len(t, result.RejectedRows, 1)
		assert.Equal(t, 3, result.RejectedRows[0].RowNumber)

		accounts, err := service.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "wallet", accounts[0].Name)
		assert.Equal(t, domain.AccountKindManual, accounts[0].Kind)
	})

	t.Run("audit requires tenant membership", func(t *testing.T) {
		fake := faker.New()
		ownerUserID := "owner-" + fake.UUID().V4()
		memberUserID := "member-" + fake.UUID().V4()
		outsiderUserID := "outsider-" + fake.UUID().V4()
		service := makeService(t)
		tenant := makeTenant(t, service, ownerUserID)
		makeMembership(t, service, ownerUserID, memberUserID, tenant.ID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: memberUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		_, err = service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: outsiderUserID,
			TenantID:    tenant.ID,
			ImportID:    preview.ImportID,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant")
	})

	t.Run("audit rejects mismatched tenant even for a real member", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		service := makeService(t)
		importTenant := makeTenant(t, service, actorUserID)
		otherTenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    importTenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		_, err = service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: actorUserID,
			TenantID:    otherTenant.ID,
			ImportID:    preview.ImportID,
		})
		require.ErrorIs(t, err, ErrTenantAccessDenied)
	})

	t.Run("confirm rejects repeated confirmation and does not enqueue duplicate job", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeAccounts,
		}
		service := makeService(t, WithCSVImportJobEnqueuer(enqueuer))
		tenant := makeTenant(t, service, actorUserID)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, err)

		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.NoError(t, err)

		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.ErrorIs(t, err, ErrCSVImportAlreadyConfirmed)
		require.Len(t, enqueuer.requests, 1)
	})

	t.Run("preview returns persistence errors after building preview", func(t *testing.T) {
		fake := faker.New()
		wantErr := assert.AnError
		service := NewService(
			failingCSVImportStore{serviceStore: makeService(t).store, saveErr: wantErr},
			WithIDGenerator(func() string { return "import-" + fake.UUID().V4() }),
		)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, service, actorUserID)

		_, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("confirm surfaces lookup and enqueue errors", func(t *testing.T) {
		fake := faker.New()
		service := NewService(
			failingCSVImportStore{serviceStore: makeService(t).store, getErr: assert.AnError},
		)
		_, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: "user-" + fake.UUID().V4(),
			ImportID:    "import-" + fake.UUID().V4(),
			Mapping:     map[string]string{"name": "name"},
		})
		require.ErrorIs(t, err, assert.AnError)

		actorUserID := "user-" + fake.UUID().V4()
		enqueueErr := fmt.Errorf("queue failed: %w", assert.AnError)
		service = makeService(
			t,
			WithCSVImportJobEnqueuer(&recordingCSVJobEnqueuer{err: enqueueErr}),
		)
		tenant := makeTenant(t, service, actorUserID)
		preview, previewErr := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\n",
		})
		require.NoError(t, previewErr)

		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
			Mapping:     preview.Mapping,
		})
		require.ErrorIs(t, err, enqueueErr)
		assert.Contains(t, err.Error(), "confirm csv import")
	})

	t.Run("preview and audit surface malformed csv and lookup failures", func(t *testing.T) {
		fake := faker.New()
		actorUserID := "user-" + fake.UUID().V4()
		service := makeService(t)
		tenant := makeTenant(t, service, actorUserID)

		_, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			FileName:    "broken.csv",
			CSV:         "account,amount\n\"unterminated",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read csv import")

		service = NewService(failingCSVImportStore{serviceStore: makeService(t).store, getErr: assert.AnError})
		_, err = service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportID:    "import-" + fake.UUID().V4(),
		})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("mapping helpers ignore unknown overrides and fall back by field name", func(t *testing.T) {
		resolved := confirmedCSVImportMapping(
			CSVImportTypeTransactions,
			[]string{"accountName", "currency"},
			map[string]string{},
			map[string]string{"accountName": "missing", "currency": "currency"},
		)
		assert.Equal(t, map[string]string{"currency": "currency"}, resolved)
		assert.Equal(
			t,
			"USD",
			csvImportMappedValue([]string{"accountName", "currency"}, nil, []string{"wallet", "USD"}, "currency"),
		)
		assert.Empty(t, csvImportMappedValue([]string{"accountName"}, nil, []string{"wallet"}, "currency"))
	})

	t.Run("helper functions cover edge cases", func(t *testing.T) {
		rows, err := readCSVRows("name,currency\nwallet,USD\n")
		require.NoError(t, err)
		assert.Equal(t, [][]string{{"name", "currency"}, {"wallet", "USD"}}, rows)

		_, err = readCSVRows("name\n\"unterminated")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read csv import")

		assert.Equal(t, "USD", valueAt([]string{"wallet", "USD"}, 1))
		assert.Empty(t, valueAt([]string{"wallet"}, 3))
		assert.Equal(t, []string{"wallet"}, appendMissing(appendMissing(nil, "wallet"), "wallet"))
		assert.Empty(t, appendMissing(nil, ""))

		accounts := []domain.Account{{ID: "account-1", Name: "wallet"}}
		categories := []domain.Category{{ID: "category-1", Name: "groceries"}}
		tags := []domain.Tag{{ID: "tag-1", Name: "team"}}

		assert.Contains(t, setFromAccounts(accounts), "wallet")
		assert.Contains(t, setFromCategories(categories), "groceries")
		assert.Contains(t, setFromTags(tags), "team")
		require.NotNil(t, findAccountByName(accounts, "wallet"))
		assert.Nil(t, findAccountByName(accounts, "missing"))
		require.NotNil(t, findCategoryByName(categories, "groceries"))
		assert.Nil(t, findCategoryByName(categories, "missing"))
		require.NotNil(t, findTagByName(tags, "team"))
		assert.Nil(t, findTagByName(tags, "missing"))
	})
}
