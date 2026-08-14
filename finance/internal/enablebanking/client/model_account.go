package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Account models the documented AccountResource response schema.
type Account struct {
	AccountID            *AccountIdentification   `json:"account_id,omitempty"`
	AllAccountIDs        *[]GenericIdentification `json:"all_account_ids,omitempty"`
	AccountServicer      *FinancialInstitution    `json:"account_servicer,omitempty"`
	Name                 *string                  `json:"name,omitempty"`
	Details              *string                  `json:"details,omitempty"`
	Usage                *string                  `json:"usage,omitempty"`
	CashAccountType      string                   `json:"cash_account_type"`
	Product              *string                  `json:"product,omitempty"`
	Currency             string                   `json:"currency"`
	PSUStatus            *string                  `json:"psu_status,omitempty"`
	CreditLimit          *Amount                  `json:"credit_limit,omitempty"`
	LegalAge             NullableBool             `json:"-"`
	PostalAddress        *PostalAddress           `json:"postal_address,omitempty"`
	UID                  *string                  `json:"uid,omitempty"`
	IdentificationHash   string                   `json:"identification_hash"`
	IdentificationHashes []string                 `json:"identification_hashes"`
}

// NullableBool distinguishes an omitted value from an explicit null and a
// present boolean, including false.
type NullableBool struct {
	Present bool
	Null    bool
	Value   bool
}

func (value *NullableBool) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Null = true
		value.Value = false
		return nil
	}
	if err := json.Unmarshal(data, &value.Value); err != nil {
		return fmt.Errorf("decode nullable bool: %w", err)
	}
	value.Null = false
	return nil
}

func (value NullableBool) MarshalJSON() ([]byte, error) {
	if value.Null {
		return []byte("null"), nil
	}
	return json.Marshal(value.Value)
}

type accountWire struct {
	AccountID            *AccountIdentification   `json:"account_id,omitempty"`
	AllAccountIDs        *[]GenericIdentification `json:"all_account_ids,omitempty"`
	AccountServicer      *FinancialInstitution    `json:"account_servicer,omitempty"`
	Name                 *string                  `json:"name,omitempty"`
	Details              *string                  `json:"details,omitempty"`
	Usage                *string                  `json:"usage,omitempty"`
	CashAccountType      string                   `json:"cash_account_type"`
	Product              *string                  `json:"product,omitempty"`
	Currency             string                   `json:"currency"`
	PSUStatus            *string                  `json:"psu_status,omitempty"`
	CreditLimit          *Amount                  `json:"credit_limit,omitempty"`
	LegalAge             NullableBool             `json:"legal_age"`
	PostalAddress        *PostalAddress           `json:"postal_address,omitempty"`
	UID                  *string                  `json:"uid,omitempty"`
	IdentificationHash   string                   `json:"identification_hash"`
	IdentificationHashes []string                 `json:"identification_hashes"`
}

func (account *Account) UnmarshalJSON(data []byte) error {
	var value accountWire
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode account: %w", err)
	}
	*account = Account(value)
	return nil
}

func (account *Account) MarshalJSON() ([]byte, error) {
	legalAge := account.LegalAge
	if !legalAge.Present {
		return json.Marshal(struct {
			accountWire

			LegalAge *NullableBool `json:"legal_age,omitempty"`
		}{accountWire: account.asWire()})
	}
	return json.Marshal(struct {
		accountWire

		LegalAge *NullableBool `json:"legal_age"`
	}{accountWire: account.asWire(), LegalAge: &legalAge})
}

func (account *Account) asWire() accountWire {
	value := accountWire(*account)
	value.LegalAge = NullableBool{}
	return value
}

// AccountIdentification models an account identifier resource.
type AccountIdentification struct {
	IBAN  *string                `json:"iban,omitempty"`
	Other *GenericIdentification `json:"other,omitempty"`
}

// GenericIdentification models a non-IBAN account identifier.
type GenericIdentification struct {
	Identification string  `json:"identification"`
	SchemeName     string  `json:"scheme_name"`
	Issuer         *string `json:"issuer,omitempty"`
}

// GenericAccountIdentification is retained as a source-compatible alias for
// provider operations that use the same documented identifier schema.
type GenericAccountIdentification = GenericIdentification

// FinancialInstitution models account servicer identification.
type FinancialInstitution struct {
	BICFI                  *string                             `json:"bic_fi,omitempty"`
	ClearingSystemMemberID *ClearingSystemMemberIdentification `json:"clearing_system_member_id,omitempty"`
	Name                   *string                             `json:"name,omitempty"`
}

// ClearingSystemMemberIdentification models a clearing-system member identifier.
type ClearingSystemMemberIdentification struct {
	ClearingSystemID *string           `json:"clearing_system_id,omitempty"`
	MemberID         *ClearingMemberID `json:"member_id,omitempty"`
}

// ClearingMemberID retains either official member_id representation. The schema
// declares a string while the official response example uses a number.
type ClearingMemberID struct {
	String *string
	Number *json.Number
}

func (id *ClearingMemberID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errorsInvalidJSONValue("clearing member ID")
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode clearing member ID string: %w", err)
		}
		id.String = &value
		id.Number = nil
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value json.Number
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode clearing member ID number: %w", err)
	}
	if _, err := value.Float64(); err != nil {
		return errorsInvalidJSONValue("clearing member ID")
	}
	id.Number = &value
	id.String = nil
	return nil
}

func (id ClearingMemberID) MarshalJSON() ([]byte, error) {
	if id.String != nil {
		return json.Marshal(*id.String)
	}
	if id.Number != nil {
		return []byte(*id.Number), nil
	}
	return nil, errorsInvalidJSONValue("clearing member ID")
}

// PostalAddress models a postal address resource.
type PostalAddress struct {
	AddressType        *string   `json:"address_type,omitempty"`
	Department         *string   `json:"department,omitempty"`
	SubDepartment      *string   `json:"sub_department,omitempty"`
	StreetName         *string   `json:"street_name,omitempty"`
	BuildingNumber     *string   `json:"building_number,omitempty"`
	PostCode           *string   `json:"post_code,omitempty"`
	TownName           *string   `json:"town_name,omitempty"`
	CountrySubDivision *string   `json:"country_sub_division,omitempty"`
	Country            *string   `json:"country,omitempty"`
	AddressLine        *[]string `json:"address_line,omitempty"`
}

// Amount models the documented AmountType resource.
type Amount struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// PartyIdentification models a transaction party.
type PartyIdentification struct {
	Name           *string                `json:"name,omitempty"`
	PostalAddress  *PostalAddress         `json:"postal_address,omitempty"`
	OrganisationID *GenericIdentification `json:"organisation_id,omitempty"`
	PrivateID      *GenericIdentification `json:"private_id,omitempty"`
	ContactDetails *ContactDetails        `json:"contact_details,omitempty"`
}

// ContactDetails models a party's documented contact details.
type ContactDetails struct {
	EmailAddress *string `json:"email_address,omitempty"`
	PhoneNumber  *string `json:"phone_number,omitempty"`
}

func errorsInvalidJSONValue(name string) error {
	return fmt.Errorf("invalid %s JSON value", name)
}
