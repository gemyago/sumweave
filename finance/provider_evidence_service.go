package finance

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type providerEvidenceServiceStore interface {
	IsTenantMember(context.Context, string, string) (bool, error)
	ListAccountProviderEvidence(context.Context, string, string) ([]domain.ProviderEvidence, error)
	GetAccountProviderEvidence(context.Context, string, string, string) (domain.ProviderEvidence, error)
	ListTransactionProviderEvidence(context.Context, string, string) ([]domain.ProviderEvidence, error)
	GetTransactionProviderEvidence(context.Context, string, string, string) (domain.ProviderEvidence, error)
}

// ProviderEvidenceService exposes bounded, tenant-authorized provider evidence reads.
type ProviderEvidenceService struct {
	store  providerEvidenceServiceStore
	access *accessGuard
}

func NewProviderEvidenceService(store providerEvidenceServiceStore) *ProviderEvidenceService {
	return &ProviderEvidenceService{store: store, access: newAccessGuard(store)}
}

func (s *ProviderEvidenceService) ListAccountProviderEvidence(
	ctx context.Context,
	params ListAccountProviderEvidenceParams,
) ([]domain.ProviderEvidence, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListAccountProviderEvidence(ctx, params.TenantID, params.AccountID)
	if err != nil {
		return nil, mapProviderEvidenceStoreError(err)
	}
	return providerEvidenceResponseItems(items)
}

func (s *ProviderEvidenceService) GetAccountProviderEvidence(
	ctx context.Context,
	params GetAccountProviderEvidenceParams,
) (domain.ProviderEvidence, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.ProviderEvidence{}, err
	}
	item, err := s.store.GetAccountProviderEvidence(
		ctx, params.TenantID, params.AccountID, params.EvidenceID,
	)
	if err != nil {
		return domain.ProviderEvidence{}, mapProviderEvidenceStoreError(err)
	}
	return providerEvidenceResponseItem(item)
}

func (s *ProviderEvidenceService) ListTransactionProviderEvidence(
	ctx context.Context,
	params ListTransactionProviderEvidenceParams,
) ([]domain.ProviderEvidence, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListTransactionProviderEvidence(ctx, params.TenantID, params.TransactionID)
	if err != nil {
		return nil, mapProviderEvidenceStoreError(err)
	}
	return providerEvidenceResponseItems(items)
}

func (s *ProviderEvidenceService) GetTransactionProviderEvidence(
	ctx context.Context,
	params GetTransactionProviderEvidenceParams,
) (domain.ProviderEvidence, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.ProviderEvidence{}, err
	}
	item, err := s.store.GetTransactionProviderEvidence(
		ctx, params.TenantID, params.TransactionID, params.EvidenceID,
	)
	if err != nil {
		return domain.ProviderEvidence{}, mapProviderEvidenceStoreError(err)
	}
	return providerEvidenceResponseItem(item)
}

func providerEvidenceResponseItems(items []domain.ProviderEvidence) ([]domain.ProviderEvidence, error) {
	result := make([]domain.ProviderEvidence, 0, len(items))
	for _, item := range items {
		redacted, err := providerEvidenceResponseItem(item)
		if err != nil {
			return nil, err
		}
		result = append(result, redacted)
	}
	return result, nil
}

func providerEvidenceResponseItem(item domain.ProviderEvidence) (domain.ProviderEvidence, error) {
	sanitizedPayload, err := domain.SanitizeProviderEvidenceJSON(item.PayloadJSON)
	if err != nil {
		return domain.ProviderEvidence{}, fmt.Errorf("sanitize provider evidence response: %w", err)
	}
	item.PayloadJSON = sanitizedPayload
	return item, nil
}

func mapProviderEvidenceStoreError(err error) error {
	switch {
	case errors.Is(err, persistence.ErrAccountNotFound):
		return ErrAccountNotFound
	case errors.Is(err, persistence.ErrTransactionNotFound):
		return ErrTransactionNotFound
	case errors.Is(err, persistence.ErrProviderEvidenceNotFound):
		return ErrProviderEvidenceNotFound
	default:
		return err
	}
}
