package finance

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/gemyago/sumweave/finance/persistence"
)

type providerSnapshotServiceStore interface {
	IsTenantMember(context.Context, string, string) (bool, error)
	ListAccountProviderSnapshots(context.Context, string, string) ([]domain.ProviderSnapshot, error)
	GetAccountProviderSnapshot(context.Context, string, string, string) (domain.ProviderSnapshot, error)
	ListTransactionProviderSnapshots(context.Context, string, string) ([]domain.ProviderSnapshot, error)
	GetTransactionProviderSnapshot(context.Context, string, string, string) (domain.ProviderSnapshot, error)
}

// ProviderSnapshotService exposes bounded, tenant-authorized provider source-data reads.
type ProviderSnapshotService struct {
	store  providerSnapshotServiceStore
	access *accessGuard
}

func NewProviderSnapshotService(store providerSnapshotServiceStore) *ProviderSnapshotService {
	return &ProviderSnapshotService{store: store, access: newAccessGuard(store)}
}

func (s *ProviderSnapshotService) ListAccountProviderSnapshots(
	ctx context.Context,
	params ListAccountProviderSnapshotsParams,
) ([]domain.ProviderSnapshot, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListAccountProviderSnapshots(ctx, params.TenantID, params.AccountID)
	if err != nil {
		return nil, mapProviderSnapshotStoreError(err)
	}
	return providerSnapshotMetadata(items), nil
}

func (s *ProviderSnapshotService) GetAccountProviderSnapshot(
	ctx context.Context,
	params GetAccountProviderSnapshotParams,
) (domain.ProviderSnapshot, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.ProviderSnapshot{}, err
	}
	item, err := s.store.GetAccountProviderSnapshot(ctx, params.TenantID, params.AccountID, params.SnapshotID)
	if err != nil {
		return domain.ProviderSnapshot{}, mapProviderSnapshotStoreError(err)
	}
	return providerSnapshotDetail(item)
}

func (s *ProviderSnapshotService) ListTransactionProviderSnapshots(
	ctx context.Context,
	params ListTransactionProviderSnapshotsParams,
) ([]domain.ProviderSnapshot, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListTransactionProviderSnapshots(ctx, params.TenantID, params.TransactionID)
	if err != nil {
		return nil, mapProviderSnapshotStoreError(err)
	}
	return providerSnapshotMetadata(items), nil
}

func (s *ProviderSnapshotService) GetTransactionProviderSnapshot(
	ctx context.Context,
	params GetTransactionProviderSnapshotParams,
) (domain.ProviderSnapshot, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.ProviderSnapshot{}, err
	}
	item, err := s.store.GetTransactionProviderSnapshot(
		ctx, params.TenantID, params.TransactionID, params.SnapshotID,
	)
	if err != nil {
		return domain.ProviderSnapshot{}, mapProviderSnapshotStoreError(err)
	}
	return providerSnapshotDetail(item)
}

func providerSnapshotMetadata(items []domain.ProviderSnapshot) []domain.ProviderSnapshot {
	metadata := make([]domain.ProviderSnapshot, 0, len(items))
	for _, item := range items {
		item.DocumentJSON = nil
		metadata = append(metadata, item)
	}
	return metadata
}

func providerSnapshotDetail(item domain.ProviderSnapshot) (domain.ProviderSnapshot, error) {
	sanitizedDocument, err := domain.SanitizeProviderSnapshotJSON(item.DocumentJSON)
	if err != nil {
		return domain.ProviderSnapshot{}, fmt.Errorf("sanitize provider snapshot response: %w", err)
	}
	item.DocumentJSON = sanitizedDocument
	return item, nil
}

func mapProviderSnapshotStoreError(err error) error {
	switch {
	case errors.Is(err, persistence.ErrAccountNotFound):
		return ErrAccountNotFound
	case errors.Is(err, persistence.ErrTransactionNotFound):
		return ErrTransactionNotFound
	case errors.Is(err, persistence.ErrProviderSnapshotNotFound):
		return ErrProviderSnapshotNotFound
	default:
		return err
	}
}
