package finance

import "errors"

var ErrProviderEvidenceNotFound = errors.New("provider evidence not found")

type ListAccountProviderEvidenceParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type GetAccountProviderEvidenceParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
	EvidenceID  string
}

type ListTransactionProviderEvidenceParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type GetTransactionProviderEvidenceParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
	EvidenceID    string
}
