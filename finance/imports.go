package finance

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gemyago/sumweave/finance/domain"
)

const (
	CSVImportJobTypeTransactions = "finance.csv_import"
	CSVImportJobTypeAccounts     = "finance.account_import"
	MaxCSVImportBytes            = 64 << 20
	MaxCSVImportRows             = 250_000
	maxCSVImportColumns          = 16
	maxCSVImportCellBytes        = 4 << 10
	csvImportFirstDataRow        = 2
	csvImportMinorScale          = 100
	csvImportFractionDigits      = 2
	recentCSVImportAuditLimit    = 10
)

// Legacy account-import keys remain private compatibility helpers. Transaction
// imports exclusively use the fixed header contract below.
const (
	csvImportFieldName      = "name"
	csvImportFieldCurrency  = "currency"
	csvImportFieldKind      = "kind"
	csvImportHeaderDate     = "Date"
	csvImportHeaderAccount  = "Account"
	csvImportHeaderCategory = "Category"
	csvImportHeaderTags     = "Tags"
	csvImportHeaderExpense  = "Expense amount"
	csvImportHeaderIncome   = "Income amount"
	csvImportHeaderCurrency = "Currency"
)

func fixedCSVHeaders() []string {
	return append(fixedCSVRequiredHeaders(), "Description")
}

func fixedCSVRequiredHeaders() []string {
	return []string{
		csvImportHeaderDate,
		csvImportHeaderAccount,
		csvImportHeaderCategory,
		csvImportHeaderTags,
		csvImportHeaderExpense,
		csvImportHeaderIncome,
		csvImportHeaderCurrency,
	}
}

type CSVImportType = domain.CSVImportType
type CSVImportStatus = domain.CSVImportStatus

const (
	CSVImportTypeTransactions = domain.CSVImportTypeTransactions
	CSVImportTypeAccounts     = domain.CSVImportTypeAccounts
	CSVImportStatusPreviewed  = domain.CSVImportStatusPreviewed
	CSVImportStatusConfirmed  = domain.CSVImportStatusConfirmed
	CSVImportStatusRunning    = domain.CSVImportStatusRunning
	CSVImportStatusCompleted  = domain.CSVImportStatusCompleted
)

type CSVImportRejectedRow = domain.CSVImportRejectedRow
type CSVImportAccountOption = domain.CSVImportAccountOption

var ErrInvalidCSVImport = errors.New("invalid csv import")

type PreviewCSVImportParams struct {
	ActorUserID, TenantID string
	ImportType            CSVImportType
	FileName, CSV         string
	// SelectedAccountNames is nil for the initial all-accounts preview. An
	// explicitly empty slice intentionally selects no accounts.
	SelectedAccountNames []string
}
type CSVImportPreview struct {
	ImportID                                                    string
	ImportType                                                  CSVImportType
	ImportableCount                                             int
	Headers                                                     []string
	Mapping                                                     map[string]string
	DuplicateRows, RejectedRows                                 []CSVImportRejectedRow
	WouldCreateAccounts, WouldCreateCategories, WouldCreateTags []string
	AccountOptions                                              []CSVImportAccountOption
}
type ConfirmCSVImportParams struct {
	ActorUserID, ImportID string
	ExpectedImportType    CSVImportType
}
type CSVImportConfirmation struct{ ImportID, JobID, JobType string }
type RunCSVImportJobParams struct{ ImportID, JobID string }
type CSVImportRunResult struct {
	ImportedCount int
	RejectedRows  []CSVImportRejectedRow
}
type GetCSVImportAuditParams struct {
	ActorUserID, TenantID, ImportID string
	ExpectedImportType              CSVImportType
}
type ListRecentCSVImportAuditsParams struct {
	ActorUserID, TenantID string
	ExpectedImportType    CSVImportType
}
type CSVImportAudit struct {
	ImportID, TenantID       string
	ImportType               CSVImportType
	Status                   CSVImportStatus
	JobID, ConfirmedByUserID string
	ImportedCount            int
	RejectedRows             []CSVImportRejectedRow
	RowOutcomes              []domain.CSVImportRowOutcome
	CreatedAt                time.Time
	ConfirmedAt, CompletedAt *time.Time
}

