package finance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	internalproviders "github.com/gemyago/signal-foundry/finance/internal/providers"
	"github.com/gemyago/signal-foundry/finance/persistence"
)

type BankConnectionService struct {
	access             *accessGuard
	pendingStartLookup bankConnectionPendingStartLookup
	linkCoordinator    bankConnectionLinkCoordinator
}

type bankConnectionServiceArgs struct {
	Store                   *persistence.Store
	ConnectionSecretCipher  connectionSecretCipher
	ConnectorRegistry       bankConnectionConnectorRegistry
	ProviderProfileRegistry bankConnectionProviderProfileRegistry
	Now                     func() time.Time
	NewID                   func() string
}

type bankConnectionConnectorRegistry interface {
	Resolve(connectorID domain.ProviderConnectorID) (internalproviders.Connector, error)
}

type bankConnectionProviderProfileRegistry = internalproviders.ProviderProfileRegistry

type bankConnectionPendingStartLookup interface {
	GetPendingStartByState(
		ctx context.Context,
		providerID domain.ProviderID,
		state string,
	) (*domain.PendingBankConnectionLinkStart, error)
}

type bankConnectionLinkCoordinator interface {
	StartRedirectLink(
		ctx context.Context,
		request internalproviders.RedirectLinkStartRequest,
	) (internalproviders.StartLinkResult, error)
	FinishRedirectLink(
		ctx context.Context,
		request internalproviders.RedirectLinkFinishRequest,
	) (domain.BankConnection, error)
	LinkToken(
		ctx context.Context,
		request internalproviders.TokenLinkRequest,
	) (domain.BankConnection, error)
}

func newBankConnectionService(args bankConnectionServiceArgs) (*BankConnectionService, error) {
	linkPersistence := persistence.NewProviderLinkPersistence(args.Store)
	coordinator, err := internalproviders.NewLinkCoordinator(internalproviders.LinkCoordinatorArgs{
		ProviderProfileRegistry: args.ProviderProfileRegistry,
		ConnectorRegistry:       args.ConnectorRegistry,
		PendingStartStore:       linkPersistence,
		ConnectionSecretWriter: newBankConnectionSecretWriter(
			args.Store,
			args.ConnectionSecretCipher,
			args.Now,
			args.NewID,
		),
		ConnectionStore:  linkPersistence,
		RawPayloadWriter: linkPersistence,
		Now:              args.Now,
		NewID:            args.NewID,
		PendingStartTTL:  pendingBankConnectionLinkStartTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("create bank connection link coordinator: %w", err)
	}
	return &BankConnectionService{
		access:             newAccessGuard(args.Store),
		pendingStartLookup: linkPersistence,
		linkCoordinator:    coordinator,
	}, nil
}

func (s *BankConnectionService) LinkTokenBankConnection(
	ctx context.Context,
	params LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	connection, err := s.linkCoordinator.LinkToken(ctx, internalproviders.TokenLinkRequest{
		TenantID:    params.TenantID,
		ActorUserID: params.ActorUserID,
		ProviderID:  domain.ProviderID(params.Provider),
		Token:       params.Token,
	})
	if err != nil {
		if isBankProviderConfigurationError(err) {
			return domain.BankConnection{}, fmt.Errorf("%w: %s", ErrBankProviderNotConfigured, params.Provider)
		}
		return domain.BankConnection{}, fmt.Errorf("link token bank connection: %w", err)
	}
	return connection, nil
}

func (s *BankConnectionService) StartBankConnectionLink(
	ctx context.Context,
	params StartBankConnectionLinkParams,
) (ProviderLinkStart, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return ProviderLinkStart{}, err
	}
	result, err := s.linkCoordinator.StartRedirectLink(ctx, internalproviders.RedirectLinkStartRequest{
		TenantID:           params.TenantID,
		ActorUserID:        params.ActorUserID,
		ProviderID:         domain.ProviderID(params.Provider),
		RedirectURL:        params.RedirectURL,
		BrowserCallbackURL: params.BrowserCallbackURL,
	})
	if err != nil {
		if isBankProviderConfigurationError(err) {
			return ProviderLinkStart{}, fmt.Errorf("%w: %s", ErrBankProviderNotConfigured, params.Provider)
		}
		return ProviderLinkStart{}, fmt.Errorf("start bank connection link: %w", err)
	}
	return ProviderLinkStart{
		State:            result.State,
		AuthorizationURL: result.AuthorizationURL,
		RawPayloads:      providerRawPayloadsFromObservations(result.RawPayloads),
	}, nil
}

