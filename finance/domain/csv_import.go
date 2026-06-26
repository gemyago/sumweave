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
	CSVImportStatusCompleted CSVImportStatus = "completed"
)

type CSVImportRejectedRow struct {
	RowNumber int
	Reason    string
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
	WouldCreateAccounts   []string
	WouldCreateCategories []string
	WouldCreateTags       []string
	JobID                 string
	ConfirmedByUserID     string
	ConfirmedAt           *time.Time
	CompletedAt           *time.Time
	ImportedCount         int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
