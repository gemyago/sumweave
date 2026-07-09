package finance

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/google/uuid"
)

var (
	ErrCSVImportAlreadyConfirmed = errors.New("csv import already confirmed")
	ErrCSVImportAlreadyCompleted = errors.New("csv import already completed")
)

type csvImportFocusedStore interface {
	IsTenantMember(ctx context.Context, tenantID string, userID string) (bool, error)
	SaveCSVImport(ctx context.Context, record domain.CSVImportRecord) (domain.CSVImportRecord, error)
	GetCSVImport(ctx context.Context, importID string) (*domain.CSVImportRecord, error)
	ListAccounts(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Account, error)
	ListTransactions(
		ctx context.Context,
		tenantID string,
		accountID string,
		source domain.TransactionSource,
		status domain.TransactionStatus,
		includeHidden bool,
		page ...persistence.ListTransactionsPage,
	) ([]domain.Transaction, error)
	ListCategories(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Category, error)
	ListTags(ctx context.Context, tenantID string, includeHidden bool) ([]domain.Tag, error)
}

type csvImportCatalogService interface {
	CreateAccount(ctx context.Context, params CreateAccountParams) (domain.Account, error)
	CreateCategory(ctx context.Context, params CreateCategoryParams) (domain.Category, error)
	CreateTag(ctx context.Context, params CreateTagParams) (domain.Tag, error)
}

type csvImportLedgerService interface {
	RecordTransaction(ctx context.Context, params RecordTransactionParams) (domain.Transaction, error)
}

type CSVImportService struct {
	store                csvImportFocusedStore
	access               *accessGuard
	catalog              csvImportCatalogService
	ledger               csvImportLedgerService
	now                  func() time.Time
	newID                func() string
	csvImportJobEnqueuer CSVImportJobEnqueuer
}

const csvImportFirstDataRow = 2

type CSVImportServiceOption func(*CSVImportService)

func WithCSVImportServiceNow(now func() time.Time) CSVImportServiceOption {
	return func(service *CSVImportService) {
		service.now = now
	}
}

func WithCSVImportServiceIDGenerator(newID func() string) CSVImportServiceOption {
	return func(service *CSVImportService) {
		service.newID = newID
	}
}

func WithCSVImportServiceJobEnqueuer(enqueuer CSVImportJobEnqueuer) CSVImportServiceOption {
	return func(service *CSVImportService) {
		service.csvImportJobEnqueuer = enqueuer
	}
}

