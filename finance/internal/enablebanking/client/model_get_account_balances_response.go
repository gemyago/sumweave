package client

// BalanceAmount models a balance amount.
type BalanceAmount struct {
	Amount   string `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// AccountBalance models an account balance.
type AccountBalance struct {
	Name                  string         `json:"name,omitempty"`
	BalanceType           string         `json:"balance_type,omitempty"`
	BalanceAmount         *BalanceAmount `json:"balance_amount,omitempty"`
	LastChangeDateTime    string         `json:"last_change_date_time,omitempty"`
	ReferenceDate         string         `json:"reference_date,omitempty"`
	LastCommittedTxn      string         `json:"last_committed_transaction,omitempty"`
	Type                  string         `json:"-"`
	CurrentBalanceMinor   int64          `json:"-"`
	AvailableBalanceMinor int64          `json:"-"`
}

// GetAccountBalancesResponse models account balances.
type GetAccountBalancesResponse struct {
	Balances []AccountBalance `json:"balances,omitempty"`
}
