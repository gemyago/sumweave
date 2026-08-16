package client

// Access models the documented session access schema.
type Access struct {
	Accounts     *[]AccountIdentification `json:"accounts,omitempty"`
	Balances     *bool                    `json:"balances,omitempty"`
	Transactions *bool                    `json:"transactions,omitempty"`
	ValidUntil   string                   `json:"valid_until"`
}

// SessionAccess remains an alias for callers using the former name.
type SessionAccess = Access

// SessionAccount models the documented GET session account item.
type SessionAccount struct {
	UID                  string   `json:"uid"`
	IdentificationHash   string   `json:"identification_hash"`
	IdentificationHashes []string `json:"identification_hashes"`
}

// SessionResponse models the documented GET /sessions/{session_id} response.
type SessionResponse struct {
	Status       string           `json:"status"`
	Accounts     []string         `json:"accounts"`
	AccountsData []SessionAccount `json:"accounts_data"`
	ASPSP        ASPSP            `json:"aspsp"`
	PSUType      string           `json:"psu_type"`
	PSUIDHash    string           `json:"psu_id_hash"`
	Access       Access           `json:"access"`
	Created      string           `json:"created"`
	Authorized   *string          `json:"authorized,omitempty"`
	Closed       *string          `json:"closed,omitempty"`
}

// CreateSessionResponse models the documented POST /sessions response.
type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	Accounts  []Account `json:"accounts"`
	ASPSP     ASPSP     `json:"aspsp"`
	PSUType   string    `json:"psu_type"`
	Access    Access    `json:"access"`
}
