package domain

import "time"

type ProviderID string

const (
	ProviderIDMonobank ProviderID = "monobank"
	ProviderIDPKO      ProviderID = "pko"
)

type ProviderConnectorID string

const (
	ProviderConnectorIDMonobank      ProviderConnectorID = "monobank"
	ProviderConnectorIDEnableBanking ProviderConnectorID = "enable-banking"
)

type ProviderConnectionRef struct {
	ConnectionID      string
	ProviderID        ProviderID
	ConnectorID       ProviderConnectorID
	ProviderReference string
	ExternalID        string
}

type ProviderSyncWindow struct {
	Start time.Time
	End   time.Time
}

type ProviderAccountObservation struct {
	Connection        ProviderConnectionRef
	ProviderAccountID string
	Name              string
	Currency          string
	IBAN              string
	MaskedPAN         string
}

type ProviderBalanceObservation struct {
	Connection            ProviderConnectionRef
	ProviderAccountID     string
	Currency              string
	CurrentBalanceMinor   int64
	AvailableBalanceMinor *int64
	CapturedAt            time.Time
}

type ProviderTransactionObservation struct {
	Connection            ProviderConnectionRef
	ProviderAccountID     string
	ProviderTransactionID string
	Status                TransactionStatus
	AmountMinor           int64
	Currency              string
	Description           string
	EffectiveAt           time.Time
	Fingerprint           string
	ProviderOriginal      *ProviderTransactionOriginal
}

type ProviderRawPayloadObservation struct {
	Connection       ProviderConnectionRef
	Scope            RawPayloadScope
	ProviderObjectID string
	PayloadJSON      []byte
	CapturedAt       time.Time
}

type ProviderSyncBatch struct {
	Connection      ProviderConnectionRef
	RequestedWindow ProviderSyncWindow
	Accounts        []ProviderAccountObservation
	Balances        []ProviderBalanceObservation
	Transactions    []ProviderTransactionObservation
	RawPayloads     []ProviderRawPayloadObservation
}

type ProviderSyncRunStatus string

const (
	ProviderSyncRunStatusPending   ProviderSyncRunStatus = "pending"
	ProviderSyncRunStatusRunning   ProviderSyncRunStatus = "running"
	ProviderSyncRunStatusSucceeded ProviderSyncRunStatus = "succeeded"
	ProviderSyncRunStatusFailed    ProviderSyncRunStatus = "failed"
)

type ProviderSyncIssueSeverity string

const (
	ProviderSyncIssueSeverityInfo    ProviderSyncIssueSeverity = "info"
	ProviderSyncIssueSeverityWarning ProviderSyncIssueSeverity = "warning"
	ProviderSyncIssueSeverityError   ProviderSyncIssueSeverity = "error"
)

type ProviderSyncStats struct {
	ObservedAccounts             int
	ObservedTransactions         int
	CreatedTransactions          int
	UpdatedTransactions          int
	AmbiguousCreatedTransactions int
}

type ProviderSyncIssue struct {
	Code                  string
	Severity              ProviderSyncIssueSeverity
	Summary               string
	ProviderAccountID     string
	ProviderTransactionID string
}

type ProviderSyncState struct {
	Connection           ProviderConnectionRef
	LastAttemptAt        *time.Time
	LastSuccessAt        *time.Time
	LastSuccessfulWindow *ProviderSyncWindow
	LastRunID            string
	LastJobID            string
	LastErrorSummary     string
	AggregateStats       ProviderSyncStats
}

type ProviderSyncRun struct {
	ID              string
	Connection      ProviderConnectionRef
	JobID           string
	Reason          string
	RequestedWindow ProviderSyncWindow
	CandidateWindow ProviderSyncWindow
	Status          ProviderSyncRunStatus
	StartedAt       time.Time
	CompletedAt     *time.Time
	Stats           ProviderSyncStats
	Issues          []ProviderSyncIssue
}
