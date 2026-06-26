//nolint:funlen,gocognit,govet,mnd,nestif // CSV import v0 intentionally keeps positional flow explicit.
package finance

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
)

const (
	CSVImportJobTypeTransactions = "finance.csv_import"
	CSVImportJobTypeAccounts     = "finance.account_import"
	csvImportFieldAccountName    = "accountName"
	csvImportFieldAmountMinor    = "amountMinor"
	csvImportFieldCategory       = "category"
	csvImportFieldCurrency       = "currency"
	csvImportFieldDescription    = "description"
	csvImportFieldEffectiveAt    = "effectiveAt"
	csvImportFieldKind           = "kind"
	csvImportFieldName           = "name"
	csvImportFieldStatus         = "status"
	csvImportFieldTag            = "tag"
)

type CSVImportType = domain.CSVImportType
type CSVImportStatus = domain.CSVImportStatus

const (
	CSVImportTypeTransactions = domain.CSVImportTypeTransactions
	CSVImportTypeAccounts     = domain.CSVImportTypeAccounts
	CSVImportStatusPreviewed  = domain.CSVImportStatusPreviewed
	CSVImportStatusConfirmed  = domain.CSVImportStatusConfirmed
	CSVImportStatusCompleted  = domain.CSVImportStatusCompleted
)

type CSVImportRejectedRow = domain.CSVImportRejectedRow

type csvImportStore interface {
	SaveCSVImport(
		ctx context.Context,
		record domain.CSVImportRecord,
	) (domain.CSVImportRecord, error)
	GetCSVImport(ctx context.Context, importID string) (*domain.CSVImportRecord, error)
}

type CSVImportJobEnqueuer interface {
	EnqueueCSVImport(ctx context.Context, request CSVImportJobRequest) (CSVImportJobRef, error)
}

type CSVImportJobRequest struct {
	JobType  string
	ImportID string
	TenantID string
	ActorID  string
}

type CSVImportJobRef struct {
	ID      string
	JobType string
}

type PreviewCSVImportParams struct {
	ActorUserID string
	TenantID    string
	ImportType  CSVImportType
	FileName    string
	CSV         string
}

type CSVImportPreview struct {
	ImportID              string
	ImportType            CSVImportType
	Headers               []string
	Mapping               map[string]string
	DuplicateRows         []CSVImportRejectedRow
	RejectedRows          []CSVImportRejectedRow
	WouldCreateAccounts   []string
	WouldCreateCategories []string
	WouldCreateTags       []string
}

type ConfirmCSVImportParams struct {
	ActorUserID string
	ImportID    string
	Mapping     map[string]string
}

type CSVImportConfirmation struct {
	ImportID string
	JobID    string
	JobType  string
}

type RunCSVImportJobParams struct {
	ImportID string
	JobID    string
}

type CSVImportRunResult struct {
	ImportedCount int
	RejectedRows  []CSVImportRejectedRow
}

type GetCSVImportAuditParams struct {
	ActorUserID string
	TenantID    string
	ImportID    string
}

type CSVImportAudit struct {
	ImportID          string
	TenantID          string
	ImportType        CSVImportType
	Status            CSVImportStatus
	JobID             string
	ConfirmedByUserID string
	ImportedCount     int
	CreatedAt         time.Time
	ConfirmedAt       *time.Time
	CompletedAt       *time.Time
}

func WithCSVImportJobEnqueuer(enqueuer CSVImportJobEnqueuer) ServiceOption {
	return func(service *Service) {
		service.csvImportJobEnqueuer = enqueuer
	}
}

func (s *Service) PreviewCSVImport(
	ctx context.Context,
	params PreviewCSVImportParams,
) (CSVImportPreview, error) { //nolint:govet // positional preview record construction keeps this flow explicit.
	if err := s.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return CSVImportPreview{}, err
	}
	rows, err := readCSVRows(params.CSV)
	if err != nil {
		return CSVImportPreview{}, err
	}
	if len(rows) == 0 {
		return CSVImportPreview{}, errors.New("csv import requires at least one row")
	}
	headers := normalizeHeaders(rows[0])
	mapping := defaultCSVImportMapping(params.ImportType, headers)
	preview := CSVImportPreview{
		ImportID:   s.newID(),
		ImportType: params.ImportType,
		Headers:    headers,
		Mapping:    mapping,
	}
	if err := s.populateCSVImportPreview(ctx, params, rows[1:], &preview); err != nil {
		return CSVImportPreview{}, err
	}
	now := s.now().UTC()
	if _, err := s.csvImportStore().SaveCSVImport(ctx, domain.CSVImportRecord{
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
	}); err != nil {
		return CSVImportPreview{}, err
	}
	return preview, nil
}

