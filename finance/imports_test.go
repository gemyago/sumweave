package finance

import (
	"context"
	"strings"
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

func TestCSVImport(t *testing.T) {
	makeImportService := func(t *testing.T) (*CSVImportService, *TenantService, *CatalogService, *LedgerService) {
		t.Helper()
		database := openTestDatabase(t)
		store := persistence.NewStore(database)
		catalog := NewCatalogService(store)
		ledger := NewLedgerService(
			store,
			WithLedgerServiceTransactionStore(persistence.NewTransactionTagStore(database)),
		)
		return NewCSVImportService(
			store,
			catalog,
			ledger,
			WithCSVImportServiceRowStore(persistence.NewCSVImportStore(database)),
		), NewTenantService(store), catalog, ledger
	}
	makeTenant := func(t *testing.T, tenants *TenantService, actorUserID string) domain.Tenant {
		t.Helper()
		tenant, err := tenants.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     actorUserID,
			Name:            "tenant-" + faker.New().Company().Name(),
			DisplayCurrency: tenantDisplayCurrencyUSD,
		})
		require.NoError(t, err)
		return tenant
	}
	transactionCSV := func(rows ...string) string {
		return "Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n" +
			strings.Join(rows, "\n") + "\n"
	}
	previewAndRun := func(
		t *testing.T,
		service *CSVImportService,
		actorUserID string,
		tenantID string,
		csv string,
	) (CSVImportPreview, CSVImportRunResult) {
		t.Helper()
		enqueuer := &recordingCSVJobEnqueuer{
			jobID:   "job-" + faker.New().UUID().V4(),
			jobType: CSVImportJobTypeTransactions,
		}
		service.csvImportJobEnqueuer = enqueuer
		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenantID,
			ImportType:  CSVImportTypeTransactions,
			FileName:    "transactions.csv",
			CSV:         csv,
		})
		require.NoError(t, err)
		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		repeated, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, confirmed, repeated)
		require.Equal(t, "finance.csv-import:"+preview.ImportID, enqueuer.requests[0].IdempotencyKey)
		result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID,
			JobID:    confirmed.JobID,
		})
		require.NoError(t, err)
		return preview, result
	}

	t.Run("parses required normalized headers with unsupported columns and localized money", func(t *testing.T) {
		headers, rows, rejected, err := parseFixedCSV(
			"\ufeff source , Description , Currency , Income amount , ignored , Tags , Category , Expense amount , Account , Date , source \n" +
				"first,lunch,pln,,middle,\"team, travel, team\",Meals,\"8\u00a0300,00\", Wallet ,29.05.26,last\n",
		)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"source", "Description", "Currency", "Income amount", "ignored", "Tags",
			"Category", "Expense amount", "Account", "Date", "source",
		}, headers)
		require.Empty(t, rejected)
		require.Len(t, rows, 1)
		assert.Equal(t, "Wallet", rows[0].Account)
		assert.Equal(t, int64(-830000), rows[0].AmountMinor)
		assert.Equal(t, []string{"team", "travel"}, rows[0].Tags)
		assert.Equal(t, time.Date(2026, time.May, 29, 0, 0, 0, 0, time.Local), rows[0].Date)
	})

	t.Run("rejects invalid headers and malformed transaction cells", func(t *testing.T) {
		cases := []string{
			"Date,Account,Category,Tags,Expense amount,Income amount\n",
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Date\n",
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Unexpected,Date\n",
		}
		for _, raw := range cases {
			_, _, _, err := parseFixedCSV(raw)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidCSVImport)
		}
		_, rows, rejected, err := parseFixedCSV(transactionCSV(
			"29.05.26,Wallet,,,1,1,USD,both",
			"29.05.26,Wallet,,,,USD,neither",
			"2026-05-29,Wallet,,,1,,USD,iso",
			"29.05.26,Wallet,,,1,,BTC,currency",
			"29.05.26,Wallet,\"bad,,tag\",1,,USD,tags",
		))
		require.NoError(t, err)
		assert.Empty(t, rows)
		require.Len(t, rejected, 5)
		assert.Equal(t, "expense amount", rejected[0].Field)
		assert.Equal(t, "date", rejected[2].Field)
		assert.Equal(t, "currency", rejected[3].Field)
		assert.Equal(t, `currency "BTC" must be one of USD, EUR, PLN, UAH`, rejected[3].Reason)
	})

	t.Run("normalizes optional descriptions and reports submitted unsupported currencies", func(t *testing.T) {
		headers, rows, rejected, err := parseFixedCSV(
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency\n" +
				"29.05.26,Wallet,,,1,,USD\n",
		)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"Date", "Account", "Category", "Tags", "Expense amount", "Income amount", "Currency",
		}, headers)
		require.Empty(t, rejected)
		require.Len(t, rows, 1)
		assert.Equal(t, "n/a", rows[0].Description)

		_, rows, rejected, err = parseFixedCSV(transactionCSV(
			"29.05.26,Wallet,,,1,,USD,   ",
			"30.05.26,Wallet,,,1,,USD,  trimmed description  ",
			"31.05.26,Wallet,,,1,, gBp ,ignored",
			"01.06.26,Wallet,,,1,,,ignored",
		))
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, "n/a", rows[0].Description)
		assert.Equal(t, "trimmed description", rows[1].Description)
		require.Len(t, rejected, 2)
		assert.Equal(t, `currency " gBp " must be one of USD, EUR, PLN, UAH`, rejected[0].Reason)
		assert.Equal(t, `currency "" must be one of USD, EUR, PLN, UAH`, rejected[1].Reason)

		_, _, _, err = parseFixedCSV(
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description,Description\n",
		)
		require.ErrorContains(t, err, `duplicate csv header "Description"`)
	})

	t.Run("enforces strict years dates money and bounded input", func(t *testing.T) {
		valid, err := parseCSVImportDate("31.12.99")
		require.NoError(t, err)
		assert.Equal(t, 2099, valid.Year())
		for _, date := range []string{"29.02.25", "1.01.26", "29.05.2026", "29-05-26"} {
			_, parseErr := parseCSVImportDate(date)
			require.Error(t, parseErr)
		}
		for input, want := range map[string]int64{"8\u00a0300,00": 830000, "1 234,5": 123450, "0": 0} {
			amount, set, parseErr := parseCSVImportAmount(input)
			require.NoError(t, parseErr)
			assert.Equal(t, want, amount)
			assert.Equal(t, want != 0, set)
		}
		for _, amount := range []string{"-1", "1.00", "1,001", ",10", "99999999999999999999"} {
			_, _, parseErr := parseCSVImportAmount(amount)
			require.Error(t, parseErr)
		}
		require.NoError(t, validateCSVImportByteLength(MaxCSVImportBytes))
		require.EqualError(
			t,
			validateCSVImportByteLength(MaxCSVImportBytes+1),
			"csv import exceeds 67108864 bytes",
		)
	})

	t.Run("accepts exactly the configured data-row limit and rejects the next row", func(t *testing.T) {
		readRows := func(source string) (int, error) {
			count := 0
			err := readBoundedCSVRecords(strings.NewReader(source), func(_ []string, rowNumber int) error {
				if rowNumber > 1 {
					count++
				}
				return nil
			})
			return count, err
		}
		atLimit := "header\n" + strings.Repeat("row\n", MaxCSVImportRows)
		count, err := readRows(atLimit)
		require.NoError(t, err)
		assert.Equal(t, MaxCSVImportRows, count)

		count, err = readRows(atLimit + "row\n")
		require.EqualError(t, err, "csv import exceeds 250000 data rows")
		assert.Equal(t, MaxCSVImportRows, count)
	})

	t.Run("rejects malformed and resource-exhausting CSV records", func(t *testing.T) {
		_, _, _, err := parseFixedCSV("")
		require.Error(t, err)
		malformedCSV := strings.Join([]string{
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description",
			"\"unterminated",
		}, "\n")
		_, _, _, err = parseFixedCSV(malformedCSV)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidCSVImport)
		_, _, _, err = parseFixedCSV("Date,Account,Category,Tags,Expense amount,Income amount,Currency,Unexpected\n")
		require.NoError(t, err)
		_, _, _, err = parseFixedCSV("Date,Date,Category,Tags,Expense amount,Income amount,Currency,Description\n")
		require.Error(t, err)
		_, _, _, err = parseFixedCSV(
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description,one,two,three,four,five,six,seven,eight,nine\n",
		)
		require.Error(t, err)
		largeCellCSV := strings.Join([]string{
			"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description",
			"29.05.26,Wallet,,,1,,USD," + strings.Repeat("a", maxCSVImportCellBytes+1),
		}, "\n")
		_, _, _, err = parseFixedCSV(largeCellCSV)
		require.Error(t, err)
	})

	t.Run("previews and imports supported columns around ignored columns", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		accountName := "wallet-" + fake.Lorem().Word()
		csv := "before,Date,Account,between,Category,Tags,Expense amount,Income amount,Currency,Description,after\n" +
			"one,29.05.26," + accountName + ",two,Meals,\"team, travel\",\"8\u00a0300,00\",,PLN,lunch,three\n"
		preview, result := previewAndRun(t, service, actorUserID, tenant.ID, csv)
		assert.Equal(t, []string{accountName}, preview.WouldCreateAccounts)
		assert.Equal(t, 1, preview.ImportableCount)
		assert.Equal(t, 1, result.ImportedCount)
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		assert.Equal(t, int64(-830000), transactions[0].AmountMinor)
		assert.Equal(t, "lunch", transactions[0].Description)
		accounts, err := catalog.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "PLN", accounts[0].Currency)
	})

	t.Run("filters transaction previews and durable imports by selected textual accounts", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		includedAccount := "included-" + fake.Lorem().Word()
		excludedAccount := "excluded-" + fake.Lorem().Word()
		includedCategory := "included-category-" + fake.Lorem().Word()
		excludedCategory := "excluded-category-" + fake.Lorem().Word()
		csv := transactionCSV(
			"29.05.26,"+includedAccount+","+includedCategory+",included-tag,1,,USD,included",
			"invalid,"+excludedAccount+","+excludedCategory+",excluded-tag,1,,USD,invalid excluded",
			"30.05.26,"+excludedAccount+","+excludedCategory+",excluded-tag,2,,USD,excluded",
		)

		allPreview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			CSV:         csv,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, allPreview.ImportableCount)
		assert.Equal(t, []CSVImportAccountOption{
			{Name: excludedAccount, SourceRowCount: 2, Selected: true},
			{Name: includedAccount, SourceRowCount: 1, Selected: true},
		}, allPreview.AccountOptions)
		assert.Len(t, allPreview.RejectedRows, 1)

		includedPreview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID:          actorUserID,
			TenantID:             tenant.ID,
			ImportType:           CSVImportTypeTransactions,
			CSV:                  csv,
			SelectedAccountNames: []string{"  " + includedAccount + "  "},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, includedPreview.ImportableCount)
		assert.Empty(t, includedPreview.RejectedRows)
		assert.Equal(t, []string{includedAccount}, includedPreview.WouldCreateAccounts)
		assert.Equal(t, []string{includedCategory}, includedPreview.WouldCreateCategories)
		assert.Equal(t, []string{"included-tag"}, includedPreview.WouldCreateTags)
		assert.Equal(t, []CSVImportAccountOption{
			{Name: excludedAccount, SourceRowCount: 2, Selected: false},
			{Name: includedAccount, SourceRowCount: 1, Selected: true},
		}, includedPreview.AccountOptions)

		nonePreview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID:          actorUserID,
			TenantID:             tenant.ID,
			ImportType:           CSVImportTypeTransactions,
			CSV:                  csv,
			SelectedAccountNames: []string{},
		})
		require.NoError(t, err)
		assert.Zero(t, nonePreview.ImportableCount)
		assert.Empty(t, nonePreview.RejectedRows)
		assert.Empty(t, nonePreview.WouldCreateAccounts)

		service.csvImportJobEnqueuer = &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeTransactions,
		}
		confirmation, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    includedPreview.ImportID,
		})
		require.NoError(t, err)
		result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: includedPreview.ImportID,
			JobID:    confirmation.JobID,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedCount)
		retried, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: includedPreview.ImportID})
		require.NoError(t, err)
		assert.Equal(t, result, retried)
		accounts, err := catalog.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{includedAccount}, []string{accounts[0].Name})
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Len(t, transactions, 1)
	})

	t.Run("previews proposed category kinds in selected source order", func(t *testing.T) {
		fake := faker.New()
		makePreview := func(t *testing.T, rows ...string) (CSVImportPreview, *CSVImportService, string, string) {
			t.Helper()
			service, tenants, _, _ := makeImportService(t)
			actorUserID := "user-" + fake.UUID().V4()
			tenant := makeTenant(t, tenants, actorUserID)
			preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
				ActorUserID: actorUserID,
				TenantID:    tenant.ID,
				ImportType:  CSVImportTypeTransactions,
				CSV:         transactionCSV(rows...),
			})
			require.NoError(t, err)
			return preview, service, actorUserID, tenant.ID
		}

		t.Run("rejects income after a proposed expense category", func(t *testing.T) {
			accountName := "wallet-" + fake.Lorem().Word()
			categoryName := "category-" + fake.Lorem().Word()
			preview, service, actorUserID, _ := makePreview(
				t,
				"29.05.26,"+accountName+","+categoryName+",first-tag,1,,USD,expense",
				"30.05.26,"+accountName+","+categoryName+",second-tag,,1,USD,income",
			)
			assert.Equal(t, 1, preview.ImportableCount)
			assert.Empty(t, preview.DuplicateRows)
			assert.Equal(t, []CSVImportRejectedRow{{
				RowNumber: 3,
				Field:     "category",
				Reason:    "Category \"" + categoryName + "\" is expense, incompatible with transaction direction",
			}}, preview.RejectedRows)
			assert.Equal(t, []string{accountName}, preview.WouldCreateAccounts)
			assert.Equal(t, []string{categoryName}, preview.WouldCreateCategories)
			assert.Equal(t, []string{"first-tag"}, preview.WouldCreateTags)

			service.csvImportJobEnqueuer = &recordingCSVJobEnqueuer{
				jobID: "job-" + fake.UUID().V4(), jobType: CSVImportJobTypeTransactions,
			}
			confirmation, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
				ActorUserID: actorUserID, ImportID: preview.ImportID,
			})
			require.NoError(t, err)
			result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
				ImportID: preview.ImportID, JobID: confirmation.JobID,
			})
			require.NoError(t, err)
			assert.Equal(t, 1, result.ImportedCount)
			assert.Len(t, result.RejectedRows, 1)
			assert.Equal(t, 3, result.RejectedRows[0].RowNumber)
			assert.Equal(
				t,
				"category \""+categoryName+"\" is expense, incompatible with transaction direction",
				result.RejectedRows[0].Reason,
			)
		})

		t.Run("rejects expense after a proposed income category", func(t *testing.T) {
			accountName := "wallet-" + fake.Lorem().Word()
			categoryName := "category-" + fake.Lorem().Word()
			preview, _, _, _ := makePreview(
				t,
				"29.05.26,"+accountName+","+categoryName+",first-tag,,1,USD,income",
				"30.05.26,"+accountName+","+categoryName+",second-tag,1,,USD,expense",
			)
			assert.Equal(t, 1, preview.ImportableCount)
			assert.Empty(t, preview.DuplicateRows)
			assert.Equal(t, []CSVImportRejectedRow{{
				RowNumber: 3,
				Field:     "category",
				Reason:    "Category \"" + categoryName + "\" is income, incompatible with transaction direction",
			}}, preview.RejectedRows)
			assert.Equal(t, []string{categoryName}, preview.WouldCreateCategories)
			assert.Equal(t, []string{"first-tag"}, preview.WouldCreateTags)
		})

		t.Run("allows same-kind category reuse", func(t *testing.T) {
			accountName := "wallet-" + fake.Lorem().Word()
			categoryName := "category-" + fake.Lorem().Word()
			preview, _, _, _ := makePreview(
				t,
				"29.05.26,"+accountName+","+categoryName+",first-tag,1,,USD,first expense",
				"30.05.26,"+accountName+","+categoryName+",second-tag,2,,USD,second expense",
			)
			assert.Equal(t, 2, preview.ImportableCount)
			assert.Empty(t, preview.RejectedRows)
			assert.Empty(t, preview.DuplicateRows)
			assert.Equal(t, []string{categoryName}, preview.WouldCreateCategories)
			assert.Equal(t, []string{"first-tag", "second-tag"}, preview.WouldCreateTags)
		})

		t.Run("ignores an unselected conflicting category row", func(t *testing.T) {
			service, tenants, _, _ := makeImportService(t)
			actorUserID := "user-" + fake.UUID().V4()
			tenant := makeTenant(t, tenants, actorUserID)
			selectedAccount := "selected-" + fake.Lorem().Word()
			unselectedAccount := "unselected-" + fake.Lorem().Word()
			categoryName := "category-" + fake.Lorem().Word()
			preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
				ActorUserID:          actorUserID,
				TenantID:             tenant.ID,
				ImportType:           CSVImportTypeTransactions,
				SelectedAccountNames: []string{selectedAccount},
				CSV: transactionCSV(
					"29.05.26,"+unselectedAccount+","+categoryName+",unselected-tag,,1,USD,income",
					"30.05.26,"+selectedAccount+","+categoryName+",selected-tag,1,,USD,expense",
				),
			})
			require.NoError(t, err)
			assert.Equal(t, 1, preview.ImportableCount)
			assert.Empty(t, preview.RejectedRows)
			assert.Empty(t, preview.DuplicateRows)
			assert.Equal(t, []string{selectedAccount}, preview.WouldCreateAccounts)
			assert.Equal(t, []string{categoryName}, preview.WouldCreateCategories)
			assert.Equal(t, []string{"selected-tag"}, preview.WouldCreateTags)
		})
	})

	t.Run("uses fallback descriptions consistently for preview duplicates execution and audit", func(t *testing.T) {
		fake := faker.New()
		service, tenants, _, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		accountName := "wallet-" + fake.Lorem().Word()
		csv := "Date,Account,Category,Tags,Expense amount,Income amount,Currency\n" +
			"29.05.26," + accountName + ",,,1,,USD\n"
		preview, result := previewAndRun(t, service, actorUserID, tenant.ID, csv)
		assert.Equal(t, 1, preview.ImportableCount)
		assert.Equal(t, 1, result.ImportedCount)
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		assert.Equal(t, "n/a", transactions[0].Description)

		duplicatePreview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			CSV:         csv,
		})
		require.NoError(t, err)
		assert.Zero(t, duplicatePreview.ImportableCount)
		assert.Len(t, duplicatePreview.DuplicateRows, 1)
	})

	t.Run("imports atomically with account category and tags then records final audit", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		accountName := "wallet-" + fake.Lorem().Word()
		categoryName := "meals-" + fake.Lorem().Word()
		preview, result := previewAndRun(t, service, actorUserID, tenant.ID, transactionCSV(
			"29.05.26,"+accountName+","+categoryName+",\"team, travel\",\"8\u00a0300,00\",,pln,lunch",
		))
		assert.Equal(t, []string{accountName}, preview.WouldCreateAccounts)
		assert.Equal(t, []string{categoryName}, preview.WouldCreateCategories)
		assert.Equal(t, []string{"team", "travel"}, preview.WouldCreateTags)
		assert.Equal(t, 1, result.ImportedCount)
		accounts, err := catalog.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "PLN", accounts[0].Currency)
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, transactions, 1)
		assert.Equal(t, int64(-830000), transactions[0].AmountMinor)
		assert.Len(t, transactions[0].TagIDs, 2)
		audit, err := service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, CSVImportStatusCompleted, audit.Status)
		assert.Equal(t, 1, audit.ImportedCount)
		require.Len(t, audit.RowOutcomes, 1)
		assert.Equal(t, domain.CSVImportRowOutcomeImported, audit.RowOutcomes[0].Status)
	})

	t.Run("rejects currency and category conflicts without partial catalog writes", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, _ := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		category, err := catalog.CreateCategory(t.Context(), CreateCategoryParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "income-" + fake.Lorem().Word(),
			Kind:        domain.CategoryKindIncome,
		})
		require.NoError(t, err)
		account, err := catalog.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "existing-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		rollbackAccount := "rollback-" + fake.Lorem().Word()
		newAccount := "new-" + fake.Lorem().Word()
		preview, result := previewAndRun(t, service, actorUserID, tenant.ID, transactionCSV(
			"29.05.26,"+account.Name+",,,1,,EUR,wrong currency",
			"29.05.26,"+rollbackAccount+","+category.Name+",,1,,USD,wrong category",
			"30.05.26,"+newAccount+",,,1,,USD,first valid currency",
			"31.05.26,"+newAccount+",,,1,,EUR,later currency conflict",
		))
		assert.Equal(t, 1, preview.ImportableCount)
		assert.Len(t, preview.RejectedRows, 3)
		assert.Equal(t, 1, result.ImportedCount)
		assert.Len(t, result.RejectedRows, 3)
		accounts, err := catalog.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 2)
		assert.Equal(t, account.ID, accounts[0].ID)
		assert.Equal(t, newAccount, accounts[1].Name)
		assert.Equal(t, "USD", accounts[1].Currency)
	})

	t.Run("persists duplicate rejection and makes retries and completed runs no-ops", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		account, err := catalog.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "wallet-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		effectiveAt, err := parseCSVImportDate("29.05.26")
		require.NoError(t, err)
		_, err = ledger.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -100,
			Currency:    "USD",
			Description: "duplicate",
			EffectiveAt: effectiveAt,
		})
		require.NoError(t, err)
		preview, result := previewAndRun(t, service, actorUserID, tenant.ID, transactionCSV(
			"29.05.26,"+account.Name+",,,1,,USD,duplicate",
			"29.05.26,"+account.Name+",,,2,,USD,accepted",
		))
		assert.Equal(t, 1, preview.ImportableCount)
		assert.Len(t, preview.DuplicateRows, 1)
		assert.Equal(t, 1, result.ImportedCount)
		assert.Len(t, result.RejectedRows, 1)
		retried, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: preview.ImportID})
		require.NoError(t, err)
		assert.Equal(t, result, retried)
		transactions, err := ledger.ListTransactions(t.Context(), ListTransactionsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Len(t, transactions, 2)
		audit, err := service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, audit.ImportedCount)
		assert.Len(t, audit.RejectedRows, 1)
		assert.Len(t, audit.RowOutcomes, 2)
		recovered, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		assert.Equal(t, preview.ImportID, recovered.ImportID)
		assert.Equal(t, audit.JobID, recovered.JobID)
		assert.Equal(t, CSVImportJobTypeTransactions, recovered.JobType)

		recent, err := service.ListRecentCSVImportAudits(t.Context(), ListRecentCSVImportAuditsParams{
			ActorUserID:        actorUserID,
			TenantID:           tenant.ID,
			ExpectedImportType: CSVImportTypeTransactions,
		})
		require.NoError(t, err)
		require.Len(t, recent, 1)
		assert.Equal(t, audit, recent[0])
	})

	t.Run("reports zero importable rows when every row is rejected or duplicate", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, ledger := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		account, err := catalog.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			Name:        "wallet-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)
		effectiveAt, err := parseCSVImportDate("29.05.26")
		require.NoError(t, err)
		_, err = ledger.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKindRegular,
			AmountMinor: -100,
			Currency:    "USD",
			Description: "duplicate",
			EffectiveAt: effectiveAt,
		})
		require.NoError(t, err)

		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			CSV: transactionCSV(
				"invalid,"+account.Name+",,,1,,USD,invalid date",
				"29.05.26,"+account.Name+",,,1,,USD,duplicate",
			),
		})
		require.NoError(t, err)
		assert.Zero(t, preview.ImportableCount)
		assert.Len(t, preview.RejectedRows, 1)
		assert.Len(t, preview.DuplicateRows, 1)
	})

	t.Run("keeps account-only imports independently valid", func(t *testing.T) {
		fake := faker.New()
		service, tenants, catalog, _ := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeAccounts,
			FileName:    "accounts.csv",
			CSV:         "name,currency,kind\nwallet,EUR,manual\ninvalid,,\n",
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"wallet"}, preview.WouldCreateAccounts)
		assert.Len(t, preview.RejectedRows, 1)
		service.csvImportJobEnqueuer = &recordingCSVJobEnqueuer{
			jobID:   "job-" + fake.UUID().V4(),
			jobType: CSVImportJobTypeAccounts,
		}
		confirmed, err := service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.NoError(t, err)
		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID:        actorUserID,
			ImportID:           preview.ImportID,
			ExpectedImportType: CSVImportTypeTransactions,
		})
		require.ErrorIs(t, err, ErrCSVImportTypeMismatch)
		_, err = service.GetCSVImportAudit(t.Context(), GetCSVImportAuditParams{
			ActorUserID:        actorUserID,
			TenantID:           tenant.ID,
			ImportID:           preview.ImportID,
			ExpectedImportType: CSVImportTypeTransactions,
		})
		require.ErrorIs(t, err, ErrCSVImportTypeMismatch)
		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.ErrorIs(t, err, ErrCSVImportAlreadyConfirmed)
		result, err := service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{
			ImportID: preview.ImportID,
			JobID:    confirmed.JobID,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ImportedCount)
		assert.Len(t, result.RejectedRows, 1)
		accounts, err := catalog.ListAccounts(t.Context(), ListAccountsParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Len(t, accounts, 1)
	})

	t.Run("requires confirmation setup and a confirmed status before a transaction run", func(t *testing.T) {
		fake := faker.New()
		service, tenants, _, _ := makeImportService(t)
		actorUserID := "user-" + fake.UUID().V4()
		tenant := makeTenant(t, tenants, actorUserID)
		preview, err := service.PreviewCSVImport(t.Context(), PreviewCSVImportParams{
			ActorUserID: actorUserID,
			TenantID:    tenant.ID,
			ImportType:  CSVImportTypeTransactions,
			CSV:         transactionCSV("29.05.26,wallet,,,1,,USD,purchase"),
		})
		require.NoError(t, err)
		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.EqualError(t, err, "csv import job enqueuer is required")
		_, err = service.RunCSVImportJob(t.Context(), RunCSVImportJobParams{ImportID: preview.ImportID})
		require.ErrorContains(t, err, "not runnable")
		service.csvImportJobEnqueuer = &recordingCSVJobEnqueuer{err: assert.AnError}
		_, err = service.ConfirmCSVImport(t.Context(), ConfirmCSVImportParams{
			ActorUserID: actorUserID,
			ImportID:    preview.ImportID,
		})
		require.ErrorIs(t, err, assert.AnError)
	})
}