func NewCSVImportService(
	store csvImportFocusedStore,
	catalog csvImportCatalogService,
	ledger csvImportLedgerService,
	opts ...CSVImportServiceOption,
) *CSVImportService {
	service := &CSVImportService{
		store:   store,
		access:  newAccessGuard(store),
		catalog: catalog,
		ledger:  ledger,
		now:     func() time.Time { return time.Now().UTC() },
		newID:   uuid.NewString,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (s *CSVImportService) PreviewCSVImport(
	ctx context.Context,
	params PreviewCSVImportParams,
) (CSVImportPreview, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return CSVImportPreview{}, err
	}
	rows, readErr := readCSVRows(params.CSV)
	if readErr != nil {
		return CSVImportPreview{}, readErr
	}
	if len(rows) == 0 {
		return CSVImportPreview{}, errors.New("csv import requires at least one row")
	}
	headers := normalizeHeaders(rows[0])
	mapping := defaultCSVImportMapping(params.ImportType, headers)
	preview := CSVImportPreview{ImportID: s.newID(), ImportType: params.ImportType, Headers: headers, Mapping: mapping}
	previewErr := s.populateCSVImportPreview(ctx, params, rows[1:], &preview)
	if previewErr != nil {
		return CSVImportPreview{}, previewErr
	}
	now := s.now().UTC()
	_, saveErr := s.store.SaveCSVImport(ctx, domain.CSVImportRecord{
		ID:                    preview.ImportID,
		TenantID:              strings.TrimSpace(params.TenantID),
		Type:                  params.ImportType,
		Status:                domain.CSVImportStatusPreviewed,
		FileName:              strings.TrimSpace(params.FileName),
		RawCSV:                params.CSV,
		Headers:               preview.Headers,
		Mapping:               preview.Mapping,
		DuplicateRows:         preview.DuplicateRows,
		RejectedRows:          preview.RejectedRows,
		WouldCreateAccounts:   preview.WouldCreateAccounts,
		WouldCreateCategories: preview.WouldCreateCategories,
		WouldCreateTags:       preview.WouldCreateTags,
		CreatedAt:             now,
		UpdatedAt:             now,
	})
	if saveErr != nil {
		return CSVImportPreview{}, saveErr
	}
	return preview, nil
}

func (s *CSVImportService) ConfirmCSVImport(
	ctx context.Context,
	params ConfirmCSVImportParams,
) (CSVImportConfirmation, error) {
	record, getErr := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if getErr != nil {
		return CSVImportConfirmation{}, getErr
	}
	if err := s.access.requireTenantMember(ctx, record.TenantID, params.ActorUserID); err != nil {
		return CSVImportConfirmation{}, err
	}
	if s.csvImportJobEnqueuer == nil {
		return CSVImportConfirmation{}, errors.New("csv import job enqueuer is required")
	}
	switch record.Status {
	case domain.CSVImportStatusPreviewed:
	case domain.CSVImportStatusConfirmed:
		return CSVImportConfirmation{}, ErrCSVImportAlreadyConfirmed
	case domain.CSVImportStatusCompleted:
		return CSVImportConfirmation{}, ErrCSVImportAlreadyCompleted
	default:
		return CSVImportConfirmation{}, fmt.Errorf("csv import is not confirmable from status %q", record.Status)
	}
	jobType := CSVImportJobTypeTransactions
	if record.Type == domain.CSVImportTypeAccounts {
		jobType = CSVImportJobTypeAccounts
	}
	jobRef, enqueueErr := s.csvImportJobEnqueuer.EnqueueCSVImport(ctx, CSVImportJobRequest{
		JobType:  jobType,
		ImportID: record.ID,
		TenantID: record.TenantID,
		ActorID:  strings.TrimSpace(params.ActorUserID),
	})
	if enqueueErr != nil {
		return CSVImportConfirmation{}, fmt.Errorf("confirm csv import: %w", enqueueErr)
	}
	now := s.now().UTC()
	record.Status = domain.CSVImportStatusConfirmed
	record.Mapping = confirmedCSVImportMapping(record.Type, record.Headers, record.Mapping, params.Mapping)
	record.JobID = jobRef.ID
	record.ConfirmedByUserID = strings.TrimSpace(params.ActorUserID)
	record.ConfirmedAt = &now
	record.UpdatedAt = now
	_, saveErr := s.store.SaveCSVImport(ctx, *record)
	if saveErr != nil {
		return CSVImportConfirmation{}, saveErr
	}
	return CSVImportConfirmation{ImportID: record.ID, JobID: jobRef.ID, JobType: jobRef.JobType}, nil
}

func (s *CSVImportService) RunCSVImportJob(
	ctx context.Context,
	params RunCSVImportJobParams,
) (CSVImportRunResult, error) {
	record, getErr := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if getErr != nil {
		return CSVImportRunResult{}, getErr
	}
	rows, readErr := readCSVRows(record.RawCSV)
	if readErr != nil {
		return CSVImportRunResult{}, readErr
	}
	result := CSVImportRunResult{
		RejectedRows: append([]CSVImportRejectedRow{}, record.RejectedRows...),
	}
	resolvedMapping := confirmedCSVImportMapping(
		record.Type,
		record.Headers,
		defaultCSVImportMapping(record.Type, record.Headers),
		record.Mapping,
	)
	duplicateRows := append([]CSVImportRejectedRow{}, record.DuplicateRows...)
	result.RejectedRows = append(result.RejectedRows, duplicateRows...)
	seenRejected := map[int]struct{}{}
	for _, row := range result.RejectedRows {
		seenRejected[row.RowNumber] = struct{}{}
	}
	//nolint:nestif // The split account-vs-transaction flow is the public job contract.
	if record.Type == domain.CSVImportTypeAccounts {
		for index, row := range rows[1:] {
			rowNumber := index + csvImportFirstDataRow
			if _, rejected := seenRejected[rowNumber]; rejected {
				continue
			}
			rowErr := s.importAccountCSVRow(ctx, record, resolvedMapping, row)
			if rowErr != nil {
				result.RejectedRows = append(
					result.RejectedRows,
					CSVImportRejectedRow{RowNumber: rowNumber, Reason: rowErr.Error()},
				)
				continue
			}
			result.ImportedCount++
		}
	} else {
		for index, row := range rows[1:] {
			rowNumber := index + csvImportFirstDataRow
			if _, rejected := seenRejected[rowNumber]; rejected {
				continue
			}
			imported, rowErr := s.importTransactionCSVRow(ctx, record, resolvedMapping, row)
			if rowErr != nil {
				result.RejectedRows = append(
					result.RejectedRows,
					CSVImportRejectedRow{RowNumber: rowNumber, Reason: rowErr.Error()},
				)
				continue
			}
			if imported {
				result.ImportedCount++
			}
		}
	}
	now := s.now().UTC()
	record.Status = domain.CSVImportStatusCompleted
	record.JobID = strings.TrimSpace(params.JobID)
	record.ImportedCount = result.ImportedCount
	record.RejectedRows = result.RejectedRows
	record.CompletedAt = &now
	record.UpdatedAt = now
	_, saveErr := s.store.SaveCSVImport(ctx, *record)
	return result, saveErr
}

func (s *CSVImportService) GetCSVImportAudit(
	ctx context.Context,
	params GetCSVImportAuditParams,
) (CSVImportAudit, error) {
	record, getErr := s.store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if getErr != nil {
		return CSVImportAudit{}, getErr
	}
	if strings.TrimSpace(params.TenantID) != record.TenantID {
		return CSVImportAudit{}, ErrTenantAccessDenied
	}
	if err := s.access.requireTenantMember(ctx, record.TenantID, params.ActorUserID); err != nil {
		return CSVImportAudit{}, err
	}
	return CSVImportAudit{
		ImportID:          record.ID,
		TenantID:          record.TenantID,
		ImportType:        record.Type,
		Status:            record.Status,
		JobID:             record.JobID,
		ConfirmedByUserID: record.ConfirmedByUserID,
		ImportedCount:     record.ImportedCount,
		CreatedAt:         record.CreatedAt,
		ConfirmedAt:       record.ConfirmedAt,
		CompletedAt:       record.CompletedAt,
	}, nil
}

//nolint:gocognit,funlen
func (s *CSVImportService) populateCSVImportPreview(
	ctx context.Context,
	params PreviewCSVImportParams,
	rows [][]string,
	preview *CSVImportPreview,
) error {
	accounts, err := s.store.ListAccounts(ctx, params.TenantID, true)
	if err != nil {
		return err
	}
	transactions, err := s.store.ListTransactions(ctx, params.TenantID, "", "", "", true)
	if err != nil {
		return err
	}
	categories, err := s.store.ListCategories(ctx, params.TenantID, true)
	if err != nil {
		return err
	}
	tags, err := s.store.ListTags(ctx, params.TenantID, true)
	if err != nil {
		return err
	}
	accountNames := setFromAccounts(accounts)
	categoryNames := setFromCategories(categories)
	tagNames := setFromTags(tags)
	for index, row := range rows {
		rowNumber := index + csvImportFirstDataRow
		if params.ImportType == CSVImportTypeAccounts {
			name := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldName)
			currency := strings.ToUpper(
				csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldCurrency),
			)
			kind := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldKind)
			if name == "" || currency == "" || kind == "" {
				preview.RejectedRows = append(
					preview.RejectedRows,
					CSVImportRejectedRow{
						RowNumber: rowNumber,
						Reason:    "account row is missing required fields",
					},
				)
				continue
			}
			preview.WouldCreateAccounts = appendMissing(preview.WouldCreateAccounts, name)
			continue
		}
		accountName := csvImportMappedValue(
			preview.Headers,
			preview.Mapping,
			row,
			csvImportFieldAccountName,
		)
		currency := strings.ToUpper(
			csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldCurrency),
		)
		effectiveAt := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldEffectiveAt)
		amount := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldAmountMinor)
		description := csvImportMappedValue(
			preview.Headers,
			preview.Mapping,
			row,
			csvImportFieldDescription,
		)
		categoryName := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldCategory)
		tagName := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldTag)
		status := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldStatus)
		parsedDate, dateErr := time.Parse(time.DateOnly, effectiveAt)
		parsedAmount, amountErr := strconv.ParseInt(amount, 10, 64)
		if accountName == "" || currency == "" || description == "" || status == "" ||
			dateErr != nil || amountErr != nil {
			preview.RejectedRows = append(
				preview.RejectedRows,
				CSVImportRejectedRow{RowNumber: rowNumber, Reason: "transaction row is invalid"},
			)
			continue
		}
		if _, ok := accountNames[accountName]; !ok {
			preview.WouldCreateAccounts = appendMissing(preview.WouldCreateAccounts, accountName)
		}
		if categoryName != "" {
			if _, ok := categoryNames[categoryName]; !ok {
				preview.WouldCreateCategories = appendMissing(
					preview.WouldCreateCategories,
					categoryName,
				)
			}
		}
		if tagName != "" {
			if _, ok := tagNames[tagName]; !ok {
				preview.WouldCreateTags = appendMissing(preview.WouldCreateTags, tagName)
			}
		}
		for _, transaction := range transactions {
			if transaction.Currency == currency &&
				transaction.AmountMinor == parsedAmount &&
				transaction.Description == description &&
				transaction.EffectiveAt.Format(time.DateOnly) == parsedDate.Format(time.DateOnly) {
				preview.DuplicateRows = append(
					preview.DuplicateRows,
					CSVImportRejectedRow{RowNumber: rowNumber, Reason: "duplicate transaction"},
				)
				break
			}
		}
	}
	slices.Sort(preview.WouldCreateAccounts)
	slices.Sort(preview.WouldCreateCategories)
	slices.Sort(preview.WouldCreateTags)
	return nil
}

