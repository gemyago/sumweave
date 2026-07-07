package client

// GetAccountDetailsResponse models account details.
type GetAccountDetailsResponse struct {
	UID                  string                         `json:"uid,omitempty"`
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
	OwnerName            string                         `json:"-"`
	BIC                  string                         `json:"-"`
}
