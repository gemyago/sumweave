package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/credentials"
	"github.com/gemyago/sumweave/finance/domain"
)

const (
	BankConnectionSyncJobType         = "finance.bank_connection_sync"
	BankConnectionSyncReasonManual    = "manual"
	BankConnectionSyncReasonScheduled = "scheduled"
	bankProviderMonobank              = "monobank"
	bankProviderPKO                   = "pko"
	bankConnectorEnableBanking        = "enable-banking"
	pendingBankConnectionLinkStartTTL = 15 * time.Minute
)

var (
	ErrBankConnectionNotFound                 = errors.New("bank connection not found")
	ErrBankConnectionNameRequired             = errors.New("bank connection name is required")
	ErrPendingBankConnectionLinkStartNotFound = errors.New("pending bank connection link start not found")
	ErrBankProviderNotConfigured              = errors.New("bank provider not configured")
	ErrUnsupportedBankProvider                = errors.New("unsupported bank provider")
	ErrUnsupportedBankLinkingMethod           = errors.New("unsupported bank linking method")
)

type connectionSecretCipher interface {
	SealString(plaintext string) (credentials.Envelope, error)
	OpenString(envelope credentials.Envelope) (string, error)
}

type BankConnectionSyncJobEnqueuer interface {
	EnqueueBankConnectionSync(
		ctx context.Context,
		request BankConnectionSyncJobRequest,
	) (BankConnectionSyncJobRef, error)
}

type BankConnectionSyncJobRequest struct {
	JobType string
	Input   BankConnectionSyncJobInput
	Reason  string
	Actor   string
}

type BankConnectionSyncJobInput struct {
	ConnectionID string
	Reason       string
	WindowStart  *time.Time
	WindowEnd    *time.Time
}

type BankConnectionSyncJobRef struct {
	ID      string
	JobType string
}

type BankConnectionProvider interface {
	Name() string
	StartLink(ctx context.Context, params ProviderStartLinkParams) (ProviderLinkStart, error)
	FinishLink(ctx context.Context, params ProviderFinishLinkParams) (ProviderLinkResult, error)
	LinkToken(ctx context.Context, params ProviderTokenLinkParams) (ProviderTokenLinkResult, error)
}

type ProviderStartLinkParams struct {
	RedirectURL string
}

type ProviderFinishLinkParams struct {
	State string
	Code  string
	Start ProviderLinkStart
}

type ProviderTokenLinkParams struct {
	Token string
}

type ProviderLinkResult struct {
	DisplayName       string
	ProviderReference string
	Secret            string
	State             domain.BankConnectionState
}

type ProviderTokenLinkResult = ProviderLinkResult

type ProviderScheduledRunMetadata struct {
	ScheduledAt time.Time
	NextRunAt   *time.Time
}

type UpsertBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	Interval     time.Duration
	NextRunAt    time.Time
}

type PauseBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
}

type ResumeBankConnectionScheduleParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	NextRunAt    time.Time
}

type TriggerBankConnectionSyncParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	Reason       string
	WindowStart  *time.Time
	WindowEnd    *time.Time
}

type DeleteBankConnectionParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
}

type RunBankConnectionSyncParams struct {
	ConnectionID       string
	JobID              string
	Reason             string
	WindowStart        *time.Time
	WindowEnd          *time.Time
	ScheduledAt        *time.Time
	ScheduledNextRunAt *time.Time
}

type RecordBankConnectionSyncScheduledParams struct {
	ConnectionID string
	JobID        string
	ScheduledAt  time.Time
	NextRunAt    time.Time
}

type BankConnectionSyncResult struct {
	ImportedAccounts     int
	ImportedTransactions int
	UpdatedTransactions  int
}

type ListBankConnectionsParams struct {
	ActorUserID string
	TenantID    string
}

type ListBankConnectionSyncedAccountsParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
}

type BankConnectionView struct {
	Connection domain.BankConnection
	Schedule   *domain.BankConnectionSchedule
}

type BankConnectionSyncedAccount struct {
	FinanceAccountID     string
	Name                 string
	Currency             string
	LastSuccessfulSyncAt *time.Time
}

type bankProviderRef struct {
	BankConnectionProvider

	bankID string
}

type bankLinkMethod string

