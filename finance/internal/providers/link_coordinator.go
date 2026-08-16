package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/google/uuid"
)

const defaultPendingStartTTL = 15 * time.Minute

var (
	ErrProviderProfileRegistryRequired = errors.New("provider profile registry is required")
	ErrPendingStartStoreRequired       = errors.New("pending start store is required")
	ErrConnectionSecretWriterRequired  = errors.New("connection secret writer is required")
	ErrConnectionStoreRequired         = errors.New("connection store is required")
	ErrRedirectLinkUnsupported         = errors.New("redirect link unsupported")
	ErrRedirectCodeRequired            = errors.New("redirect code is required")
	ErrTokenLinkUnsupported            = errors.New("token link unsupported")
	ErrPendingStartNotFound            = errors.New("pending start not found")
)

type ProviderProfileRegistry interface {
	Resolve(providerID domain.ProviderID) (ProviderProfile, error)
}

type ConsumePendingStartRequest struct {
	TenantID    string
	ActorUserID string
	ProviderID  domain.ProviderID
	ConnectorID domain.ProviderConnectorID
	State       string
	ConsumedAt  time.Time
}

type RestorePendingStartRequest struct {
	TenantID    string
	ActorUserID string
	ProviderID  domain.ProviderID
	ConnectorID domain.ProviderConnectorID
	State       string
	RestoredAt  time.Time
}

type PendingStartStore interface {
	SavePendingStart(
		ctx context.Context,
		start domain.PendingBankConnectionLinkStart,
	) (domain.PendingBankConnectionLinkStart, error)
	ConsumePendingStart(
		ctx context.Context,
		request ConsumePendingStartRequest,
	) (*domain.PendingBankConnectionLinkStart, error)
	RestorePendingStart(
		ctx context.Context,
		request RestorePendingStartRequest,
	) error
}

type ConnectionSecretWriter interface {
	PrepareConnectionSecret(provider string, reference string, secret string) (domain.ConnectionSecret, error)
}

type ConnectionStore interface {
	SaveLinkedConnectionWithSnapshot(
		ctx context.Context,
		connection domain.BankConnection,
		secret domain.ConnectionSecret,
		snapshot *domain.ProviderSnapshot,
	) (domain.BankConnection, error)
}

type LinkCoordinatorArgs struct {
	ProviderProfileRegistry ProviderProfileRegistry
	ConnectorRegistry       ConnectorRegistry
	PendingStartStore       PendingStartStore
	ConnectionSecretWriter  ConnectionSecretWriter
	ConnectionStore         ConnectionStore
	Logger                  *slog.Logger
	Now                     func() time.Time
	NewID                   func() string
	PendingStartTTL         time.Duration
}

type LinkCoordinator struct {
	providerProfileRegistry ProviderProfileRegistry
	connectorRegistry       ConnectorRegistry
	pendingStartStore       PendingStartStore
	connectionSecretWriter  ConnectionSecretWriter
	connectionStore         ConnectionStore
	logger                  *slog.Logger
	now                     func() time.Time
	newID                   func() string
	pendingStartTTL         time.Duration
}

type RedirectLinkStartRequest struct {
	TenantID           string
	ActorUserID        string
	ProviderID         domain.ProviderID
	RedirectURL        string
	BrowserCallbackURL string
}

type RedirectLinkFinishRequest struct {
	TenantID    string
	ActorUserID string
	ProviderID  domain.ProviderID
	State       string
	Code        string
}

type TokenLinkRequest struct {
	TenantID    string
	ActorUserID string
	ProviderID  domain.ProviderID
	Token       string
}

