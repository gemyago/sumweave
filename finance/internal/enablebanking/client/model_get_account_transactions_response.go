package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// TransactionAmount models a transaction amount.
type TransactionAmount = Amount

// AccountTransaction models a documented transaction item.
type AccountTransaction struct {
	EntryReference                          *string                    `json:"entry_reference,omitempty"`
	MerchantCategoryCode                    *string                    `json:"merchant_category_code,omitempty"`
	TransactionAmount                       Amount                     `json:"transaction_amount"`
	Creditor                                *PartyIdentification       `json:"creditor,omitempty"`
	CreditorAccount                         *AccountIdentification     `json:"creditor_account,omitempty"`
	CreditorAgent                           *FinancialInstitution      `json:"creditor_agent,omitempty"`
	Debtor                                  *PartyIdentification       `json:"debtor,omitempty"`
	DebtorAccount                           *AccountIdentification     `json:"debtor_account,omitempty"`
	DebtorAgent                             *FinancialInstitution      `json:"debtor_agent,omitempty"`
	BankTransactionCode                     *BankTransactionCode       `json:"bank_transaction_code,omitempty"`
	CreditDebitIndicator                    string                     `json:"credit_debit_indicator"`
	Status                                  string                     `json:"status"`
	BookingDate                             *string                    `json:"booking_date,omitempty"`
	ValueDate                               *string                    `json:"value_date,omitempty"`
	TransactionDate                         *string                    `json:"transaction_date,omitempty"`
	BalanceAfterTransaction                 *TransactionAmount         `json:"balance_after_transaction,omitempty"`
	ReferenceNumber                         *string                    `json:"reference_number,omitempty"`
	ReferenceNumberSchema                   *string                    `json:"reference_number_schema,omitempty"`
	RemittanceInformation                   *[]string                  `json:"remittance_information,omitempty"`
	DebtorAccountAdditionalIdentification   *AdditionalIdentifications `json:"debtor_account_additional_identification,omitempty"`
	CreditorAccountAdditionalIdentification *AdditionalIdentifications `json:"creditor_account_additional_identification,omitempty"`
	ExchangeRate                            *ExchangeRate              `json:"exchange_rate,omitempty"`
	Note                                    *string                    `json:"note,omitempty"`
	TransactionID                           NullableString             `json:"-"`
}

type accountTransactionWire struct {
	EntryReference                          *string                    `json:"entry_reference,omitempty"`
	MerchantCategoryCode                    *string                    `json:"merchant_category_code,omitempty"`
	TransactionAmount                       Amount                     `json:"transaction_amount"`
	Creditor                                *PartyIdentification       `json:"creditor,omitempty"`
	CreditorAccount                         *AccountIdentification     `json:"creditor_account,omitempty"`
	CreditorAgent                           *FinancialInstitution      `json:"creditor_agent,omitempty"`
	Debtor                                  *PartyIdentification       `json:"debtor,omitempty"`
	DebtorAccount                           *AccountIdentification     `json:"debtor_account,omitempty"`
	DebtorAgent                             *FinancialInstitution      `json:"debtor_agent,omitempty"`
	BankTransactionCode                     *BankTransactionCode       `json:"bank_transaction_code,omitempty"`
	CreditDebitIndicator                    string                     `json:"credit_debit_indicator"`
	Status                                  string                     `json:"status"`
	BookingDate                             *string                    `json:"booking_date,omitempty"`
	ValueDate                               *string                    `json:"value_date,omitempty"`
	TransactionDate                         *string                    `json:"transaction_date,omitempty"`
	BalanceAfterTransaction                 *TransactionAmount         `json:"balance_after_transaction,omitempty"`
	ReferenceNumber                         *string                    `json:"reference_number,omitempty"`
	ReferenceNumberSchema                   *string                    `json:"reference_number_schema,omitempty"`
	RemittanceInformation                   *[]string                  `json:"remittance_information,omitempty"`
	DebtorAccountAdditionalIdentification   *AdditionalIdentifications `json:"debtor_account_additional_identification,omitempty"`
	CreditorAccountAdditionalIdentification *AdditionalIdentifications `json:"creditor_account_additional_identification,omitempty"`
	ExchangeRate                            *ExchangeRate              `json:"exchange_rate,omitempty"`
	Note                                    *string                    `json:"note,omitempty"`
	TransactionID                           NullableString             `json:"transaction_id"`
}

func (transaction *AccountTransaction) UnmarshalJSON(data []byte) error {
	var value accountTransactionWire
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode transaction: %w", err)
	}
	*transaction = AccountTransaction(value)
	return nil
}

func (transaction *AccountTransaction) MarshalJSON() ([]byte, error) {
	transactionID := transaction.TransactionID
	if !transactionID.Present {
		return json.Marshal(struct {
			accountTransactionWire

			TransactionID *NullableString `json:"transaction_id,omitempty"`
		}{accountTransactionWire: transaction.asWire()})
	}
	return json.Marshal(struct {
		accountTransactionWire

		TransactionID *NullableString `json:"transaction_id"`
	}{accountTransactionWire: transaction.asWire(), TransactionID: &transactionID})
}

