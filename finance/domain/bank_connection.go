package domain

import "time"

type BankConnectionState string

const (
	BankConnectionStatePendingAuth    BankConnectionState = "pending_auth"
	BankConnectionStateActive         BankConnectionState = "active"
	BankConnectionStateReauthRequired BankConnectionState = "reauth_required"
	BankConnectionStateDisconnected   BankConnectionState = "disconnected"
)

type ConnectionReauthMetadata struct {
	RequiredAt *time.Time
	Reason     string
}

type BankConnection struct {
	ID                   string
	TenantID             string
	Provider             string
	ConnectorID          ProviderConnectorID
	DisplayName          string
	ProviderReference    string
	SecretID             string
	State                BankConnectionState
	Reauth               *ConnectionReauthMetadata
	LastSyncJobID        string
	LastSyncStartedAt    *time.Time
	LastSuccessfulSyncAt *time.Time
	LastSyncError        string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PendingBankConnectionLinkStart struct {
	ID                string
	TenantID          string
	ActorUserID       string
	Provider          string
	ConnectorID       ProviderConnectorID
	State             string
	CallbackURL       string
	AuthorizationURL  string
	ProviderReference string
	StartResult       PendingBankConnectionLinkStartResult
	ExpiresAt         time.Time
	ConsumedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PendingBankConnectionLinkStartResult struct {
	State            string
	AuthorizationURL string
	DocumentJSON     []byte
}

type BankConnectionSchedule struct {
	ConnectionID    string
	Interval        time.Duration
	NextRunAt       *time.Time
	LastScheduledAt *time.Time
	LastStartedAt   *time.Time
	LastCompletedAt *time.Time
	LastJobID       string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ConnectionProviderAccount struct {
	ID                   string
	ConnectionID         string
	ProviderAccountID    string
	FinanceAccountID     string
	Name                 string
	Currency             string
	IBAN                 string
	MaskedPAN            string
	LastSuccessfulSyncAt *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type BalanceSnapshot struct {
	ID                    string
	ConnectionID          string
	ProviderAccountID     string
	FinanceAccountID      string
	Currency              string
	CurrentBalanceMinor   int64
	AvailableBalanceMinor *int64
	CapturedAt            time.Time
}

type ProviderTransactionMatch struct {
	ID                    string
	ConnectionID          string
	ProviderAccountID     string
	ProviderTransactionID string
	Fingerprint           string
	TransactionID         string
	Status                TransactionStatus
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type BankConnectionSyncRun struct {
	ID           string
	ConnectionID string
	SyncKey      string
	JobID        string
	CreatedAt    time.Time
}