func NewLinkCoordinator(args LinkCoordinatorArgs) (*LinkCoordinator, error) {
	if args.ProviderProfileRegistry == nil {
		return nil, ErrProviderProfileRegistryRequired
	}
	if args.ConnectorRegistry == nil {
		return nil, ErrConnectorRegistryRequired
	}
	if args.PendingStartStore == nil {
		return nil, ErrPendingStartStoreRequired
	}
	if args.ConnectionSecretWriter == nil {
		return nil, ErrConnectionSecretWriterRequired
	}
	if args.ConnectionStore == nil {
		return nil, ErrConnectionStoreRequired
	}
	if args.Now == nil {
		args.Now = time.Now
	}
	if args.NewID == nil {
		args.NewID = uuid.NewString
	}
	if args.PendingStartTTL <= 0 {
		args.PendingStartTTL = defaultPendingStartTTL
	}
	if args.Logger == nil {
		args.Logger = slog.New(slog.DiscardHandler)
	}
	return &LinkCoordinator{
		providerProfileRegistry: args.ProviderProfileRegistry,
		connectorRegistry:       args.ConnectorRegistry,
		pendingStartStore:       args.PendingStartStore,
		connectionSecretWriter:  args.ConnectionSecretWriter,
		connectionStore:         args.ConnectionStore,
		logger:                  args.Logger.With("component", "linkCoordinator"),
		now:                     args.Now,
		newID:                   args.NewID,
		pendingStartTTL:         args.PendingStartTTL,
	}, nil
}

//nolint:funlen // Ordered durable-link milestones remain together for traceable recovery.
func (c *LinkCoordinator) StartRedirectLink(
	ctx context.Context,
	request RedirectLinkStartRequest,
) (StartLinkResult, error) {
	profile, connector, err := c.resolveConnector(request.ProviderID)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "resolveConnector"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", ""),
			slog.Any("err", err),
		)
		return StartLinkResult{}, err
	}
	c.logger.InfoContext(
		ctx,
		"bank link connector resolved",
		slog.String("operation", "redirectStart"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
	)
	capabilities := connector.Capabilities()
	if !capabilities.SupportsStartLink {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "capabilityCheck"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", ErrRedirectLinkUnsupported),
		)
		return StartLinkResult{}, ErrRedirectLinkUnsupported
	}

	result, err := connector.StartLink(ctx, StartLinkRequest{
		Profile:            profile,
		RedirectURL:        strings.TrimSpace(request.RedirectURL),
		BrowserCallbackURL: strings.TrimSpace(request.BrowserCallbackURL),
	})
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "connectorStart"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return StartLinkResult{}, fmt.Errorf("start redirect link: %w", err)
	}
	pendingDocument := []byte(nil)
	if len(result.PendingDocument) > 0 {
		sanitizedDocument, sanitizeErr := domain.SanitizeProviderSnapshotJSON(result.PendingDocument)
		if sanitizeErr != nil {
			c.logger.WarnContext(
				ctx,
				"bank link coordinator failed",
				slog.String("operation", "redirectStart"),
				slog.String("failureStage", "sanitizePendingDocument"),
				slog.String("tenantId", request.TenantID),
				slog.String("provider", string(request.ProviderID)),
				slog.String("connectorId", string(profile.ConnectorID)),
				slog.Any("err", sanitizeErr),
			)
			return StartLinkResult{}, fmt.Errorf("sanitize pending start document: %w", sanitizeErr)
		}
		pendingDocument = sanitizedDocument
	}
	now := c.now()
	pendingStart := domain.PendingBankConnectionLinkStart{
		ID:                c.newID(),
		TenantID:          strings.TrimSpace(request.TenantID),
		ActorUserID:       strings.TrimSpace(request.ActorUserID),
		Provider:          string(profile.ProviderID),
		ConnectorID:       profile.ConnectorID,
		State:             strings.TrimSpace(result.State),
		CallbackURL:       strings.TrimSpace(request.BrowserCallbackURL),
		AuthorizationURL:  strings.TrimSpace(result.AuthorizationURL),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		StartResult: domain.PendingBankConnectionLinkStartResult{
			State:            strings.TrimSpace(result.State),
			AuthorizationURL: strings.TrimSpace(result.AuthorizationURL),
			DocumentJSON:     pendingDocument,
		},
		ExpiresAt: now.Add(c.pendingStartTTL),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = c.pendingStartStore.SavePendingStart(ctx, pendingStart)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "savePendingStart"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return StartLinkResult{}, fmt.Errorf("save pending start: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"bank link pending start saved",
		slog.String("operation", "redirectStart"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
		slog.String("pendingStartId", pendingStart.ID),
		slog.Bool("hasPendingDocument", len(result.PendingDocument) > 0),
	)
	return result, nil
}

//nolint:funlen // Ordered durable-link milestones remain together for traceable recovery.
func (c *LinkCoordinator) FinishRedirectLink(
	ctx context.Context,
	request RedirectLinkFinishRequest,
) (domain.BankConnection, error) {
	profile, connector, err := c.resolveConnector(request.ProviderID)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "resolveConnector"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", ""),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, err
	}
	c.logger.InfoContext(
		ctx,
		"bank link connector resolved",
		slog.String("operation", "redirectFinish"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
	)
	capabilities := connector.Capabilities()
	if !capabilities.SupportsFinishLink {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "capabilityCheck"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", ErrRedirectLinkUnsupported),
		)
		return domain.BankConnection{}, ErrRedirectLinkUnsupported
	}
	if capabilities.RequiresRedirectCode && strings.TrimSpace(request.Code) == "" {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "validateCode"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", ErrRedirectCodeRequired),
		)
		return domain.BankConnection{}, ErrRedirectCodeRequired
	}

	now := c.now()
	pendingStart, err := c.pendingStartStore.ConsumePendingStart(ctx, ConsumePendingStartRequest{
		TenantID:    strings.TrimSpace(request.TenantID),
		ActorUserID: strings.TrimSpace(request.ActorUserID),
		ProviderID:  profile.ProviderID,
		ConnectorID: profile.ConnectorID,
		State:       strings.TrimSpace(request.State),
		ConsumedAt:  now,
	})
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "consumePendingStart"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		if errors.Is(err, ErrPendingStartNotFound) {
			return domain.BankConnection{}, ErrPendingStartNotFound
		}
		return domain.BankConnection{}, fmt.Errorf("consume pending start: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"bank link pending start consumed",
		slog.String("operation", "redirectFinish"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
		slog.String("pendingStartId", pendingStart.ID),
	)

	result, err := connector.FinishLink(ctx, FinishLinkRequest{
		Profile: profile,
		State:   strings.TrimSpace(request.State),
		Code:    strings.TrimSpace(request.Code),
		Start: StartLinkResult{
			State:            pendingStart.StartResult.State,
			AuthorizationURL: pendingStart.StartResult.AuthorizationURL,
			PendingDocument:  append([]byte(nil), pendingStart.StartResult.DocumentJSON...),
		},
	})
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "connectorFinish"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, c.restorePendingStartOnError(
			ctx,
			request,
			profile,
			err,
			"finish redirect link",
		)
	}
	connection, err := c.saveLinkedConnection(
		ctx,
		"redirectFinish",
		strings.TrimSpace(request.TenantID),
		profile,
		result,
	)
	if err != nil {
		return domain.BankConnection{}, c.restorePendingStartOnError(
			ctx,
			request,
			profile,
			err,
			"save bank connection",
		)
	}
	return connection, nil
}

