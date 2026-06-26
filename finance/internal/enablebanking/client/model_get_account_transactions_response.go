package client

// TransactionAmount models a transaction amount.
type TransactionAmount struct {
	Amount   string         `json:"amount,omitempty"`
	Currency string         `json:"currency,omitempty"`
	Raw      map[string]any `json:"-"`
}

// AccountTransaction models a transaction item.
type AccountTransaction struct {
	TransactionID                     string             `json:"transactionId,omitempty"`
	ID                                string             `json:"id,omitempty"`
	Status                            string             `json:"status,omitempty"`
	BookingDate                       string             `json:"bookingDate,omitempty"`
	ValueDate                         string             `json:"valueDate,omitempty"`
	Currency                          string             `json:"currency,omitempty"`
	AmountMinor                       int64              `json:"amountMinor,omitempty"`
	Description                       string             `json:"description,omitempty"`
	EffectiveAt                       string             `json:"effectiveAt,omitempty"`
	CreditDebitIndicator              string             `json:"creditDebitIndicator,omitempty"`
	RemittanceInformationUnstructured string             `json:"remittanceInformationUnstructured,omitempty"`
	Amount                            *TransactionAmount `json:"amount,omitempty"`
	Raw                               map[string]any     `json:"-"`
}

// GetAccountTransactionsResponse models transactions pages.
type GetAccountTransactionsResponse struct {
	ContinuationKey string               `json:"continuationKey,omitempty"`
	Transactions    []AccountTransaction `json:"transactions,omitempty"`
	Raw             map[string]any       `json:"-"`
}