type parsedCSVImportRow struct {
	RowNumber                                int
	Date                                     time.Time
	Account, Category, Currency, Description string
	Tags                                     []string
	AmountMinor                              int64
}

type parsedFixedCSV struct {
	headers          []string
	rows             []parsedCSVImportRow
	rejected         []CSVImportRejectedRow
	accountOptions   []CSVImportAccountOption
	accountNameByRow map[int]string
}

func parseFixedCSV(raw string) ([]string, []parsedCSVImportRow, []CSVImportRejectedRow, error) {
	parsed, err := parseFixedCSVWithAccountOptions(raw)
	if err != nil {
		return nil, nil, nil, err
	}
	return parsed.headers, parsed.rows, parsed.rejected, nil
}

func parseFixedCSVWithAccountOptions(raw string) (parsedFixedCSV, error) {
	if err := validateCSVImportByteLength(len(raw)); err != nil {
		return parsedFixedCSV{}, invalidCSVImportError(err)
	}
	parsed, err := parseFixedCSVReader(strings.NewReader(raw))
	if err != nil {
		return parsedFixedCSV{}, invalidCSVImportError(err)
	}
	return parsed, nil
}

func validateCSVImportByteLength(length int) error {
	if length > MaxCSVImportBytes {
		return fmt.Errorf("csv import exceeds %d bytes", MaxCSVImportBytes)
	}
	return nil
}

func invalidCSVImportError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidCSVImport, err)
}

func parseFixedCSVReader(source io.Reader) (parsedFixedCSV, error) {
	var headers []string
	var indexes map[string]int
	rows := []parsedCSVImportRow{}
	rejected := []CSVImportRejectedRow{}
	accountCounts := map[string]int{}
	accountNameByRow := map[int]string{}
	err := readBoundedCSVRecords(source, func(record []string, rowNumber int) error {
		if rowNumber == 1 {
			var headerErr error
			headers, indexes, headerErr = fixedCSVHeaderIndexes(record)
			return headerErr
		}
		if accountName := fixedCSVAccountName(record, indexes); accountName != "" {
			accountCounts[accountName]++
			accountNameByRow[rowNumber] = accountName
		}
		if columnErr := fixedCSVRecordHasRequiredColumns(record, indexes); columnErr != nil {
			rejected = append(rejected, csvImportRowDiagnostic(rowNumber, columnErr.Error()))
			return nil
		}
		row, parseErr := parseFixedCSVRow(record, indexes, rowNumber)
		if parseErr != nil {
			rejected = append(rejected, csvImportRowDiagnostic(rowNumber, parseErr.Error()))
			return nil
		}
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return parsedFixedCSV{}, err
	}
	if headers == nil {
		return parsedFixedCSV{}, errors.New("csv import requires a header row")
	}
	accountOptions := make([]CSVImportAccountOption, 0, len(accountCounts))
	for name, count := range accountCounts {
		accountOptions = append(accountOptions, CSVImportAccountOption{Name: name, SourceRowCount: count})
	}
	sort.Slice(accountOptions, func(i, j int) bool { return accountOptions[i].Name < accountOptions[j].Name })
	return parsedFixedCSV{
		headers:          headers,
		rows:             rows,
		rejected:         rejected,
		accountOptions:   accountOptions,
		accountNameByRow: accountNameByRow,
	}, nil
}

func fixedCSVAccountName(record []string, indexes map[string]int) string {
	index, ok := indexes[csvImportHeaderAccount]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

// readBoundedCSVRecords validates records as they are read without storing
// an additional copy of the full CSV before the caller processes each row.
func readBoundedCSVRecords(source io.Reader, visit func([]string, int) error) error {
	r := csv.NewReader(source)
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	dataRows := 0
	rowNumber := 0
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read csv import: %w", err)
		}
		rowNumber++
		if rowNumber > 1 {
			if dataRows >= MaxCSVImportRows {
				return fmt.Errorf("csv import exceeds %d data rows", MaxCSVImportRows)
			}
			dataRows++
		}
		if len(record) > maxCSVImportColumns {
			return fmt.Errorf("csv import exceeds %d columns", maxCSVImportColumns)
		}
		for _, cell := range record {
			if len(cell) > maxCSVImportCellBytes {
				return fmt.Errorf("csv import cell exceeds %d bytes", maxCSVImportCellBytes)
			}
		}
		if visitErr := visit(record, rowNumber); visitErr != nil {
			return visitErr
		}
	}
}

