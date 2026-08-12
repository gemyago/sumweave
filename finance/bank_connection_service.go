package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	enablebankingclient "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	internalproviders "github.com/gemyago/sumweave/finance/internal/providers"
	"github.com/gemyago/sumweave/finance/persistence"
)

type BankConnectionService struct {
	access             *accessGuard
	pendingStartLookup bankConnectionPendingStartLookup
	connectionStore    bankConnectionMetadataStore
	linkCoordinator    bankConnectionLinkCoordinator
	logger             *slog.Logger
	now                func() time.Time
}

type bankConnectionServiceArgs struct {
	Store                   *persistence.Store
	Logger                  *slog.Logger
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

type bankConnectionMetadataStore interface {
	UpdateBankConnectionDisplayName(
		ctx context.Context,
		tenantID string,
		connectionID string,
		displayName string,
		updatedAt time.Time,
	) error
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
		Logger:           args.Logger,
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
		connectionStore:    linkPersistence,
		linkCoordinator:    coordinator,
		logger:             args.Logger.With("component", "bankConnectionService"),
		now:                args.Now,
	}, nil
}

func (s *BankConnectionService) LinkTokenBankConnection(
	ctx context.Context,
	params LinkTokenBankConnectionParams,
) (domain.BankConnection, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return domain.BankConnection{}, err
	}
	s.logger.InfoContext(ctx, "bank connection link requested",
		slog.String("operation", "token"), slog.String("tenantId", params.TenantID),
		slog.String("actorUserId", params.ActorUserID), slog.String("provider", params.Provider))
	connection, err := s.linkCoordinator.LinkToken(ctx, internalproviders.TokenLinkRequest{
		TenantID:    params.TenantID,
		ActorUserID: params.ActorUserID,
		ProviderID:  domain.ProviderID(params.Provider),
		Token:       params.Token,
	})
	if err != nil {
		s.logger.WarnContext(ctx, "bank connection link failed",
			slog.String("operation", "token"), slog.String("failureStage", "coordinator"),
			slog.String("tenantId", params.TenantID), slog.String("actorUserId", params.ActorUserID),
			slog.String("provider", params.Provider), slog.Any("err", err))
		if isBankProviderConfigurationError(err) {
			return domain.BankConnection{}, fmt.Errorf(
				"%w: %s",
				ErrBankProviderNotConfigured,
				params.Provider,
			)
		}
		return domain.BankConnection{}, fmt.Errorf("link token bank connection: %w", err)
	}
	s.logger.InfoContext(ctx, "bank connection token link completed",
		slog.String("tenantId", params.TenantID), slog.String("actorUserId", params.ActorUserID),
		slog.String("provider", params.Provider), slog.String("connectionId", connection.ID))
	return connection, nil
}

