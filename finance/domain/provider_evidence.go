package domain

import "time"

// ProviderEvidenceSubject identifies the finance object an observation describes.
// An empty value represents a legacy row whose subject is unknown.
type ProviderEvidenceSubject string

const (
	ProviderEvidenceSubjectAccount     ProviderEvidenceSubject = "account"
	ProviderEvidenceSubjectTransaction ProviderEvidenceSubject = "transaction"
	ProviderEvidenceSubjectConnection  ProviderEvidenceSubject = "connection"
)

// ProviderEvidence is a sanitized provider observation attached to a finance record.
type ProviderEvidence struct {
	ID                   string
	TenantID             string
	ConnectionID         string
	FinanceAccountID     string
	FinanceTransactionID string
	Subject              ProviderEvidenceSubject
	Scope                RawPayloadScope
	ProviderObjectID     string
	PayloadJSON          []byte
	CapturedAt           time.Time
}
