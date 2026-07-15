package finance

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

var (
	ErrCSVImportAlreadyConfirmed = errors.New("csv import already confirmed")
	ErrCSVImportAlreadyCompleted = errors.New("csv import already completed")
	ErrCSVImportTypeMismatch     = errors.New("csv import type does not match this endpoint")
)

type csvImportFocusedStore interface {
	IsTenantMember(context.Context, string, string) (bool, error)
	SaveCSVImport(context.Context, domain.CSVImportRecord) (domain.CSVImportRecord, error)
	GetCSVImport(context.Context, string) (*domain.CSVImportRecord, error)
	ListAccounts(context.Context, string, bool) ([]domain.Account, error)
	ListTransactions(
		context.Context,
		string,
		string,
		domain.TransactionSource,
		domain.TransactionStatus,
		bool,
		...persistence.ListTransactionsPage,
	) ([]domain.Transaction, error)
	ListCategories(context.Context, string, bool) ([]domain.Category, error)
	ListTags(context.Context, string, bool) ([]domain.Tag, error)
}

type csvImportRowStore interface {
	ImportTransactionRow(context.Context, domain.CSVImportTransactionRow) (domain.CSVImportRowOutcome, error)
	ListCSVImportRowOutcomes(context.Context, string) ([]domain.CSVImportRowOutcome, error)
	ListRecentCSVImports(context.Context, string, domain.CSVImportType, int) ([]domain.CSVImportRecord, error)
}

type csvImportPreviewData struct {
	accounts     []domain.Account
	categories   []domain.Category
	tags         []domain.Tag
	transactions []domain.Transaction
}

type CSVImportService struct {
	store                csvImportFocusedStore
	rowStore             csvImportRowStore
	catalog              csvImportCatalogService
	ledger               csvImportLedgerService
	access               *accessGuard
	now                  func() time.Time
	newID                func() string
	csvImportJobEnqueuer CSVImportJobEnqueuer
}

type CSVImportServiceOption func(*CSVImportService)

func WithCSVImportServiceNow(now func() time.Time) CSVImportServiceOption {
	return func(s *CSVImportService) { s.now = now }
}
func WithCSVImportServiceIDGenerator(newID func() string) CSVImportServiceOption {
	return func(s *CSVImportService) { s.newID = newID }
}
func WithCSVImportServiceJobEnqueuer(enqueuer CSVImportJobEnqueuer) CSVImportServiceOption {
	return func(s *CSVImportService) { s.csvImportJobEnqueuer = enqueuer }
}
func WithCSVImportServiceRowStore(store csvImportRowStore) CSVImportServiceOption {
	return func(s *CSVImportService) { s.rowStore = store }
}

