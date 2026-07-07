package client

// TransactionAmount models a transaction amount.
type TransactionAmount struct {
	Amount   string `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// AccountTransaction models a transaction item.
type AccountTransaction struct {
	EntryReference                    string             `json:"entry_reference,omitempty"`
	TransactionID                     string             `json:"transaction_id,omitempty"`
	Status                            string             `json:"status,omitempty"`
	BookingDate                       string             `json:"booking_date,omitempty"`
	ValueDate                         string             `json:"value_date,omitempty"`
	TransactionDate                   string             `json:"transaction_date,omitempty"`
	CreditDebitIndicator              string             `json:"credit_debit_indicator,omitempty"`
	TransactionAmount                 *TransactionAmount `json:"transaction_amount,omitempty"`
	Note                              string             `json:"note,omitempty"`
	RemittanceInformation             []string           `json:"remittance_information,omitempty"`
	Currency                          string             `json:"-"`
	AmountMinor                       int64              `json:"-"`
	Description                       string             `json:"-"`
	EffectiveAt                       string             `json:"-"`
	RemittanceInformationUnstructured string             `json:"-"`
	Amount                            *TransactionAmount `json:"-"`
	ID                                string             `json:"-"`
}

// GetAccountTransactionsResponse models transactions pages.
type GetAccountTransactionsResponse struct {
	ContinuationKey string               `json:"continuation_key,omitempty"`
	Transactions    []AccountTransaction `json:"transactions,omitempty"`
}