func (s *Service) ConfirmCSVImport(
	ctx context.Context,
	params ConfirmCSVImportParams,
) (CSVImportConfirmation, error) { //nolint:govet // confirmation response construction stays field-by-field for clarity.
	store := s.csvImportStore()
	record, err := store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportConfirmation{}, err
	}
	if err := s.requireTenantMember(ctx, record.TenantID, params.ActorUserID); err != nil {
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
	jobRef, err := s.csvImportJobEnqueuer.EnqueueCSVImport(ctx, CSVImportJobRequest{
		JobType:  jobType,
		ImportID: record.ID,
		TenantID: record.TenantID,
		ActorID:  strings.TrimSpace(params.ActorUserID),
	})
	if err != nil {
		return CSVImportConfirmation{}, fmt.Errorf("confirm csv import: %w", err)
	}
	now := s.now().UTC()
	record.Status = domain.CSVImportStatusConfirmed
	record.Mapping = confirmedCSVImportMapping(record.Type, record.Headers, record.Mapping, params.Mapping)
	record.JobID = jobRef.ID
	record.ConfirmedByUserID = strings.TrimSpace(params.ActorUserID)
	record.ConfirmedAt = &now
	record.UpdatedAt = now
	if _, err := store.SaveCSVImport(ctx, *record); err != nil {
		return CSVImportConfirmation{}, err
	}
	return CSVImportConfirmation{
		ImportID: record.ID,
		JobID:    jobRef.ID,
		JobType:  jobRef.JobType,
	}, nil
}