func NewCSVImportService(
	store csvImportFocusedStore,
	catalog csvImportCatalogService,
	ledger csvImportLedgerService,
	opts ...CSVImportServiceOption,
) *CSVImportService {
	s := &CSVImportService{
		store:   store,
		catalog: catalog,
		ledger:  ledger,
		access:  newAccessGuard(store),
		now:     time.Now,
		newID:   uuid.NewString,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Kept private to preserve the narrow constructor contract for older callers.
type csvImportCatalogService interface {
	CreateAccount(context.Context, CreateAccountParams) (domain.Account, error)
	CreateCategory(context.Context, CreateCategoryParams) (domain.Category, error)
	CreateTag(context.Context, CreateTagParams) (domain.Tag, error)
}
type csvImportLedgerService interface {
	RecordTransaction(context.Context, RecordTransactionParams) (domain.Transaction, error)
}

func (s *CSVImportService) PreviewCSVImport(
	ctx context.Context,
	params PreviewCSVImportParams,
) (CSVImportPreview, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return CSVImportPreview{}, err
	}
	if params.ImportType == CSVImportTypeAccounts {
		return s.previewLegacyCSVImport(ctx, params)
	}
	parsed, parseErr := parseFixedCSVWithAccountOptions(params.CSV)
	if parseErr != nil {
		return CSVImportPreview{}, parseErr
	}
	selectedAccountNames, accountOptions := selectedCSVImportAccounts(
		params.SelectedAccountNames,
		parsed.accountOptions,
	)
	preview := CSVImportPreview{
		ImportID:       s.newID(),
		ImportType:     params.ImportType,
		Headers:        parsed.headers,
		Mapping:        map[string]string{},
		AccountOptions: accountOptions,
		RejectedRows: filterCSVImportRejectedRows(
			parsed.rejected,
			parsed.accountNameByRow,
			selectedAccountNames,
		),
	}
	if populateErr := s.populatePreview(
		ctx,
		params.TenantID,
		parsed.rows,
		selectedAccountNames,
		&preview,
	); populateErr != nil {
		return CSVImportPreview{}, populateErr
	}
	now := s.now()
	_, saveErr := s.store.SaveCSVImport(
		ctx,
		domain.CSVImportRecord{
			ID:                    preview.ImportID,
			TenantID:              strings.TrimSpace(params.TenantID),
			Type:                  params.ImportType,
			Status:                domain.CSVImportStatusPreviewed,
			FileName:              strings.TrimSpace(params.FileName),
			RawCSV:                params.CSV,
			Headers:               parsed.headers,
			Mapping:               preview.Mapping,
			DuplicateRows:         preview.DuplicateRows,
			RejectedRows:          preview.RejectedRows,
			ImportableCount:       preview.ImportableCount,
			WouldCreateAccounts:   preview.WouldCreateAccounts,
			WouldCreateCategories: preview.WouldCreateCategories,
			WouldCreateTags:       preview.WouldCreateTags,
			AccountOptions:        preview.AccountOptions,
			SelectedAccountNames:  selectedCSVImportAccountNames(accountOptions),
			CreatedAt:             now,
			UpdatedAt:             now,
		},
	)
	if saveErr != nil {
		return CSVImportPreview{}, fmt.Errorf("save csv import preview: %w", saveErr)
	}
	return preview, nil
}

func (s *CSVImportService) populatePreview(
	ctx context.Context,
	tenantID string,
	rows []parsedCSVImportRow,
	selectedAccountNames map[string]struct{},
	preview *CSVImportPreview,
) error {
	data, err := s.loadCSVImportPreviewData(ctx, tenantID)
	if err != nil {
		return err
	}
	newAccountCurrencies := map[string]string{}
	proposedCategoryKinds := map[string]domain.CategoryKind{}
	for _, row := range rows {
		if !csvImportAccountSelected(selectedAccountNames, row.Account) {
			continue
		}
		previewTransactionRow(data, newAccountCurrencies, proposedCategoryKinds, row, preview)
	}
	slices.Sort(preview.WouldCreateAccounts)
	slices.Sort(preview.WouldCreateCategories)
	slices.Sort(preview.WouldCreateTags)
	return nil
}

func (s *CSVImportService) loadCSVImportPreviewData(
	ctx context.Context,
	tenantID string,
) (csvImportPreviewData, error) {
	accounts, err := s.store.ListAccounts(ctx, tenantID, true)
	if err != nil {
		return csvImportPreviewData{}, err
	}
	categories, err := s.store.ListCategories(ctx, tenantID, true)
	if err != nil {
		return csvImportPreviewData{}, err
	}
	tags, err := s.store.ListTags(ctx, tenantID, true)
	if err != nil {
		return csvImportPreviewData{}, err
	}
	transactions, err := s.store.ListTransactions(ctx, tenantID, "", "", "", true)
	if err != nil {
		return csvImportPreviewData{}, err
	}
	return csvImportPreviewData{accounts: accounts, categories: categories, tags: tags, transactions: transactions}, nil
}

func previewTransactionRow(
	data csvImportPreviewData,
	newAccountCurrencies map[string]string,
	proposedCategoryKinds map[string]domain.CategoryKind,
	row parsedCSVImportRow,
	preview *CSVImportPreview,
) {
	reason := validateCatalogRow(
		data.accounts,
		data.categories,
		data.tags,
		newAccountCurrencies,
		proposedCategoryKinds,
		row,
	)
	if reason != "" {
		preview.RejectedRows = append(preview.RejectedRows, csvImportRowDiagnostic(row.RowNumber, reason))
		return
	}
	if row.Category != "" && categoryByName(data.categories, row.Category) == nil {
		proposedCategoryKinds[row.Category] = categoryKindForAmount(row.AmountMinor)
	}
	appendCSVImportPreviewCreations(data, newAccountCurrencies, row, preview)
	if isDuplicateCSVImportTransaction(data.transactions, data.accounts, row) {
		preview.DuplicateRows = append(
			preview.DuplicateRows,
			csvImportRowDiagnostic(row.RowNumber, "duplicate transaction"),
		)
		return
	}
	preview.ImportableCount++
}

func appendCSVImportPreviewCreations(
	data csvImportPreviewData,
	newAccountCurrencies map[string]string,
	row parsedCSVImportRow,
	preview *CSVImportPreview,
) {
	if accountByName(data.accounts, row.Account) == nil {
		preview.WouldCreateAccounts = appendUnique(preview.WouldCreateAccounts, row.Account)
		newAccountCurrencies[row.Account] = row.Currency
	}
	if row.Category != "" && categoryByName(data.categories, row.Category) == nil {
		preview.WouldCreateCategories = appendUnique(preview.WouldCreateCategories, row.Category)
	}
	for _, tag := range row.Tags {
		if tagByName(data.tags, tag) == nil {
			preview.WouldCreateTags = appendUnique(preview.WouldCreateTags, tag)
		}
	}
}

func isDuplicateCSVImportTransaction(
	transactions []domain.Transaction,
	accounts []domain.Account,
	row parsedCSVImportRow,
) bool {
	for _, transaction := range transactions {
		if transaction.AccountID == accountIDByName(accounts, row.Account) && transaction.Currency == row.Currency &&
			transaction.AmountMinor == row.AmountMinor && transaction.Description == row.Description &&
			transaction.EffectiveAt.Equal(row.Date) {
			return true
		}
	}
	return false
}

func validateCatalogRow(
	accounts []domain.Account,
	categories []domain.Category,
	tags []domain.Tag,
	newCurrencies map[string]string,
	proposedCategoryKinds map[string]domain.CategoryKind,
	row parsedCSVImportRow,
) string {
	if account := accountByName(accounts, row.Account); account != nil {
		if account.Currency != row.Currency {
			return fmt.Sprintf("Account %q currency is %s, not %s", row.Account, account.Currency, row.Currency)
		}
	} else if current, ok := newCurrencies[row.Account]; ok && current != row.Currency {
		return fmt.Sprintf("Account %q first valid row currency is %s, not %s", row.Account, current, row.Currency)
	}
	if category := categoryByName(categories, row.Category); category != nil {
		expected := categoryKindForAmount(row.AmountMinor)
		if category.Kind != expected {
			return fmt.Sprintf(
				"Category %q is %s, incompatible with transaction direction",
				row.Category,
				category.Kind,
			)
		}
	} else if proposedKind, ok := proposedCategoryKinds[row.Category]; ok &&
		proposedKind != categoryKindForAmount(row.AmountMinor) {
		return fmt.Sprintf(
			"Category %q is %s, incompatible with transaction direction",
			row.Category,
			proposedKind,
		)
	}
	if ambiguousAccount(accounts, row.Account) || ambiguousCategory(categories, row.Category) {
		return "catalog name is ambiguous"
	}
	for _, tag := range row.Tags {
		if ambiguousTag(tags, tag) {
			return fmt.Sprintf("Tag %q is ambiguous", tag)
		}
	}
	return ""
}

func (s *CSVImportService) ConfirmCSVImport(
	ctx context.Context,
	params ConfirmCSVImportParams,
) (CSVImportConfirmation, error) {
	record, err := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportConfirmation{}, err
	}
	if accessErr := s.access.requireTenantMember(ctx, record.TenantID, params.ActorUserID); accessErr != nil {
		return CSVImportConfirmation{}, accessErr
	}
	if params.ExpectedImportType != "" && record.Type != params.ExpectedImportType {
		return CSVImportConfirmation{}, ErrCSVImportTypeMismatch
	}
	if record.Status == domain.CSVImportStatusCompleted {
		return csvImportConfirmationFromRecord(record), nil
	}
	if record.Status == domain.CSVImportStatusConfirmed || record.Status == domain.CSVImportStatusRunning {
		if record.Type == CSVImportTypeAccounts {
			return CSVImportConfirmation{}, ErrCSVImportAlreadyConfirmed
		}
		return csvImportConfirmationFromRecord(record), nil
	}
	if record.Status != domain.CSVImportStatusPreviewed {
		return CSVImportConfirmation{}, fmt.Errorf("csv import is not confirmable from status %q", record.Status)
	}
	if s.csvImportJobEnqueuer == nil {
		return CSVImportConfirmation{}, errors.New("csv import job enqueuer is required")
	}
	now := s.now()
	record.Status = domain.CSVImportStatusConfirmed
	record.ConfirmedByUserID = strings.TrimSpace(params.ActorUserID)
	record.ConfirmedAt = &now
	record.UpdatedAt = now
	if _, err = s.store.SaveCSVImport(ctx, *record); err != nil {
		return CSVImportConfirmation{}, fmt.Errorf("persist confirmed csv import: %w", err)
	}
	jobType := CSVImportJobTypeTransactions
	if record.Type == CSVImportTypeAccounts {
		jobType = CSVImportJobTypeAccounts
	}
	job, err := s.csvImportJobEnqueuer.EnqueueCSVImport(
		ctx,
		CSVImportJobRequest{
			JobType:        jobType,
			ImportID:       record.ID,
			TenantID:       record.TenantID,
			ActorID:        record.ConfirmedByUserID,
			IdempotencyKey: "finance.csv-import:" + record.ID,
		},
	)
	if err != nil {
		return CSVImportConfirmation{}, fmt.Errorf("enqueue confirmed csv import: %w", err)
	}
	if record.JobID == "" {
		record.JobID = job.ID
		record.UpdatedAt = s.now()
		if _, err = s.store.SaveCSVImport(ctx, *record); err != nil {
			return CSVImportConfirmation{}, fmt.Errorf("save csv import job reference: %w", err)
		}
	}
	return CSVImportConfirmation{ImportID: record.ID, JobID: job.ID, JobType: job.JobType}, nil
}

