package finance

import (
	"errors"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrHiddenAccount       = errors.New("account is hidden")
	ErrDuplicateTagID      = errors.New("duplicate tag id")
	ErrTagNotAssignable    = errors.New("tag is not assignable")
	ErrInvalidTransferPair = errors.New("invalid transfer pair")
	ErrTransferNotLinked   = errors.New("transfer pair is not linked")
)

type RecordTransactionParams struct {
	ActorUserID      string
	TenantID         string
	AccountID        string
	Source           domain.TransactionSource
	Status           domain.TransactionStatus
	Kind             domain.TransactionKind
	AmountMinor      int64
	Currency         string
	Description      string
	EffectiveAt      time.Time
	CategoryID       string
	TagIDs           []string
	TransferGroupID  string
	ProviderOriginal *domain.ProviderTransactionOriginal
}

type UpdateTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
	Description   string
	AmountMinor   int64
	EffectiveAt   *time.Time
	CategoryID    string
	ClearCategory bool
	TagIDs        []string
}

type HideTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type GetTransactionParams struct {
	ActorUserID   string
	TenantID      string
	TransactionID string
}

type LinkTransfersParams struct {
	ActorUserID         string
	TenantID            string
	FirstTransactionID  string
	SecondTransactionID string
}

type UnlinkTransfersParams struct {
	ActorUserID         string
	TenantID            string
	FirstTransactionID  string
	SecondTransactionID string
}

type ListTransactionsParams struct {
	ActorUserID   string
	TenantID      string
	AccountID     string
	Source        domain.TransactionSource
	Status        domain.TransactionStatus
	IncludeHidden bool
	Limit         int64
	Offset        int64
}

type SummarizeTransactionsParams struct {
	ActorUserID string
	TenantID    string
}

type GetAccountBalanceParams struct {
	ActorUserID string
	TenantID    string
	AccountID   string
}

func bookedMatchedTransfer(item domain.Transaction) bool {
	if item.HiddenAt != nil ||
		item.Status != domain.TransactionStatusBooked ||
		item.Kind != domain.TransactionKindTransfer {
		return false
	}
	if item.TransferMatchedAt == nil || item.TransferMatchedAt.IsZero() {
		return false
	}
	return item.AmountMinor != 0
}

func existingTransferGroupID(
	firstTransaction domain.Transaction,
	secondTransaction domain.Transaction,
) string {
	if firstTransaction.TransferGroupID != nil {
		if groupID := strings.TrimSpace(*firstTransaction.TransferGroupID); groupID != "" {
			return groupID
		}
	}
	if secondTransaction.TransferGroupID != nil {
		if groupID := strings.TrimSpace(*secondTransaction.TransferGroupID); groupID != "" {
			return groupID
		}
	}
	return ""
}
