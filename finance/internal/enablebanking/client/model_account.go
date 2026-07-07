package client

// Account models an Enable Banking account.
type Account struct {
	UID                  string                         `json:"uid,omitempty"`
	ID                   string                         `json:"id,omitempty"`
	Name                 string                         `json:"name,omitempty"`
	Details              string                         `json:"details,omitempty"`
	IBAN                 string                         `json:"-"`
	Currency             string                         `json:"currency,omitempty"`
	Product              string                         `json:"product,omitempty"`
	AccountID            *AccountIdentification         `json:"account_id,omitempty"`
	AllAccountIDs        []GenericAccountIdentification `json:"all_account_ids,omitempty"`
	AccountServicer      *FinancialInstitution          `json:"account_servicer,omitempty"`
	IdentificationHash   string                         `json:"identification_hash,omitempty"`
	IdentificationHashes []string                       `json:"identification_hashes,omitempty"`
}

// AccountIdentification models an account identifier resource.
type AccountIdentification struct {
	IBAN string `json:"iban,omitempty"`
}

// GenericAccountIdentification models a non-IBAN account identifier.
type GenericAccountIdentification struct {
	Identification string `json:"identification,omitempty"`
	SchemeName     string `json:"scheme_name,omitempty"`
}

// FinancialInstitution models account servicer identification.
type FinancialInstitution struct {
	BICFI string `json:"bic_fi,omitempty"`
	Name  string `json:"name,omitempty"`
}
