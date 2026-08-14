package client

// Account models an Enable Banking account.
type Account struct {
	UID                  string                         `json:"uid,omitempty"`
	Name                 string                         `json:"name,omitempty"`
	Details              string                         `json:"details,omitempty"`
	Usage                string                         `json:"usage,omitempty"`
	CashAccountType      string                         `json:"cash_account_type,omitempty"`
	Product              string                         `json:"product,omitempty"`
	Currency             string                         `json:"currency,omitempty"`
	PSUStatus            string                         `json:"psu_status,omitempty"`
	CreditLimit          *Amount                        `json:"credit_limit,omitempty"`
	LegalAge             *bool                          `json:"legal_age,omitempty"`
	PostalAddress        *PostalAddress                 `json:"postal_address,omitempty"`
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
	BICFI                  string                              `json:"bic_fi,omitempty"`
	ClearingSystemMemberID *ClearingSystemMemberIdentification `json:"clearing_system_member_id,omitempty"`
	Name                   string                              `json:"name,omitempty"`
}

// ClearingSystemMemberIdentification models a clearing-system member identifier.
type ClearingSystemMemberIdentification struct {
	ClearingSystemID string `json:"clearing_system_id,omitempty"`
	MemberID         *int   `json:"member_id,omitempty"`
}

// PostalAddress models a postal address resource.
type PostalAddress struct {
	AddressType        string   `json:"address_type,omitempty"`
	Department         string   `json:"department,omitempty"`
	SubDepartment      string   `json:"sub_department,omitempty"`
	StreetName         string   `json:"street_name,omitempty"`
	BuildingNumber     string   `json:"building_number,omitempty"`
	PostCode           string   `json:"post_code,omitempty"`
	TownName           string   `json:"town_name,omitempty"`
	CountrySubDivision string   `json:"country_sub_division,omitempty"`
	Country            string   `json:"country,omitempty"`
	AddressLine        []string `json:"address_line,omitempty"`
}

// Amount models an Enable Banking amount resource.
type Amount struct {
	Amount   string `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// PartyIdentification models a transaction party.
type PartyIdentification struct {
	Name          string         `json:"name,omitempty"`
	PostalAddress *PostalAddress `json:"postal_address,omitempty"`
}
