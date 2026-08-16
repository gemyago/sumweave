package finance

import "errors"

var ErrProviderSnapshotNotFound = errors.New("provider snapshot not found")

type ListAccountProviderSnapshotsParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

type GetAccountProviderSnapshotParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
	SnapshotID  string
}

type ListTransactionProviderSnapshotsParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type GetTransactionProviderSnapshotParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
	SnapshotID    string
}