func csvImportConfirmationFromRecord(record *domain.CSVImportRecord) CSVImportConfirmation {
	jobType := CSVImportJobTypeTransactions
	if record.Type == CSVImportTypeAccounts {
		jobType = CSVImportJobTypeAccounts
	}
	return CSVImportConfirmation{ImportID: record.ID, JobID: record.JobID, JobType: jobType}
}

func (s *CSVImportService) RunCSVImportJob(
	ctx context.Context,
	params RunCSVImportJobParams,
) (CSVImportRunResult, error) {
	record, err := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportRunResult{}, err
	}
	if record.Status == domain.CSVImportStatusCompleted {
		return s.runResultFromRecord(record)
	}
	if record.Status != domain.CSVImportStatusConfirmed && record.Status != domain.CSVImportStatusRunning {
		return CSVImportRunResult{}, fmt.Errorf("csv import is not runnable from status %q", record.Status)
	}
	record.Status = domain.CSVImportStatusRunning
	record.UpdatedAt = s.now()
	if _, saveErr := s.store.SaveCSVImport(ctx, *record); saveErr != nil {
		return CSVImportRunResult{}, saveErr
	}
	if record.Type == CSVImportTypeAccounts {
		return s.runLegacyAccountImport(ctx, record, params)
	}
	parsed, err := parseFixedCSVWithAccountOptions(record.RawCSV)
	if err != nil {
		return CSVImportRunResult{}, err
	}
	selectedAccountNames := selectedAccountNameSet(record.SelectedAccountNames)
	if record.SelectedAccountNames == nil {
		selectedAccountNames, _ = selectedCSVImportAccounts(nil, parsed.accountOptions)
	}
	result := CSVImportRunResult{RejectedRows: filterCSVImportRejectedRows(
		parsed.rejected,
		parsed.accountNameByRow,
		selectedAccountNames,
	)}
	for _, row := range parsed.rows {
		if !csvImportAccountSelected(selectedAccountNames, row.Account) ||
			containsOutcomeRejected(result.RejectedRows, row.RowNumber) {
			continue
		}
		if s.rowStore == nil {
			return CSVImportRunResult{}, errors.New("csv import row store is required")
		}
		outcome, rowErr := s.rowStore.ImportTransactionRow(
			ctx,
			domain.CSVImportTransactionRow{
				ImportID:     record.ID,
				RowNumber:    row.RowNumber,
				TenantID:     record.TenantID,
				ActorUserID:  record.ConfirmedByUserID,
				AccountName:  row.Account,
				CategoryName: row.Category,
				TagNames:     row.Tags,
				Currency:     row.Currency,
				Description:  row.Description,
				AmountMinor:  row.AmountMinor,
				EffectiveAt:  row.Date,
			},
		)
		if rowErr != nil {
			result.RejectedRows = append(
				result.RejectedRows,
				csvImportRowDiagnostic(row.RowNumber, rowErr.Error()),
			)
			continue
		}
		switch outcome.Status {
		case domain.CSVImportRowOutcomeImported:
			result.ImportedCount++
		case domain.CSVImportRowOutcomeRejected:
			result.RejectedRows = append(
				result.RejectedRows,
				csvImportRowDiagnostic(row.RowNumber, outcome.Reason),
			)
		}
	}
	now := s.now()
	record.Status = domain.CSVImportStatusCompleted
	record.JobID = firstNonEmpty(params.JobID, record.JobID)
	record.ImportedCount = result.ImportedCount
	record.RejectedRows = result.RejectedRows
	record.CompletedAt = &now
	record.UpdatedAt = now
	_, err = s.store.SaveCSVImport(ctx, *record)
	return result, err
}

