package finance

type ProviderLinkStart struct {
	State             string
	AuthorizationURL  string
	ProviderReference string
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

type UpdateBankConnectionParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	Name         string
}
