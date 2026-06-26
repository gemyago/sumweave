package client

// SessionAccess models access metadata.
type SessionAccess struct {
	ValidForDays int            `json:"valid_for_days,omitempty"`
	ValidUntil   string         `json:"valid_until,omitempty"`
	Raw          map[string]any `json:"-"`
}

// SessionResponse models a session response.
type SessionResponse struct {
	ID                string         `json:"id,omitempty"`
	SessionID         string         `json:"sessionId,omitempty"`
	ExternalID        string         `json:"externalId,omitempty"`
	ProviderReference string         `json:"providerReference,omitempty"`
	DisplayName       string         `json:"displayName,omitempty"`
	Secret            string         `json:"secret,omitempty"`
	State             string         `json:"state,omitempty"`
	Access            *SessionAccess `json:"access,omitempty"`
	Accounts          []Account      `json:"accounts,omitempty"`
	Raw               map[string]any `json:"-"`
}
