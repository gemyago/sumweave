package client

// SessionAccess models access metadata.
type SessionAccess struct {
	ValidForDays *int   `json:"valid_for_days,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"`
}

// SessionResponse models a session response.
type SessionResponse struct {
	Access       *SessionAccess `json:"access,omitempty"`
	Accounts     []string       `json:"accounts,omitempty"`
	AccountsData []Account      `json:"accounts_data,omitempty"`
	ASPSP        *ASPSP         `json:"aspsp,omitempty"`
	PSUType      string         `json:"psu_type,omitempty"`
	Status       string         `json:"status,omitempty"`
	Authorized   string         `json:"authorized,omitempty"`
	Created      string         `json:"created,omitempty"`
}

// CreateSessionResponse models the documented POST /sessions response.
type CreateSessionResponse struct {
	SessionID string         `json:"session_id,omitempty"`
	Access    *SessionAccess `json:"access,omitempty"`
	Accounts  []Account      `json:"accounts,omitempty"`
	ASPSP     *ASPSP         `json:"aspsp,omitempty"`
	PSUType   string         `json:"psu_type,omitempty"`
}
