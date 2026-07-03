package finance

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
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