func csvImportRowDiagnostic(rowNumber int, reason string) CSVImportRejectedRow {
	return CSVImportRejectedRow{
		RowNumber: rowNumber,
		Field:     csvImportDiagnosticField(reason),
		Reason:    reason,
	}
}

func csvImportDiagnosticField(reason string) string {
	lowerReason := strings.ToLower(reason)
	for _, field := range []string{
		"date",
		"account",
		strings.ToLower(csvImportHeaderCategory),
		"tags",
		"expense amount",
		"income amount",
		csvImportFieldCurrency,
		"description",
	} {
		if strings.Contains(lowerReason, field) {
			return field
		}
	}
	return ""
}

func fixedCSVHeaderIndexes(raw []string) ([]string, map[string]int, error) {
	requiredHeaders := fixedCSVHeaders()
	headers := make([]string, len(raw))
	indexes := make(map[string]int, len(requiredHeaders))
	valid := make(map[string]string, len(requiredHeaders))
	for _, header := range requiredHeaders {
		valid[strings.ToLower(header)] = header
	}
	for i, value := range raw {
		if i == 0 {
			value = strings.TrimPrefix(value, "\ufeff")
		}
		value = strings.TrimSpace(value)
		headers[i] = value
		key := strings.ToLower(value)
		canonical, ok := valid[key]
		if !ok {
			continue
		}
		if _, duplicate := indexes[canonical]; duplicate {
			return nil, nil, fmt.Errorf("duplicate csv header %q", canonical)
		}
		headers[i] = canonical
		indexes[canonical] = i
	}
	for _, required := range fixedCSVRequiredHeaders() {
		if _, ok := indexes[required]; !ok {
			return nil, nil, fmt.Errorf("missing csv header %q", required)
		}
	}
	return headers, indexes, nil
}

func fixedCSVRecordHasRequiredColumns(record []string, indexes map[string]int) error {
	for _, header := range fixedCSVRequiredHeaders() {
		index := indexes[header]
		if index >= len(record) {
			return fmt.Errorf("missing column %q", header)
		}
	}
	return nil
}

