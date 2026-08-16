package client

// BalanceAmount models a balance amount.
type BalanceAmount = Amount

// AccountBalance models a documented balance resource.
type AccountBalance struct {
	Name               string  `json:"name"`
	BalanceAmount      Amount  `json:"balance_amount"`
	BalanceType        string  `json:"balance_type"`
	LastChangeDateTime *string `json:"last_change_date_time,omitempty"`
	ReferenceDate      *string `json:"reference_date,omitempty"`
	LastCommittedTxn   *string `json:"last_committed_transaction,omitempty"`
}

// GetAccountBalancesResponse models the documented balances envelope.
type GetAccountBalancesResponse struct {
	Balances []AccountBalance `json:"balances"`
}