func (c *LinkCoordinator) LinkToken(
	ctx context.Context,
	request TokenLinkRequest,
) (domain.BankConnection, error) {
	profile, connector, err := c.resolveConnector(request.ProviderID)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "token"),
			slog.String("failureStage", "resolveConnector"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", ""),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, err
	}
	c.logger.InfoContext(
		ctx,
		"bank link connector resolved",
		slog.String("operation", "token"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
	)
	if !connector.Capabilities().SupportsTokenLink {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "token"),
			slog.String("failureStage", "capabilityCheck"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", ErrTokenLinkUnsupported),
		)
		return domain.BankConnection{}, ErrTokenLinkUnsupported
	}

	result, err := connector.LinkToken(ctx, LinkTokenRequest{
		Profile: profile,
		Token:   strings.TrimSpace(request.Token),
	})
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "token"),
			slog.String("failureStage", "connectorToken"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("link token: %w", err)
	}
	connection, err := c.saveLinkedConnection(
		ctx,
		"token",
		strings.TrimSpace(request.TenantID),
		profile,
		result,
	)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "token"),
			slog.String("failureStage", "saveConnection"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("save bank connection: %w", err)
	}
	return connection, nil
}

//nolint:ireturn // The connector contract is intentionally resolved behind an interface seam.
func (c *LinkCoordinator) resolveConnector(
	providerID domain.ProviderID,
) (ProviderProfile, Connector, error) {
	profile, err := c.providerProfileRegistry.Resolve(providerID)
	if err != nil {
		return ProviderProfile{}, nil, err
	}
	connector, err := c.connectorRegistry.Resolve(profile.ConnectorID)
	if err != nil {
		return ProviderProfile{}, nil, fmt.Errorf("resolve link connector: %w", err)
	}
	return profile, connector, nil
}

