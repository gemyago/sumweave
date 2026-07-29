package finance

import "github.com/gemyago/sumweave/finance/domain"

type ProviderLinkStart struct {
	State             string
	AuthorizationURL  string
	ProviderReference string
	RawPayloads       []ProviderRawPayload
}

type ProviderRawPayload struct {
	Scope            domain.RawPayloadScope
	ProviderObjectID string
	PayloadJSON      []byte
}

type LinkTokenBankConnectionParams struct {
	ActorUserID string
	TenantID    string
	Provider    string
	Token       string
}

type StartBankConnectionLinkParams struct {
	ActorUserID        string
	TenantID           string
	Provider           string
	RedirectURL        string
	BrowserCallbackURL string
}

type GetPendingBankConnectionLinkStartByStateParams struct {
	Provider string
	State    string
}

type FinishBankConnectionLinkParams struct {
	ActorUserID string
	TenantID    string
	Provider    string
	State       string
	Code        string
	Start       ProviderLinkStart
}