const (
	bankLinkMethodToken    bankLinkMethod = "token"
	bankLinkMethodRedirect bankLinkMethod = "redirect"
)

type bankSyncStoreRef struct {
	bankSyncStore
}

type connectionSecretsStoreRef struct {
	connectionSecretStore
}

type BankConnectionSyncScheduleWriter interface {
	UpsertBankConnectionSyncSchedule(context.Context, BankConnectionSyncSchedule) error
}

type BankConnectionSyncSchedule struct {
	ScheduleID   string
	ConnectionID string
	ActorUserID  string
	Interval     time.Duration
	NextRunAt    *time.Time
	Enabled      bool
}

type bankSyncStore interface {
	SaveBankConnection(
		ctx context.Context,
		connection domain.BankConnection,
	) (domain.BankConnection, error)
	GetBankConnection(ctx context.Context, connectionID string) (*domain.BankConnection, error)
	ListBankConnections(ctx context.Context, tenantID string) ([]domain.BankConnection, error)
	DeleteBankConnection(ctx context.Context, connectionID string) error
	SaveBankConnectionSchedule(
		ctx context.Context,
		schedule domain.BankConnectionSchedule,
	) (domain.BankConnectionSchedule, error)
	GetBankConnectionSchedule(
		ctx context.Context,
		connectionID string,
	) (*domain.BankConnectionSchedule, error)
	DeleteBankConnectionSchedule(ctx context.Context, connectionID string) error
	SaveConnectionProviderAccount(
		ctx context.Context,
		account domain.ConnectionProviderAccount,
	) (domain.ConnectionProviderAccount, error)
	ListConnectionProviderAccounts(
		ctx context.Context,
		connectionID string,
	) ([]domain.ConnectionProviderAccount, error)
	DeleteConnectionProviderAccounts(ctx context.Context, connectionID string) error
	DeleteBalanceSnapshots(ctx context.Context, connectionID string) error
	DeleteProviderTransactionMatches(ctx context.Context, connectionID string) error
}

type connectionSecretStore interface {
	SaveConnectionSecret(
		ctx context.Context,
		secret domain.ConnectionSecret,
	) (domain.ConnectionSecret, error)
	GetConnectionSecret(ctx context.Context, secretID string) (*domain.ConnectionSecret, error)
	DeleteConnectionSecret(ctx context.Context, secretID string) error
}

func bankConnectionSyncScheduleID(connectionID string) string {
	return "finance.bank_connection_sync:" + strings.TrimSpace(connectionID)
}

func bankProviderNotConfiguredError(providerName string) error {
	return fmt.Errorf("%w: %s", ErrBankProviderNotConfigured, strings.TrimSpace(providerName))
}

func bankProviderNotConfiguredForBankError(bankID string, providerName string) error {
	trimmedBankID := strings.TrimSpace(bankID)
	trimmedProviderName := strings.TrimSpace(providerName)
	if trimmedBankID == trimmedProviderName || trimmedProviderName == "" {
		return bankProviderNotConfiguredError(trimmedBankID)
	}
	return fmt.Errorf(
		"%w: bank provider %s requires connector %s",
		ErrBankProviderNotConfigured,
		trimmedBankID,
		trimmedProviderName,
	)
}

func configuredBankProviderName(bankID string, method bankLinkMethod) (string, error) {
	switch strings.TrimSpace(bankID) {
	case bankProviderMonobank:
		if method != bankLinkMethodToken {
			return "", unsupportedBankLinkingMethodError(bankProviderMonobank, method)
		}
		return bankProviderMonobank, nil
	case bankProviderPKO:
		if method != bankLinkMethodRedirect {
			return "", unsupportedBankLinkingMethodError(bankProviderPKO, method)
		}
		return bankConnectorEnableBanking, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedBankProvider, strings.TrimSpace(bankID))
	}
}

func unsupportedBankLinkingMethodError(bankID string, method bankLinkMethod) error {
	if method == bankLinkMethodToken {
		return fmt.Errorf(
			"%w: token linking unsupported for bank provider: %s",
			ErrUnsupportedBankLinkingMethod,
			bankID,
		)
	}
	return fmt.Errorf(
		"%w: redirect linking unsupported for bank provider: %s",
		ErrUnsupportedBankLinkingMethod,
		bankID,
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
