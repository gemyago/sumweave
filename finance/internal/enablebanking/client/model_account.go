package client

// Account models an Enable Banking account.
type Account struct {
	UID      string         `json:"uid,omitempty"`
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	IBAN     string         `json:"iban,omitempty"`
	Currency string         `json:"currency,omitempty"`
	Raw      map[string]any `json:"-"`
}

// ListAccountsResponse models an accounts response.
type ListAccountsResponse struct {
	State            string         `json:"state,omitempty"`
	ReauthReason     string         `json:"reauthReason,omitempty"`
	ReauthRequiredAt string         `json:"reauthRequiredAt,omitempty"`
	Accounts         []Account      `json:"accounts,omitempty"`
	Raw              map[string]any `json:"-"`
}
