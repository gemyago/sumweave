package providers

import (
	"context"

	"github.com/gemyago/signal-foundry/finance/domain"
)

const marketSegmentPersonal = "personal"

type ConnectorCapabilities struct {
	SupportsStartLink  bool
	SupportsFinishLink bool
	SupportsTokenLink  bool
	SupportsFetch      bool
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
	State            string
	AuthorizationURL string
	RawPayloads      []domain.ProviderRawPayloadObservation
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
	DisplayName       string
	ProviderReference string
	ExternalID        string
	Secret            string
	State             domain.BankConnectionState
	RawPayloads       []domain.ProviderRawPayloadObservation
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
	SaveConnectionProviderAccount(
		ctx context.Context,
		account domain.ConnectionProviderAccount,
	) (domain.ConnectionProviderAccount, error)
	SaveBalanceSnapshot(
		ctx context.Context,
		snapshot domain.BalanceSnapshot,
	) (domain.BalanceSnapshot, error)
	SaveRawPayload(
		ctx context.Context,
		payload domain.RawPayload,
	) (domain.RawPayload, error)
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