func (s *CSVImportService) runLegacyAccountImport(
	ctx context.Context,
	record *domain.CSVImportRecord,
	params RunCSVImportJobParams,
) (CSVImportRunResult, error) {
	rows, err := readCSVRows(record.RawCSV)
	if err != nil {
		return CSVImportRunResult{}, err
	}
	result := CSVImportRunResult{RejectedRows: append([]CSVImportRejectedRow{}, record.RejectedRows...)}
	for index, row := range rows[1:] {
		number := index + csvImportFirstDataRow
		if containsOutcomeRejected(result.RejectedRows, number) {
			continue
		}
		_, createErr := s.catalog.CreateAccount(
			ctx,
			CreateAccountParams{
				ActorUserID: record.ConfirmedByUserID,
				TenantID:    record.TenantID,
				Name:        csvImportMappedValue(record.Headers, record.Mapping, row, csvImportFieldName),
				Currency: strings.ToUpper(
					csvImportMappedValue(record.Headers, record.Mapping, row, csvImportFieldCurrency),
				),
				Kind: domain.AccountKind(
					csvImportMappedValue(record.Headers, record.Mapping, row, csvImportFieldKind),
				),
			},
		)
		if createErr != nil {
			result.RejectedRows = append(
				result.RejectedRows,
				csvImportRowDiagnostic(number, createErr.Error()),
			)
			continue
		}
		result.ImportedCount++
	}
	now := s.now()
	record.Status = domain.CSVImportStatusCompleted
	record.JobID = firstNonEmpty(params.JobID, record.JobID)
	record.ImportedCount = result.ImportedCount
	record.RejectedRows = result.RejectedRows
	record.CompletedAt = &now
	record.UpdatedAt = now
	_, err = s.store.SaveCSVImport(ctx, *record)
	return result, err
}

