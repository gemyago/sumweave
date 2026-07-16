package finance

import "time"

type ListTransferCandidatesParams struct {
	ActorUserID     string
	TenantID        string
	TransactionID   string
	EffectiveFrom   time.Time
	EffectiveBefore time.Time
	Limit           int64
	Offset          int64
}

type GetTransferPartnerParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}
