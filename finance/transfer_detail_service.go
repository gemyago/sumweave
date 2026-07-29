package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

const transferCandidateLimitMaximum int64 = 200

var (
	ErrInvalidTransferCandidateQuery = errors.New("invalid transfer candidate query")
	ErrTransferPartnerNotFound       = errors.New("transfer partner not found")
	ErrInvalidTransferPartner        = errors.New("invalid transfer partner")
)

type transferDetailStore interface {
	accessGuardStore
	GetTransaction(context.Context, string) (*domain.Transaction, error)
	ListCandidates(
		context.Context,
		string,
		string,
		string,
		time.Time,
		time.Time,
		int64,
		int64,
	) ([]domain.Transaction, error)
	ListTransferGroupTransactions(context.Context, string, string) ([]domain.Transaction, error)
}

// TransferDetailService reads candidate and matched-partner records for transfer detail.
type TransferDetailService struct {
	store  transferDetailStore
	access *accessGuard
}

func NewTransferDetailService(store transferDetailStore) *TransferDetailService {
	return &TransferDetailService{store: store, access: newAccessGuard(store)}
}

func (s *TransferDetailService) ListTransferCandidates(
	ctx context.Context,
	params ListTransferCandidatesParams,
) ([]domain.Transaction, error) {
	if err := validateTransferCandidateQuery(params); err != nil {
		return nil, err
	}
	source, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListCandidates(
		ctx,
		source.TenantID,
		source.ID,
		source.AccountID,
		params.EffectiveFrom,
		params.EffectiveBefore,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list transfer candidates: %w", err)
	}
	return items, nil
}

func (s *TransferDetailService) GetTransferPartner(
	ctx context.Context,
	params GetTransferPartnerParams,
) (domain.Transaction, error) {
	source, err := s.requireTenantTransaction(ctx, params.TenantID, params.ActorUserID, params.TransactionID)
	if err != nil {
		return domain.Transaction{}, err
	}
	groupID, sourceValid := matchedTransferGroupID(source)
	if !sourceValid {
		return domain.Transaction{}, ErrTransferPartnerNotFound
	}
	items, err := s.store.ListTransferGroupTransactions(ctx, source.TenantID, groupID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("list transfer partner: %w", err)
	}
	partners := make([]domain.Transaction, 0, 1)
	for _, item := range items {
		if item.ID != source.ID {
			partners = append(partners, item)
		}
	}
	if len(partners) == 0 {
		return domain.Transaction{}, ErrTransferPartnerNotFound
	}
	if len(partners) != 1 {
		return domain.Transaction{}, ErrInvalidTransferPartner
	}
	partner := partners[0]
	partnerGroupID, partnerValid := matchedTransferGroupID(partner)
	if !partnerValid || partnerGroupID != groupID {
		return domain.Transaction{}, ErrInvalidTransferPartner
	}
	return partner, nil
}

func (s *TransferDetailService) requireTenantTransaction(
	ctx context.Context,
	tenantID string,
	userID string,
	transactionID string,
) (domain.Transaction, error) {
	trimmedTenantID := strings.TrimSpace(tenantID)
	if err := s.access.requireTenantMember(ctx, trimmedTenantID, strings.TrimSpace(userID)); err != nil {
		return domain.Transaction{}, err
	}
	transaction, err := s.store.GetTransaction(ctx, strings.TrimSpace(transactionID))
	if err != nil {
		if errors.Is(err, persistence.ErrTransactionNotFound) {
			return domain.Transaction{}, ErrTransactionNotFound
		}
		return domain.Transaction{}, fmt.Errorf("get transfer transaction: %w", err)
	}
	if transaction.TenantID != trimmedTenantID {
		return domain.Transaction{}, ErrTransactionNotFound
	}
	return *transaction, nil
}

func validateTransferCandidateQuery(params ListTransferCandidatesParams) error {
	validRange := !params.EffectiveFrom.IsZero() && !params.EffectiveBefore.IsZero() &&
		params.EffectiveFrom.Before(params.EffectiveBefore)
	validPage := params.Limit >= 1 && params.Limit <= transferCandidateLimitMaximum && params.Offset >= 0
	if !validRange || !validPage {
		return ErrInvalidTransferCandidateQuery
	}
	return nil
}

func matchedTransferGroupID(transaction domain.Transaction) (string, bool) {
	if transaction.Kind != domain.TransactionKindTransfer || transaction.TransferGroupID == nil ||
		transaction.TransferMatchedAt == nil || transaction.TransferMatchedAt.IsZero() {
		return "", false
	}
	groupID := strings.TrimSpace(*transaction.TransferGroupID)
	return groupID, groupID != ""
}