func (s *CSVImportService) runResultFromRecord(record *domain.CSVImportRecord) (CSVImportRunResult, error) {
	return CSVImportRunResult{ImportedCount: record.ImportedCount, RejectedRows: record.RejectedRows}, nil
}

func (s *CSVImportService) GetCSVImportAudit(
	ctx context.Context,
	params GetCSVImportAuditParams,
) (CSVImportAudit, error) {
	record, err := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportAudit{}, err
	}
	if strings.TrimSpace(params.TenantID) != record.TenantID {
		return CSVImportAudit{}, ErrTenantAccessDenied
	}
	if params.ExpectedImportType != "" && record.Type != params.ExpectedImportType {
		return CSVImportAudit{}, ErrCSVImportTypeMismatch
	}
	if accessErr := s.access.requireTenantMember(ctx, record.TenantID, params.ActorUserID); accessErr != nil {
		return CSVImportAudit{}, accessErr
	}
	audit := csvImportAuditFromRecord(record)
	if s.rowStore != nil {
		audit.RowOutcomes, err = s.rowStore.ListCSVImportRowOutcomes(ctx, record.ID)
		if err != nil {
			return CSVImportAudit{}, err
		}
	}
	return audit, nil
}

func (s *CSVImportService) ListRecentCSVImportAudits(
	ctx context.Context,
	params ListRecentCSVImportAuditsParams,
) ([]CSVImportAudit, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	if s.rowStore == nil {
		return nil, errors.New("csv import row store is required")
	}
	records, err := s.rowStore.ListRecentCSVImports(
		ctx,
		strings.TrimSpace(params.TenantID),
		params.ExpectedImportType,
		recentCSVImportAuditLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent csv import audits: %w", err)
	}
	items := make([]CSVImportAudit, 0, len(records))
	for index := range records {
		audit := csvImportAuditFromRecord(&records[index])
		audit.RowOutcomes, err = s.rowStore.ListCSVImportRowOutcomes(ctx, records[index].ID)
		if err != nil {
			return nil, fmt.Errorf("list csv import row outcomes: %w", err)
		}
		items = append(items, audit)
	}
	return items, nil
}