func (s *CSVImportService) importAccountCSVRow(
	ctx context.Context,
	record *domain.CSVImportRecord,
	mapping map[string]string,
	row []string,
) error {
	_, err := s.catalog.CreateAccount(ctx, CreateAccountParams{
		ActorUserID: record.ConfirmedByUserID,
		TenantID:    record.TenantID,
		Name:        csvImportMappedValue(record.Headers, mapping, row, csvImportFieldName),
		Currency:    strings.ToUpper(csvImportMappedValue(record.Headers, mapping, row, csvImportFieldCurrency)),
		Kind:        domain.AccountKind(csvImportMappedValue(record.Headers, mapping, row, csvImportFieldKind)),
	})
	return err
}

//nolint:gocognit,funlen
func (s *CSVImportService) importTransactionCSVRow(
	ctx context.Context,
	record *domain.CSVImportRecord,
	mapping map[string]string,
	row []string,
) (bool, error) {
	accountName := csvImportMappedValue(record.Headers, mapping, row, csvImportFieldAccountName)
	currency := strings.ToUpper(csvImportMappedValue(record.Headers, mapping, row, csvImportFieldCurrency))
	effectiveAt, err := time.Parse(
		time.DateOnly,
		csvImportMappedValue(record.Headers, mapping, row, csvImportFieldEffectiveAt),
	)
	if err != nil {
		return false, errors.New("transaction row is invalid")
	}
	amountMinor, err := strconv.ParseInt(
		csvImportMappedValue(record.Headers, mapping, row, csvImportFieldAmountMinor),
		10,
		64,
	)
	if err != nil {
		return false, errors.New("transaction row is invalid")
	}
	description := csvImportMappedValue(record.Headers, mapping, row, csvImportFieldDescription)
	categoryName := csvImportMappedValue(record.Headers, mapping, row, csvImportFieldCategory)
	tagName := csvImportMappedValue(record.Headers, mapping, row, csvImportFieldTag)
	status := domain.TransactionStatus(
		csvImportMappedValue(record.Headers, mapping, row, csvImportFieldStatus),
	)
	accounts, err := s.store.ListAccounts(ctx, record.TenantID, true)
	if err != nil {
		return false, err
	}
	account := findAccountByName(accounts, accountName)
	if account == nil {
		created, createErr := s.catalog.CreateAccount(ctx, CreateAccountParams{
			ActorUserID: record.ConfirmedByUserID,
			TenantID:    record.TenantID,
			Name:        accountName,
			Currency:    currency,
			Kind:        domain.AccountKindImported,
		})
		if createErr != nil {
			return false, createErr
		}
		account = &created
	}
	var categoryID string
	if categoryName != "" {
		categories, listErr := s.store.ListCategories(ctx, record.TenantID, true)
		if listErr != nil {
			return false, listErr
		}
		category := findCategoryByName(categories, categoryName)
		if category == nil {
			created, createErr := s.catalog.CreateCategory(ctx, CreateCategoryParams{
				ActorUserID: record.ConfirmedByUserID,
				TenantID:    record.TenantID,
				Name:        categoryName,
				Kind:        domain.CategoryKindExpense,
			})
			if createErr != nil {
				return false, createErr
			}
			category = &created
		}
		categoryID = category.ID
	}
	if tagName != "" {
		tags, listErr := s.store.ListTags(ctx, record.TenantID, true)
		if listErr != nil {
			return false, listErr
		}
		if findTagByName(tags, tagName) == nil {
			if _, createErr := s.catalog.CreateTag(ctx, CreateTagParams{
				ActorUserID: record.ConfirmedByUserID,
				TenantID:    record.TenantID,
				Name:        tagName,
			}); createErr != nil {
				return false, createErr
			}
		}
	}
	transactions, err := s.store.ListTransactions(ctx, record.TenantID, "", "", "", true)
	if err != nil {
		return false, err
	}
	for _, transaction := range transactions {
		if transaction.AccountID == account.ID &&
			transaction.Currency == currency &&
			transaction.AmountMinor == amountMinor &&
			transaction.Description == description &&
			transaction.EffectiveAt.Format(time.DateOnly) == effectiveAt.Format(time.DateOnly) {
			return false, errors.New("duplicate transaction")
		}
	}
	_, err = s.ledger.RecordTransaction(ctx, RecordTransactionParams{
		ActorUserID: record.ConfirmedByUserID,
		TenantID:    record.TenantID,
		AccountID:   account.ID,
		Source:      domain.TransactionSourceCSV,
		Status:      status,
		Kind:        domain.TransactionKindRegular,
		AmountMinor: amountMinor,
		Currency:    currency,
		Description: description,
		EffectiveAt: effectiveAt,
		CategoryID:  categoryID,
	})
	return err == nil, err
}