func (c *LinkCoordinator) restorePendingStartOnError(
	ctx context.Context,
	request RedirectLinkFinishRequest,
	profile ProviderProfile,
	linkErr error,
	operation string,
) error {
	restoreErr := c.pendingStartStore.RestorePendingStart(ctx, RestorePendingStartRequest{
		TenantID:    strings.TrimSpace(request.TenantID),
		ActorUserID: strings.TrimSpace(request.ActorUserID),
		ProviderID:  profile.ProviderID,
		ConnectorID: profile.ConnectorID,
		State:       strings.TrimSpace(request.State),
		RestoredAt:  c.now(),
	})
	if restoreErr != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "restorePendingStart"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", restoreErr),
		)
		return fmt.Errorf(
			"%s: %w",
			operation,
			errors.Join(
				linkErr,
				fmt.Errorf("restore pending bank connection link start: %w", restoreErr),
			),
		)
	}
	c.logger.InfoContext(
		ctx,
		"bank link pending start restored",
		slog.String("operation", "redirectFinish"),
		slog.String("tenantId", request.TenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
	)
	return fmt.Errorf("%s: %w", operation, linkErr)
}

func (c *LinkCoordinator) saveLinkedConnection(
	ctx context.Context,
	operation string,
	tenantID string,
	profile ProviderProfile,
	result LinkResult,
) (domain.BankConnection, error) {
	now := c.now()
	connection := domain.BankConnection{
		TenantID:          tenantID,
		Provider:          string(profile.ProviderID),
		ConnectorID:       profile.ConnectorID,
		DisplayName:       strings.TrimSpace(result.DisplayName),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		State:             result.State,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	secret, err := c.connectionSecretWriter.PrepareConnectionSecret(
		string(profile.ProviderID),
		connection.ProviderReference,
		strings.TrimSpace(result.Secret),
	)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", operation),
			slog.String("failureStage", "prepareSecret"),
			slog.String("tenantId", tenantID),
			slog.String("provider", string(profile.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("save connection secret: %w", err)
	}
	connection.ID = c.newID()
	connection.SecretID = secret.ID
	var snapshot *domain.ProviderSnapshot
	if result.ConnectionSnapshot != nil {
		document, sanitizeErr := domain.SanitizeProviderSnapshotJSON(result.ConnectionSnapshot.DocumentJSON)
		if sanitizeErr != nil {
			return domain.BankConnection{}, fmt.Errorf("sanitize connection provider snapshot: %w", sanitizeErr)
		}
		snapshot = &domain.ProviderSnapshot{
			ID:               c.newID(),
			TenantID:         tenantID,
			ConnectionID:     connection.ID,
			Subject:          domain.ProviderSnapshotSubjectConnection,
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: result.ConnectionSnapshot.ProviderObjectID,
			DocumentJSON:     document,
			CapturedAt:       result.ConnectionSnapshot.CapturedAt,
		}
		if snapshot.CapturedAt.IsZero() {
			snapshot.CapturedAt = now
		}
	}
	savedConnection, err := c.connectionStore.SaveLinkedConnectionWithSnapshot(
		ctx, connection, secret, snapshot,
	)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", operation),
			slog.String("failureStage", "saveLinkedConnection"),
			slog.String("tenantId", tenantID),
			slog.String("provider", string(profile.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("save linked bank connection: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"bank link connection saved",
		slog.String("operation", "persist"),
		slog.String("tenantId", tenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
		slog.String("connectionId", savedConnection.ID),
	)
	c.logger.InfoContext(
		ctx,
		"bank link connection snapshot saved",
		slog.String("operation", "persist"),
		slog.String("tenantId", tenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
		slog.String("connectionId", savedConnection.ID),
		slog.Bool("hasConnectionSnapshot", snapshot != nil),
	)
	return savedConnection, nil
}