func (s *BankConnectionService) StartBankConnectionLink(
	ctx context.Context,
	params StartBankConnectionLinkParams,
) (ProviderLinkStart, error) {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return ProviderLinkStart{}, err
	}

	s.logger.InfoContext(ctx, "bank connection link requested",
		slog.String("operation", "redirectStart"), slog.String("tenantId", params.TenantID),
		slog.String("actorUserId", params.ActorUserID), slog.String("provider", params.Provider))

	result, err := s.linkCoordinator.StartRedirectLink(
		ctx,
		internalproviders.RedirectLinkStartRequest{
			TenantID:           params.TenantID,
			ActorUserID:        params.ActorUserID,
			ProviderID:         domain.ProviderID(params.Provider),
			RedirectURL:        params.RedirectURL,
			BrowserCallbackURL: params.BrowserCallbackURL,
		},
	)
	if err != nil {
		var responseErr *enablebankingclient.ResponseError
		errorClass := "coordinator"
		if errors.As(err, &responseErr) {
			errorClass = "providerResponse"
		}
		attributes := []slog.Attr{
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "coordinator"),
			slog.String("tenantId", params.TenantID),
			slog.String("actorUserId", params.ActorUserID),
			slog.String("provider", params.Provider),
			slog.String("errorClass", errorClass),
		}
		if responseErr != nil {
			attributes = append(
				attributes,
				slog.Int("providerStatus", responseErr.StatusCode),
				slog.String("providerCode", responseErr.Code),
				slog.Bool(
					"retryable",
					responseErr.StatusCode == http.StatusTooManyRequests ||
						responseErr.StatusCode >= http.StatusInternalServerError,
				),
			)
		}
		attributes = append(attributes, slog.Any("err", err))
		s.logger.LogAttrs(ctx, slog.LevelWarn, "bank connection link failed", attributes...)
		if isBankProviderConfigurationError(err) {
			return ProviderLinkStart{}, fmt.Errorf(
				"%w: %s",
				ErrBankProviderNotConfigured,
				params.Provider,
			)
		}
		if providerResponseErr := providerResponseErrorForBankConnection(err); providerResponseErr != nil {
			return ProviderLinkStart{}, fmt.Errorf("start bank connection link: %w", providerResponseErr)
		}
		return ProviderLinkStart{}, fmt.Errorf("start bank connection link: %w", err)
	}
	s.logger.InfoContext(
		ctx,
		"bank connection redirect link started",
		slog.String("tenantId", params.TenantID),
		slog.String("actorUserId", params.ActorUserID),
		slog.String(
			"provider",
			params.Provider,
		),
		slog.Int("rawPayloadCount", len(result.RawPayloads)),
	)
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
	s.logger.InfoContext(ctx, "bank connection link requested",
		slog.String("operation", "redirectFinish"), slog.String("tenantId", params.TenantID),
		slog.String("actorUserId", params.ActorUserID), slog.String("provider", params.Provider))
	connection, err := s.linkCoordinator.FinishRedirectLink(
		ctx,
		internalproviders.RedirectLinkFinishRequest{
			TenantID:    params.TenantID,
			ActorUserID: params.ActorUserID,
			ProviderID:  domain.ProviderID(params.Provider),
			State:       params.State,
			Code:        params.Code,
		},
	)
	if err == nil {
		s.logger.InfoContext(ctx, "bank connection redirect link completed",
			slog.String("tenantId", params.TenantID), slog.String("actorUserId", params.ActorUserID),
			slog.String("provider", params.Provider), slog.String("connectionId", connection.ID))
		return connection, nil
	}
	var responseErr *enablebankingclient.ResponseError
	errorClass := "coordinator"
	if errors.As(err, &responseErr) {
		errorClass = "providerResponse"
	}
	attributes := []slog.Attr{
		slog.String("operation", "redirectFinish"),
		slog.String("failureStage", "coordinator"),
		slog.String("tenantId", params.TenantID),
		slog.String("actorUserId", params.ActorUserID),
		slog.String("provider", params.Provider),
		slog.String("errorClass", errorClass),
	}
	if responseErr != nil {
		attributes = append(
			attributes,
			slog.Int("providerStatus", responseErr.StatusCode),
			slog.String("providerCode", responseErr.Code),
			slog.Bool(
				"retryable",
				responseErr.StatusCode == http.StatusTooManyRequests ||
					responseErr.StatusCode >= http.StatusInternalServerError,
			),
		)
	}
	attributes = append(attributes, slog.Any("err", err))
	s.logger.LogAttrs(ctx, slog.LevelWarn, "bank connection link failed", attributes...)
	if errors.Is(err, internalproviders.ErrPendingStartNotFound) {
		return domain.BankConnection{}, ErrPendingBankConnectionLinkStartNotFound
	}
	if isBankProviderConfigurationError(err) {
		return domain.BankConnection{}, fmt.Errorf(
			"%w: %s",
			ErrBankProviderNotConfigured,
			params.Provider,
		)
	}
	if providerResponseErr := providerResponseErrorForBankConnection(err); providerResponseErr != nil {
		return domain.BankConnection{}, fmt.Errorf("finish bank connection link: %w", providerResponseErr)
	}
	return domain.BankConnection{}, fmt.Errorf("finish bank connection link: %w", err)
}

func (s *BankConnectionService) UpdateBankConnection(
	ctx context.Context,
	params UpdateBankConnectionParams,
) error {
	if err := s.access.requireTenantMember(ctx, params.TenantID, params.ActorUserID); err != nil {
		return err
	}
	displayName := strings.TrimSpace(params.Name)
	if displayName == "" {
		return ErrBankConnectionNameRequired
	}
	if err := s.connectionStore.UpdateBankConnectionDisplayName(
		ctx,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.ConnectionID),
		displayName,
		s.now(),
	); err != nil {
		if errors.Is(err, persistence.ErrBankConnectionNotFound) {
			return ErrBankConnectionNotFound
		}
		return fmt.Errorf("update bank connection display name: %w", err)
	}
	return nil
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

func providerResponseErrorForBankConnection(err error) *ProviderResponseError {
	var responseErr *enablebankingclient.ResponseError
	if !errors.As(err, &responseErr) {
		return nil
	}
	return &ProviderResponseError{
		Provider:   bankConnectorEnableBanking,
		Operation:  responseErr.Operation,
		StatusCode: responseErr.StatusCode,
		Code:       responseErr.Code,
		Message:    responseErr.Message,
	}
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
	prepared, err := w.PrepareConnectionSecret(provider, reference, secret)
	if err != nil {
		return "", err
	}
	_, err = w.store.SaveConnectionSecret(ctx, prepared)
	if err != nil {
		return "", fmt.Errorf("save connection secret: %w", err)
	}
	return prepared.ID, nil
}

func (w *bankConnectionSecretWriter) PrepareConnectionSecret(
	provider string,
	reference string,
	secret string,
) (domain.ConnectionSecret, error) {
	envelope, err := w.cipher.SealString(secret)
	if err != nil {
		return domain.ConnectionSecret{}, fmt.Errorf("seal connection secret: %w", err)
	}
	now := w.now()
	return domain.ConnectionSecret{
		ID:        w.newID(),
		Provider:  provider,
		Reference: reference,
		Envelope:  envelope,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
