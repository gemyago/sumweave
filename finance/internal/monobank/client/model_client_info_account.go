package client

// InfoAccount models a Monobank account.
type InfoAccount struct {
	ID           string   `json:"id"`
	Type         string   `json:"type,omitempty"`
	CurrencyCode int      `json:"currencyCode,omitempty"`
	Balance      int64    `json:"balance,omitempty"`
	CreditLimit  int64    `json:"creditLimit,omitempty"`
	MaskedPAN    []string `json:"maskedPan,omitempty"`
	IBAN         string   `json:"iban,omitempty"`
}
