package client

// ASPSP models the session-response ASPSP schema.
type ASPSP struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}

// ASPSPData models a list-ASPSP item, which is distinct from the session schema.
type ASPSPData struct {
	Name                   string                 `json:"name"`
	Country                string                 `json:"country"`
	Logo                   string                 `json:"logo"`
	PSUTypes               []string               `json:"psu_types"`
	AuthMethods            []AuthMethod           `json:"auth_methods"`
	MaximumConsentValidity int                    `json:"maximum_consent_validity"`
	Sandbox                *SandboxInfo           `json:"sandbox,omitempty"`
	Beta                   bool                   `json:"beta"`
	BIC                    *string                `json:"bic,omitempty"`
	RequiredPSUHeaders     *[]string              `json:"required_psu_headers,omitempty"`
	Payments               *[]ResponsePaymentType `json:"payments,omitempty"`
	Group                  *ASPSPGroup            `json:"group,omitempty"`
}

// ListASPSPsResponse models the ASPSP list response.
type ListASPSPsResponse struct {
	ASPSPs []ASPSPData `json:"aspsps"`
}

// AuthMethod models an ASPSP authentication method.
type AuthMethod struct {
	Name         *string       `json:"name,omitempty"`
	Title        *string       `json:"title,omitempty"`
	PSUType      string        `json:"psu_type"`
	Credentials  *[]Credential `json:"credentials,omitempty"`
	Approach     string        `json:"approach"`
	HiddenMethod bool          `json:"hidden_method"`
}

// Credential models a credential accepted by an authentication method.
type Credential struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Required    bool    `json:"required"`
	Description *string `json:"description,omitempty"`
	Template    *string `json:"template,omitempty"`
}

// SandboxInfo models ASPSP sandbox access information.
type SandboxInfo struct {
	Users *[]SandboxUser `json:"users,omitempty"`
}

// SandboxUser models sandbox test credentials.
type SandboxUser struct {
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	OTP      *string `json:"otp,omitempty"`
}

// ASPSPGroup models the group to which an ASPSP belongs.
type ASPSPGroup struct {
	Name string `json:"name"`
	Logo string `json:"logo"`
}

// ResponsePaymentType models payment capabilities offered by an ASPSP.
type ResponsePaymentType struct {
	PaymentType                                 string                           `json:"payment_type"`
	MaxTransactions                             *int                             `json:"max_transactions,omitempty"`
	Currencies                                  *[]string                        `json:"currencies,omitempty"`
	DebtorAccountRequired                       *bool                            `json:"debtor_account_required,omitempty"`
	DebtorAccountSchemas                        *[]string                        `json:"debtor_account_schemas,omitempty"`
	CreditorAccountSchemas                      *[]string                        `json:"creditor_account_schemas,omitempty"`
	PriorityCodes                               *[]string                        `json:"priority_codes,omitempty"`
	ChargeBearerValues                          *[]string                        `json:"charge_bearer_values,omitempty"`
	CreditorCountryRequired                     *bool                            `json:"creditor_country_required,omitempty"`
	CreditorNameRequired                        *bool                            `json:"creditor_name_required,omitempty"`
	CreditorPostalAddressRequired               *bool                            `json:"creditor_postal_address_required,omitempty"`
	RemittanceInformationRequired               *bool                            `json:"remittance_information_required,omitempty"`
	RemittanceInformationLines                  *[]RemittanceInformationLineInfo `json:"remittance_information_lines,omitempty"`
	DebtorCurrencyRequired                      *bool                            `json:"debtor_currency_required,omitempty"`
	DebtorContactEmailRequired                  *bool                            `json:"debtor_contact_email_required,omitempty"`
	DebtorContactPhoneRequired                  *bool                            `json:"debtor_contact_phone_required,omitempty"`
	CreditorAgentBICFIRequired                  *bool                            `json:"creditor_agent_bic_fi_required,omitempty"`
	CreditorAgentClearingSystemMemberIDRequired *bool                            `json:"creditor_agent_clearing_system_member_id_required,omitempty"`
	AllowedAuthMethods                          *[]string                        `json:"allowed_auth_methods,omitempty"`
	RegulatoryReportingCodes                    *[]RegulatoryReportingCode       `json:"regulatory_reporting_codes,omitempty"`
	RegulatoryReportingCodeRequired             *bool                            `json:"regulatory_reporting_code_required,omitempty"`
	ReferenceNumberSupported                    *bool                            `json:"reference_number_supported,omitempty"`
	ReferenceNumberSchemas                      *[]string                        `json:"reference_number_schemas,omitempty"`
	RequestedExecutionDateSupported             *bool                            `json:"requested_execution_date_supported,omitempty"`
	RequestedExecutionDateMaxPeriod             *int                             `json:"requested_execution_date_max_period,omitempty"`
	RemittanceReferenceSupported                *bool                            `json:"remittance_reference_supported,omitempty"`
	DeferredSubmissionSupported                 *bool                            `json:"deferred_submission_supported,omitempty"`
	FinalSuccessfulStatuses                     *[]string                        `json:"final_successful_statuses,omitempty"`
	PSUType                                     string                           `json:"psu_type"`
}

// RemittanceInformationLineInfo models constraints for a remittance line.
type RemittanceInformationLineInfo struct {
	MinLength *int    `json:"min_length,omitempty"`
	MaxLength *int    `json:"max_length,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
}

// RegulatoryReportingCode models a supported regulatory reporting code.
type RegulatoryReportingCode struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}