func (s *Service) RunCSVImportJob(
	ctx context.Context,
	params RunCSVImportJobParams,
) (CSVImportRunResult, error) { //nolint:govet,mnd,nestif // import job flow is intentionally linear and status-driven.
	store := s.csvImportStore()
	record, err := store.GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportRunResult{}, err
	}
	rows, err := readCSVRows(record.RawCSV)
	if err != nil {
		return CSVImportRunResult{}, err
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
	if record.Type == domain.CSVImportTypeAccounts {
		for index, row := range rows[1:] {
			rowNumber := index + 2
			if _, rejected := seenRejected[rowNumber]; rejected {
				continue
			}
			if err := s.importAccountCSVRow(ctx, record, resolvedMapping, row); err != nil {
				result.RejectedRows = append(
					result.RejectedRows,
					CSVImportRejectedRow{RowNumber: rowNumber, Reason: err.Error()},
				)
				continue
			}
			result.ImportedCount++
		}
	} else {
		for index, row := range rows[1:] {
			rowNumber := index + 2
			if _, rejected := seenRejected[rowNumber]; rejected {
				continue
			}
			imported, err := s.importTransactionCSVRow(ctx, record, resolvedMapping, row)
			if err != nil {
				result.RejectedRows = append(
					result.RejectedRows,
					CSVImportRejectedRow{RowNumber: rowNumber, Reason: err.Error()},
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
	_, err = store.SaveCSVImport(ctx, *record)
	return result, err
}

func (s *Service) GetCSVImportAudit(
	ctx context.Context,
	params GetCSVImportAuditParams,
) (CSVImportAudit, error) { //nolint:govet // audit response mapping stays explicit for API parity.
	record, err := s.csvImportStore().GetCSVImport(ctx, strings.TrimSpace(params.ImportID))
	if err != nil {
		return CSVImportAudit{}, err
	}
	if strings.TrimSpace(params.TenantID) != record.TenantID {
		return CSVImportAudit{}, ErrTenantAccessDenied
	}
	if err := s.requireTenantMember(ctx, record.TenantID, params.ActorUserID); err != nil {
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

func (s *Service) populateCSVImportPreview(
	ctx context.Context,
	params PreviewCSVImportParams,
	rows [][]string,
	preview *CSVImportPreview,
) error { //nolint:gocognit,funlen,mnd // CSV preview validation intentionally keeps each positional rule inline.
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
		rowNumber := index + 2
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
		effectiveAt := csvImportMappedValue(
			preview.Headers,
			preview.Mapping,
			row,
			csvImportFieldEffectiveAt,
		)
		amount := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldAmountMinor)
		description := csvImportMappedValue(
			preview.Headers,
			preview.Mapping,
			row,
			csvImportFieldDescription,
		)
		categoryName := csvImportMappedValue(
			preview.Headers,
			preview.Mapping,
			row,
			csvImportFieldCategory,
		)
		tagName := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldTag)
		status := csvImportMappedValue(preview.Headers, preview.Mapping, row, csvImportFieldStatus)
		parsedDate, dateErr := time.Parse(time.DateOnly, effectiveAt)
		parsedAmount, amountErr := strconv.ParseInt(amount, 10, 64)
		if accountName == "" || currency == "" || description == "" || status == "" ||
			dateErr != nil ||
			amountErr != nil {
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

func (s *Service) importAccountCSVRow(
	ctx context.Context,
	record *domain.CSVImportRecord,
	mapping map[string]string,
	row []string,
) error {
	_, err := s.CreateAccount(ctx, CreateAccountParams{
		ActorUserID: record.ConfirmedByUserID,
		TenantID:    record.TenantID,
		Name:        csvImportMappedValue(record.Headers, mapping, row, csvImportFieldName),
		Currency:    strings.ToUpper(csvImportMappedValue(record.Headers, mapping, row, csvImportFieldCurrency)),
		Kind:        domain.AccountKind(csvImportMappedValue(record.Headers, mapping, row, csvImportFieldKind)),
	})
	return err
}

func (s *Service) importTransactionCSVRow(
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
		created, createErr := s.CreateAccount(ctx, CreateAccountParams{
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
			created, createErr := s.CreateCategory(ctx, CreateCategoryParams{
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
			if _, createErr := s.CreateTag(ctx, CreateTagParams{
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
		if transaction.AccountID == account.ID && transaction.Currency == currency &&
			transaction.AmountMinor == amountMinor &&
			transaction.Description == description &&
			transaction.EffectiveAt.Format(time.DateOnly) == effectiveAt.Format(time.DateOnly) {
			return false, errors.New("duplicate transaction")
		}
	}
	_, err = s.RecordTransaction(ctx, RecordTransactionParams{
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

func readCSVRows(raw string) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	rows := make([][]string, 0)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv import: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func normalizeHeaders(headers []string) []string {
	result := make([]string, 0, len(headers))
	for _, header := range headers {
		result = append(result, strings.TrimSpace(header))
	}
	return result
}

func valueAt(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return row[index]
}

func setFromAccounts(accounts []domain.Account) map[string]struct{} {
	result := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		result[account.Name] = struct{}{}
	}
	return result
}

func setFromCategories(categories []domain.Category) map[string]struct{} {
	result := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		result[category.Name] = struct{}{}
	}
	return result
}

func setFromTags(tags []domain.Tag) map[string]struct{} {
	result := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		result[tag.Name] = struct{}{}
	}
	return result
}

func appendMissing(items []string, value string) []string {
	if value == "" || slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

func findAccountByName(accounts []domain.Account, name string) *domain.Account {
	for _, account := range accounts {
		if account.Name == name {
			copyAccount := account
			return &copyAccount
		}
	}
	return nil
}

func findCategoryByName(categories []domain.Category, name string) *domain.Category {
	for _, category := range categories {
		if category.Name == name {
			copyCategory := category
			return &copyCategory
		}
	}
	return nil
}

func findTagByName(tags []domain.Tag, name string) *domain.Tag {
	for _, tag := range tags {
		if tag.Name == name {
			copyTag := tag
			return &copyTag
		}
	}
	return nil
}

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	maps.Copy(result, input)
	return result
}

func defaultCSVImportMapping(importType CSVImportType, headers []string) map[string]string {
	result := make(map[string]string, len(headers))
	headerSet := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		headerSet[header] = struct{}{}
	}
	for _, field := range csvImportFields(importType) {
		if _, ok := headerSet[field]; ok {
			result[field] = field
		}
	}
	return result
}

func confirmedCSVImportMapping(
	importType CSVImportType,
	headers []string,
	base map[string]string,
	overrides map[string]string,
) map[string]string {
	resolved := cloneStringMap(base)
	validHeaders := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		validHeaders[header] = struct{}{}
	}
	for _, field := range csvImportFields(importType) {
		header := strings.TrimSpace(overrides[field])
		if header == "" {
			continue
		}
		if _, ok := validHeaders[header]; ok {
			resolved[field] = header
		}
	}
	return resolved
}

func csvImportFields(importType CSVImportType) []string {
	if importType == CSVImportTypeAccounts {
		return []string{csvImportFieldName, csvImportFieldCurrency, csvImportFieldKind}
	}
	return []string{
		csvImportFieldAccountName,
		csvImportFieldCurrency,
		csvImportFieldEffectiveAt,
		csvImportFieldAmountMinor,
		csvImportFieldDescription,
		csvImportFieldCategory,
		csvImportFieldTag,
		csvImportFieldStatus,
	}
}

func csvImportMappedValue(
	headers []string,
	mapping map[string]string,
	row []string,
	field string,
) string {
	mappedHeader := strings.TrimSpace(mapping[field])
	if mappedHeader == "" {
		mappedHeader = field
	}
	for index, header := range headers {
		if header == mappedHeader {
			return strings.TrimSpace(valueAt(row, index))
		}
	}
	return ""
}

func (s *Service) csvImportStore() csvImportStore { //nolint:ireturn
	return s.store
}
