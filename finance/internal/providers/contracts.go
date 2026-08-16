package providers

import (
	"context"

	"github.com/gemyago/sumweave/finance/domain"
)

const marketSegmentPersonal = "personal"

type ConnectorCapabilities struct {
	SupportsStartLink    bool
	SupportsFinishLink   bool
	RequiresRedirectCode bool
	SupportsTokenLink    bool
	SupportsFetch        bool
}

type ProviderProfile struct {
	ProviderID    domain.ProviderID
	ConnectorID   domain.ProviderConnectorID
	DisplayName   string
	CountryCode   string
	MarketSegment string
}

// PKOProfile keeps the product-level PKO provider composed with Enable Banking.
func PKOProfile() ProviderProfile {
	return ProviderProfile{
		ProviderID:    domain.ProviderIDPKO,
		ConnectorID:   domain.ProviderConnectorIDEnableBanking,
		DisplayName:   "PKO Bank Polski",
		CountryCode:   "PL",
		MarketSegment: marketSegmentPersonal,
	}
}

type StartLinkRequest struct {
	Profile            ProviderProfile
	RedirectURL        string
	BrowserCallbackURL string
}

type StartLinkResult struct {
	State             string
	ProviderReference string
	AuthorizationURL  string
	PendingDocument   []byte
}

type FinishLinkRequest struct {
	Profile ProviderProfile
	State   string
	Code    string
	Start   StartLinkResult
}

type LinkTokenRequest struct {
	Profile ProviderProfile
	Token   string
}

type LinkResult struct {
	DisplayName        string
	ProviderReference  string
	Secret             string
	State              domain.BankConnectionState
	ConnectionSnapshot *domain.ProviderSnapshotObservation
}

type FetchRequest struct {
	Connection      domain.ProviderConnectionRef
	Secret          domain.ConnectionSecret
	RequestedWindow domain.ProviderSyncWindow
	SyncState       *domain.ProviderSyncState
}

type ExistingWindowSnapshot struct {
	Connection     domain.ProviderConnectionRef
	SnapshotWindow domain.ProviderSyncWindow
	Accounts       []domain.ConnectionProviderAccount
	Transactions   []domain.Transaction
	Matches        []domain.ProviderTransactionMatch
}

type WindowSyncSnapshotReader interface {
	ListConnectionProviderAccounts(
		ctx context.Context,
		connectionID string,
	) ([]domain.ConnectionProviderAccount, error)
	ListProviderTransactionsInWindow(
		ctx context.Context,
		financeAccountIDs []string,
		window domain.ProviderSyncWindow,
	) ([]domain.Transaction, error)
	ListProviderTransactionMatchesByTransactionIDs(
		ctx context.Context,
		connectionID string,
		transactionIDs []string,
	) ([]domain.ProviderTransactionMatch, error)
}

type WindowSyncApplyStore interface {
	GetBankConnection(ctx context.Context, connectionID string) (*domain.BankConnection, error)
	GetAccount(ctx context.Context, accountID string) (*domain.Account, error)
	SaveAccount(ctx context.Context, account domain.Account) (domain.Account, error)
	SaveConnectionProviderAccount(
		ctx context.Context,
		account domain.ConnectionProviderAccount,
	) (domain.ConnectionProviderAccount, error)
	SaveBalanceSnapshot(
		ctx context.Context,
		snapshot domain.BalanceSnapshot,
	) (domain.BalanceSnapshot, error)
	SaveProviderSnapshot(
		ctx context.Context,
		snapshot domain.ProviderSnapshot,
	) (domain.ProviderSnapshot, error)
	SaveTransaction(
		ctx context.Context,
		transaction domain.Transaction,
	) (domain.Transaction, error)
	SaveProviderTransactionMatch(
		ctx context.Context,
		match domain.ProviderTransactionMatch,
	) (domain.ProviderTransactionMatch, error)
}

type WindowSyncTransactor interface {
	WithTransaction(ctx context.Context, fn func(WindowSyncApplyStore) error) error
}

type WindowSyncPersistence interface {
	WindowSyncSnapshotReader
	WindowSyncTransactor
}

type Connector interface {
	ConnectorID() domain.ProviderConnectorID
	Capabilities() ConnectorCapabilities
	StartLink(ctx context.Context, request StartLinkRequest) (StartLinkResult, error)
	FinishLink(ctx context.Context, request FinishLinkRequest) (LinkResult, error)
	LinkToken(ctx context.Context, request LinkTokenRequest) (LinkResult, error)
	Fetch(ctx context.Context, request FetchRequest) (domain.ProviderSyncBatch, error)
}