func (transaction *AccountTransaction) asWire() accountTransactionWire {
	value := accountTransactionWire(*transaction)
	value.TransactionID = NullableString{}
	return value
}

// AdditionalIdentifications retains either documented collection form. The
// schema declares an array while the official transaction example uses one object.
type AdditionalIdentifications struct {
	Values []GenericIdentification
	Array  bool
}

func (identifications *AdditionalIdentifications) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errorsInvalidJSONValue("additional account identification")
	}
	if trimmed[0] == '{' {
		var value GenericIdentification
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return fmt.Errorf("decode additional account identification object: %w", err)
		}
		identifications.Values = []GenericIdentification{value}
		identifications.Array = false
		return nil
	}
	if trimmed[0] != '[' {
		return errorsInvalidJSONValue("additional account identification")
	}
	if err := json.Unmarshal(trimmed, &identifications.Values); err != nil {
		return fmt.Errorf("decode additional account identification array: %w", err)
	}
	identifications.Array = true
	return nil
}

func (identifications AdditionalIdentifications) MarshalJSON() ([]byte, error) {
	if identifications.Array {
		return json.Marshal(identifications.Values)
	}
	if len(identifications.Values) != 1 {
		return nil, errorsInvalidJSONValue("additional account identification")
	}
	return json.Marshal(identifications.Values[0])
}

// BankTransactionCode models a bank transaction code.
type BankTransactionCode struct {
	Description *string `json:"description,omitempty"`
	Code        *string `json:"code,omitempty"`
	SubCode     *string `json:"sub_code,omitempty"`
}

// ExchangeRate models a transaction exchange-rate resource.
type ExchangeRate struct {
	UnitCurrency           *string            `json:"unit_currency,omitempty"`
	ExchangeRate           *string            `json:"exchange_rate,omitempty"`
	RateType               *string            `json:"rate_type,omitempty"`
	ContractIdentification *string            `json:"contract_identification,omitempty"`
	InstructedAmount       *TransactionAmount `json:"instructed_amount,omitempty"`
}

// NullableString distinguishes an omitted value from the documented explicit
// null continuation key and a present (including empty) string.
type NullableString struct {
	Present bool
	Null    bool
	Value   string
}

func (value *NullableString) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Null = true
		value.Value = ""
		return nil
	}
	if err := json.Unmarshal(data, &value.Value); err != nil {
		return fmt.Errorf("decode nullable string: %w", err)
	}
	value.Null = false
	return nil
}

func (value *NullableString) MarshalJSON() ([]byte, error) {
	if value.Null {
		return []byte("null"), nil
	}
	return json.Marshal(value.Value)
}

// String returns the continuation string, treating null and omitted as empty.
func (value *NullableString) String() string {
	return value.Value
}

// GetAccountTransactionsResponse models a documented transactions page.
type GetAccountTransactionsResponse struct {
	ContinuationKey NullableString       `json:"-"`
	Transactions    []AccountTransaction `json:"transactions"`
}

func (response *GetAccountTransactionsResponse) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode transactions response: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return errorsInvalidJSONValue("transactions response")
	}
	response.ContinuationKey = NullableString{}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return fmt.Errorf("decode transactions response field: %w", err)
		}
		field, ok := token.(string)
		if !ok {
			return errorsInvalidJSONValue("transactions response field")
		}
		switch field {
		case "transactions":
			if err = decoder.Decode(&response.Transactions); err != nil {
				return fmt.Errorf("decode transactions: %w", err)
			}
		case "continuation_key":
			if err = decoder.Decode(&response.ContinuationKey); err != nil {
				return fmt.Errorf("decode continuation key: %w", err)
			}
		default:
			if err = discardJSONValue(decoder); err != nil {
				return fmt.Errorf("discard undocumented transactions response field: %w", err)
			}
		}
	}
	if _, err = decoder.Token(); err != nil {
		return fmt.Errorf("decode transactions response close: %w", err)
	}
	return nil
}

func (response GetAccountTransactionsResponse) MarshalJSON() ([]byte, error) {
	var continuationKey *NullableString
	if response.ContinuationKey.Present {
		continuationKey = &response.ContinuationKey
	}
	return json.Marshal(struct {
		Transactions    []AccountTransaction `json:"transactions"`
		ContinuationKey *NullableString      `json:"continuation_key,omitempty"`
	}{
		Transactions:    response.Transactions,
		ContinuationKey: continuationKey,
	})
}

func discardJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		for decoder.More() {
			if _, err = decoder.Token(); err != nil {
				return err
			}
			if err = discardJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
	case '[':
		for decoder.More() {
			if err = discardJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
	}
	return err
}
