package client

// TransactionAmount models a transaction amount.
type TransactionAmount = Amount

// AccountTransaction models a transaction item.
type AccountTransaction struct {
	EntryReference                          string                        `json:"entry_reference,omitempty"`
	MerchantCategoryCode                    string                        `json:"merchant_category_code,omitempty"`
	TransactionAmount                       *TransactionAmount            `json:"transaction_amount,omitempty"`
	Creditor                                *PartyIdentification          `json:"creditor,omitempty"`
	CreditorAccount                         *AccountIdentification        `json:"creditor_account,omitempty"`
	CreditorAgent                           *FinancialInstitution         `json:"creditor_agent,omitempty"`
	Debtor                                  *PartyIdentification          `json:"debtor,omitempty"`
	DebtorAccount                           *AccountIdentification        `json:"debtor_account,omitempty"`
	DebtorAgent                             *FinancialInstitution         `json:"debtor_agent,omitempty"`
	BankTransactionCode                     *BankTransactionCode          `json:"bank_transaction_code,omitempty"`
	CreditDebitIndicator                    string                        `json:"credit_debit_indicator,omitempty"`
	Status                                  string                        `json:"status,omitempty"`
	BookingDate                             string                        `json:"booking_date,omitempty"`
	ValueDate                               string                        `json:"value_date,omitempty"`
	TransactionDate                         string                        `json:"transaction_date,omitempty"`
	BalanceAfterTransaction                 *TransactionAmount            `json:"balance_after_transaction,omitempty"`
	ReferenceNumber                         string                        `json:"reference_number,omitempty"`
	ReferenceNumberSchema                   string                        `json:"reference_number_schema,omitempty"`
	RemittanceInformation                   []string                      `json:"remittance_information,omitempty"`
	DebtorAccountAdditionalIdentification   *GenericAccountIdentification `json:"debtor_account_additional_identification,omitempty"`
	CreditorAccountAdditionalIdentification *GenericAccountIdentification `json:"creditor_account_additional_identification,omitempty"`
	ExchangeRate                            *ExchangeRate                 `json:"exchange_rate,omitempty"`
	Note                                    *string                       `json:"note,omitempty"`
	TransactionID                           string                        `json:"transaction_id,omitempty"`
}

// BankTransactionCode models a bank transaction code.
type BankTransactionCode struct {
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
	SubCode     string `json:"sub_code,omitempty"`
}

// ExchangeRate models a transaction exchange-rate resource.
type ExchangeRate struct {
	UnitCurrency           string             `json:"unit_currency,omitempty"`
	ExchangeRate           string             `json:"exchange_rate,omitempty"`
	RateType               string             `json:"rate_type,omitempty"`
	ContractIdentification *string            `json:"contract_identification,omitempty"`
	InstructedAmount       *TransactionAmount `json:"instructed_amount,omitempty"`
}

// GetAccountTransactionsResponse models transactions pages.
type GetAccountTransactionsResponse struct {
	ContinuationKey string               `json:"continuation_key,omitempty"`
	Transactions    []AccountTransaction `json:"transactions,omitempty"`
}
