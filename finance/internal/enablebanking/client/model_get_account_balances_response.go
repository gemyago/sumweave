package client

// BalanceAmount models a balance amount.
type BalanceAmount struct {
	Amount   string         `json:"amount,omitempty"`
	Currency string         `json:"currency,omitempty"`
	Raw      map[string]any `json:"-"`
}

// AccountBalance models an account balance.
type AccountBalance struct {
	Type                  string         `json:"type,omitempty"`
	CurrentBalanceMinor   int64          `json:"currentBalanceMinor,omitempty"`
	AvailableBalanceMinor int64          `json:"availableBalanceMinor,omitempty"`
	BalanceAmount         *BalanceAmount `json:"balanceAmount,omitempty"`
	Raw                   map[string]any `json:"-"`
}

// GetAccountBalancesResponse models account balances.
type GetAccountBalancesResponse struct {
	Balances []AccountBalance `json:"balances,omitempty"`
	Raw      map[string]any   `json:"-"`
}