func parseFixedCSVRow(record []string, indexes map[string]int, rowNumber int) (parsedCSVImportRow, error) {
	value := func(header string) string { return strings.TrimSpace(record[indexes[header]]) }
	optionalValue := func(header string) string {
		index, ok := indexes[header]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	date, err := parseCSVImportDate(value(csvImportHeaderDate))
	if err != nil {
		return parsedCSVImportRow{}, fmt.Errorf("date: %w", err)
	}
	account, currency, description := value(
		csvImportHeaderAccount,
	), normalizeSupportedCurrency(
		value(csvImportHeaderCurrency),
	), optionalValue("Description")
	if account == "" {
		return parsedCSVImportRow{}, errors.New("account is required")
	}
	if currency == "" {
		return parsedCSVImportRow{}, fmt.Errorf(
			"currency %q must be one of USD, EUR, PLN, UAH",
			record[indexes[csvImportHeaderCurrency]],
		)
	}
	description = normalizedCSVImportDescription(description)
	expense, expenseSet, err := parseCSVImportAmount(value(csvImportHeaderExpense))
	if err != nil {
		return parsedCSVImportRow{}, fmt.Errorf("expense amount: %w", err)
	}
	income, incomeSet, err := parseCSVImportAmount(value(csvImportHeaderIncome))
	if err != nil {
		return parsedCSVImportRow{}, fmt.Errorf("income amount: %w", err)
	}
	if expenseSet == incomeSet {
		return parsedCSVImportRow{}, errors.New("exactly one of Expense amount or Income amount must be positive")
	}
	amount := income
	if expenseSet {
		amount = -expense
	}
	tags, err := parseCSVImportTags(value(csvImportHeaderTags))
	if err != nil {
		return parsedCSVImportRow{}, fmt.Errorf("tags: %w", err)
	}
	return parsedCSVImportRow{
		RowNumber:   rowNumber,
		Date:        date,
		Account:     account,
		Category:    value(csvImportHeaderCategory),
		Tags:        tags,
		Currency:    currency,
		Description: description,
		AmountMinor: amount,
	}, nil
}

func normalizedCSVImportDescription(value string) string {
	if value = strings.TrimSpace(value); value == "" {
		return "n/a"
	}
	return value
}

func parseCSVImportDate(value string) (time.Time, error) {
	if len(value) != 8 || value[2] != '.' || value[5] != '.' {
		return time.Time{}, errors.New("must use strict dd.MM.yy format")
	}
	day, e1 := strconv.Atoi(value[:2])
	month, e2 := strconv.Atoi(value[3:5])
	year, e3 := strconv.Atoi(value[6:])
	if e1 != nil || e2 != nil || e3 != nil {
		return time.Time{}, errors.New("must use strict dd.MM.yy format")
	}
	result := time.Date(2000+year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	if result.Year() != 2000+year || int(result.Month()) != month || result.Day() != day {
		return time.Time{}, errors.New("is not a real calendar date")
	}
	return result, nil
}

func parseCSVImportAmount(value string) (int64, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, nil
	}
	integer, fraction, err := splitCSVImportAmount(value)
	if err != nil {
		return 0, false, err
	}
	whole, err := strconv.ParseInt(integer, 10, 64)
	if err != nil || whole > math.MaxInt64/csvImportMinorScale {
		return 0, false, errors.New("overflows minor units")
	}
	minorPart, err := parseCSVImportFraction(fraction)
	if err != nil {
		return 0, false, err
	}
	if whole == math.MaxInt64/csvImportMinorScale && minorPart > math.MaxInt64%csvImportMinorScale {
		return 0, false, errors.New("overflows minor units")
	}
	minor := whole*csvImportMinorScale + minorPart
	return minor, minor != 0, nil
}

func splitCSVImportAmount(value string) (string, string, error) {
	var digits strings.Builder
	comma := -1
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if r == ',' && comma == -1 {
			comma = digits.Len()
			continue
		}
		return "", "", errors.New("must be an unsigned decimal with an optional comma fraction")
	}
	number := digits.String()
	if number == "" {
		return "", "", errors.New("must contain digits")
	}
	fraction := ""
	integer := number
	if comma >= 0 {
		integer, fraction = number[:comma], number[comma:]
		if len(fraction) > csvImportFractionDigits {
			return "", "", errors.New("has more than two fractional digits")
		}
	}
	if integer == "" {
		return "", "", errors.New("must contain digits before the decimal separator")
	}
	return integer, fraction, nil
}

func parseCSVImportFraction(fraction string) (int64, error) {
	for len(fraction) < csvImportFractionDigits {
		fraction += "0"
	}
	if fraction == "" {
		return 0, nil
	}
	minorPart, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, errors.New("invalid fractional amount")
	}
	return minorPart, nil
}

func parseCSVImportTags(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	seen := map[string]struct{}{}
	result := []string{}
	for raw := range strings.SplitSeq(value, ",") {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			return nil, errors.New("contains an empty tag")
		}
		if _, ok := seen[tag]; !ok {
			seen[tag] = struct{}{}
			result = append(result, tag)
		}
	}
	return result, nil
}

func normalizeSupportedCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case tenantDisplayCurrencyUSD, tenantDisplayCurrencyEUR, tenantDisplayCurrencyPLN, tenantDisplayCurrencyUAH:
		return value
	}
	return ""
}

func readCSVRows(raw string) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv import: %w", err)
	}
	return rows, nil
}

func normalizeHeaders(headers []string) []string {
	result := make([]string, len(headers))
	for i, header := range headers {
		result[i] = strings.TrimSpace(header)
	}
	return result
}
func valueAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}
func csvImportMappedValue(headers []string, mapping map[string]string, row []string, field string) string {
	header := mapping[field]
	for i, current := range headers {
		if current == header {
			return strings.TrimSpace(valueAt(row, i))
		}
	}
	return ""
}