func csvImportAuditFromRecord(record *domain.CSVImportRecord) CSVImportAudit {
	return CSVImportAudit{
		ImportID:          record.ID,
		TenantID:          record.TenantID,
		ImportType:        record.Type,
		Status:            record.Status,
		JobID:             record.JobID,
		ConfirmedByUserID: record.ConfirmedByUserID,
		ImportedCount:     record.ImportedCount,
		RejectedRows:      record.RejectedRows,
		CreatedAt:         record.CreatedAt,
		ConfirmedAt:       record.ConfirmedAt,
		CompletedAt:       record.CompletedAt,
	}
}

// legacy preview keeps the independently supported account-import surface alive;
// transaction rows using it are intentionally rejected in favour of the fixed contract.
func (s *CSVImportService) previewLegacyCSVImport(
	ctx context.Context,
	params PreviewCSVImportParams,
) (CSVImportPreview, error) {
	rows, err := readCSVRows(params.CSV)
	if err != nil {
		return CSVImportPreview{}, err
	}
	if len(rows) == 0 {
		return CSVImportPreview{}, errors.New("csv import requires at least one row")
	}
	headers := normalizeHeaders(rows[0])
	mapping := map[string]string{}
	if params.ImportType == CSVImportTypeAccounts {
		for _, field := range []string{csvImportFieldName, csvImportFieldCurrency, csvImportFieldKind} {
			for _, header := range headers {
				if header == field {
					mapping[field] = header
				}
			}
		}
	} else {
		return CSVImportPreview{}, errors.New("transaction CSV must use the fixed Finance import headers")
	}
	preview := CSVImportPreview{ImportID: s.newID(), ImportType: params.ImportType, Headers: headers, Mapping: mapping}
	for i, row := range rows[1:] {
		name := csvImportMappedValue(headers, mapping, row, csvImportFieldName)
		currency := strings.ToUpper(csvImportMappedValue(headers, mapping, row, csvImportFieldCurrency))
		kind := csvImportMappedValue(headers, mapping, row, csvImportFieldKind)
		if name == "" || currency == "" || kind == "" {
			preview.RejectedRows = append(
				preview.RejectedRows,
				CSVImportRejectedRow{
					RowNumber: i + csvImportFirstDataRow,
					Reason:    "account row is missing required fields",
				},
			)
			continue
		}
		preview.WouldCreateAccounts = appendUnique(preview.WouldCreateAccounts, name)
	}
	now := s.now()
	_, err = s.store.SaveCSVImport(
		ctx,
		domain.CSVImportRecord{
			ID:                  preview.ImportID,
			TenantID:            strings.TrimSpace(params.TenantID),
			Type:                params.ImportType,
			Status:              domain.CSVImportStatusPreviewed,
			FileName:            strings.TrimSpace(params.FileName),
			RawCSV:              params.CSV,
			Headers:             headers,
			Mapping:             mapping,
			RejectedRows:        preview.RejectedRows,
			WouldCreateAccounts: preview.WouldCreateAccounts,
			CreatedAt:           now,
			UpdatedAt:           now,
		},
	)
	if err != nil {
		return CSVImportPreview{}, err
	}
	return preview, nil
}