func (s *BankConnectionService) FinishBankConnectionLink(
	ctx context.Context,
	params FinishBankConnectionLinkParams,
) (domain.BankConnection, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	connection, err := s.linkCoordinator.FinishRedirectLink(ctx, internalproviders.RedirectLinkFinishRequest{
		TenantID:    params.TenantID,
		ActorUserID: params.ActorUserID,
		ProviderID:  domain.ProviderID(params.Provider),
		State:       params.State,
		Code:        params.Code,
	})
	if err != nil {
		if errors.Is(err, internalproviders.ErrPendingStartNotFound) {
			return domain.BankConnection{}, ErrPendingBankConnectionLinkStartNotFound
		}
		if isBankProviderConfigurationError(err) {
			return domain.BankConnection{}, fmt.Errorf("%w: %s", ErrBankProviderNotConfigured, params.Provider)
		}
		return domain.BankConnection{}, fmt.Errorf("finish bank connection link: %w", err)
	}
	return connection, nil
}

func (s *BankConnectionService) GetPendingBankConnectionLinkStartByState(
	ctx context.Context,
	params GetPendingBankConnectionLinkStartByStateParams,
) (domain.PendingBankConnectionLinkStart, error) {
	pendingStart, err := s.pendingStartLookup.GetPendingStartByState(
		ctx,
		domain.ProviderID(params.Provider),
		params.State,
	)
	if err != nil {
		if errors.Is(err, persistence.ErrPendingBankConnectionLinkStartNotFound) {
			return domain.PendingBankConnectionLinkStart{}, ErrPendingBankConnectionLinkStartNotFound
		}
		return domain.PendingBankConnectionLinkStart{}, fmt.Errorf(
			"get pending bank connection link start by state: %w",
			err,
		)
	}
	return *pendingStart, nil
}

func providerRawPayloadsFromObservations(
	payloads []domain.ProviderRawPayloadObservation,
) []ProviderRawPayload {
	items := make([]ProviderRawPayload, 0, len(payloads))
	for _, payload := range payloads {
		items = append(items, ProviderRawPayload{
			Scope:            payload.Scope,
			ProviderObjectID: payload.ProviderObjectID,
			PayloadJSON:      payload.PayloadJSON,
		})
	}
	return items
}

func isBankProviderConfigurationError(err error) bool {
	return errors.Is(err, internalproviders.ErrProviderNotConfigured) ||
		errors.Is(err, internalproviders.ErrConnectorNotConfigured)
}

type bankConnectionSecretWriter struct {
	store  connectionSecretStore
	cipher connectionSecretCipher
	now    func() time.Time
	newID  func() string
}

func newBankConnectionSecretWriter(
	store connectionSecretStore,
	cipher connectionSecretCipher,
	now func() time.Time,
	newID func() string,
) *bankConnectionSecretWriter {
	return &bankConnectionSecretWriter{store: store, cipher: cipher, now: now, newID: newID}
}

func (w *bankConnectionSecretWriter) SaveConnectionSecret(
	ctx context.Context,
	provider string,
	reference string,
	secret string,
) (string, error) {
	envelope, err := w.cipher.SealString(secret)
	if err != nil {
		return "", fmt.Errorf("seal connection secret: %w", err)
	}
	secretID := w.newID()
	now := w.now().UTC()
	_, err = w.store.SaveConnectionSecret(ctx, domain.ConnectionSecret{
		ID:        secretID,
		Provider:  provider,
		Reference: reference,
		Envelope:  envelope,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("save connection secret: %w", err)
	}
	return secretID, nil
}
