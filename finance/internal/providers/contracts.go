package providers

import (
	"context"

	"github.com/gemyago/signal-foundry/finance/domain"
)

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

func MonobankProfile() ProviderProfile {
	return ProviderProfile{
		ProviderID:    domain.ProviderIDMonobank,
		ConnectorID:   domain.ProviderConnectorIDMonobank,
		DisplayName:   "Monobank",
		CountryCode:   "UA",
		MarketSegment: "personal",
	}
}

// PKOProfile keeps the product-level PKO provider composed with Enable Banking.
func PKOProfile() ProviderProfile {
	return ProviderProfile{
		ProviderID:    domain.ProviderIDPKO,
		ConnectorID:   domain.ProviderConnectorIDEnableBanking,
		DisplayName:   "PKO Bank Polski",
		CountryCode:   "PL",
		MarketSegment: "personal",
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
	Connection      domain.ProviderConnectionRef
	CandidateWindow domain.ProviderSyncWindow
	Accounts        []domain.ConnectionProviderAccount
	Transactions    []domain.Transaction
	Matches         []domain.ProviderTransactionMatch
}

type Connector interface {
	ConnectorID() domain.ProviderConnectorID
	Capabilities() ConnectorCapabilities
	StartLink(ctx context.Context, request StartLinkRequest) (StartLinkResult, error)
	FinishLink(ctx context.Context, request FinishLinkRequest) (LinkResult, error)
	LinkToken(ctx context.Context, request LinkTokenRequest) (LinkResult, error)
	Fetch(ctx context.Context, request FetchRequest) (domain.ProviderSyncBatch, error)
}