func accountByName(items []domain.Account, name string) *domain.Account {
	var match *domain.Account
	for _, item := range items {
		if item.Name == name {
			matched := item
			match = &matched
		}
	}
	return match
}
func categoryByName(items []domain.Category, name string) *domain.Category {
	if name == "" {
		return nil
	}
	var match *domain.Category
	for _, item := range items {
		if item.Name == name {
			matched := item
			match = &matched
		}
	}
	return match
}
func tagByName(items []domain.Tag, name string) *domain.Tag {
	var match *domain.Tag
	for _, item := range items {
		if item.Name == name {
			matched := item
			match = &matched
		}
	}
	return match
}
func ambiguousAccount(items []domain.Account, name string) bool {
	count := 0
	for _, item := range items {
		if item.Name == name {
			count++
		}
	}
	return count > 1
}
func ambiguousCategory(items []domain.Category, name string) bool {
	if name == "" {
		return false
	}
	count := 0
	for _, item := range items {
		if item.Name == name {
			count++
		}
	}
	return count > 1
}
func ambiguousTag(items []domain.Tag, name string) bool {
	count := 0
	for _, item := range items {
		if item.Name == name {
			count++
		}
	}
	return count > 1
}
func accountIDByName(items []domain.Account, name string) string {
	account := accountByName(items, name)
	if account == nil {
		return ""
	}
	return account.ID
}
func appendUnique(items []string, value string) []string {
	if !slices.Contains(items, value) {
		return append(items, value)
	}
	return items
}

func selectedCSVImportAccounts(
	selectedAccountNames []string,
	accountOptions []CSVImportAccountOption,
) (map[string]struct{}, []CSVImportAccountOption) {
	selected := selectedAccountNameSet(selectedAccountNames)
	if selectedAccountNames == nil {
		selected = make(map[string]struct{}, len(accountOptions))
		for _, option := range accountOptions {
			selected[option.Name] = struct{}{}
		}
	}
	result := make([]CSVImportAccountOption, len(accountOptions))
	for index, option := range accountOptions {
		_, option.Selected = selected[option.Name]
		result[index] = option
	}
	return selected, result
}

func selectedCSVImportAccountNames(accountOptions []CSVImportAccountOption) []string {
	result := make([]string, 0, len(accountOptions))
	for _, option := range accountOptions {
		if option.Selected {
			result = append(result, option.Name)
		}
	}
	return result
}

func selectedAccountNameSet(names []string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		if normalized := strings.TrimSpace(name); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func csvImportAccountSelected(selectedAccountNames map[string]struct{}, accountName string) bool {
	_, selected := selectedAccountNames[accountName]
	return selected
}

func filterCSVImportRejectedRows(
	items []CSVImportRejectedRow,
	accountNameByRow map[int]string,
	selectedAccountNames map[string]struct{},
) []CSVImportRejectedRow {
	result := make([]CSVImportRejectedRow, 0, len(items))
	for _, item := range items {
		accountName, assignable := accountNameByRow[item.RowNumber]
		if !assignable || csvImportAccountSelected(selectedAccountNames, accountName) {
			result = append(result, item)
		}
	}
	return result
}
func categoryKindForAmount(amount int64) domain.CategoryKind {
	if amount < 0 {
		return domain.CategoryKindExpense
	}
	return domain.CategoryKindIncome
}
func containsOutcomeRejected(items []CSVImportRejectedRow, number int) bool {
	for _, item := range items {
		if item.RowNumber == number {
			return true
		}
	}
	return false
}
