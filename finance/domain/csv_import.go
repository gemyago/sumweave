package domain

import "time"

type CSVImportType string

const (
	CSVImportTypeTransactions CSVImportType = "transactions"
	CSVImportTypeAccounts     CSVImportType = "accounts"
)

type CSVImportStatus string

const (
	CSVImportStatusPreviewed CSVImportStatus = "previewed"
	CSVImportStatusConfirmed CSVImportStatus = "confirmed"
	CSVImportStatusRunning   CSVImportStatus = "running"
	CSVImportStatusCompleted CSVImportStatus = "completed"
)

type CSVImportRejectedRow struct {
	RowNumber int
	Field     string
	Reason    string
}

// CSVImportAccountOption describes one textual account detected in a preview source.
type CSVImportAccountOption struct {
	Name           string
	SourceRowCount int
	Selected       bool
}

type CSVImportRowOutcomeStatus string

const (
	CSVImportRowOutcomeImported CSVImportRowOutcomeStatus = "imported"
	CSVImportRowOutcomeRejected CSVImportRowOutcomeStatus = "rejected"
)

// CSVImportTransactionRow is the validated, fixed-contract input for one row.
type CSVImportTransactionRow struct {
	ImportID, TenantID, ActorUserID string
	RowNumber                       int
	AccountName, CategoryName       string
	TagNames                        []string
	Currency, Description           string
	AmountMinor                     int64
	EffectiveAt                     time.Time
}

// CSVImportRowOutcome is durable audit data keyed by import and CSV row.
type CSVImportRowOutcome struct {
	ImportID, TransactionID string
	RowNumber               int
	Status                  CSVImportRowOutcomeStatus
	Reason                  string
	CreatedAt, UpdatedAt    time.Time
}

type CSVImportRecord struct {
	ID                    string
	TenantID              string
	Type                  CSVImportType
	Status                CSVImportStatus
	FileName              string
	RawCSV                string
	Headers               []string
	Mapping               map[string]string
	DuplicateRows         []CSVImportRejectedRow
	RejectedRows          []CSVImportRejectedRow
	ImportableCount       int
	WouldCreateAccounts   []string
	WouldCreateCategories []string
	WouldCreateTags       []string
	AccountOptions        []CSVImportAccountOption
	SelectedAccountNames  []string
	JobID                 string
	ConfirmedByUserID     string
	ConfirmedAt           *time.Time
	CompletedAt           *time.Time
	ImportedCount         int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
