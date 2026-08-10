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
	ErrRawPayloadWriterRequired        = errors.New("raw payload writer is required")
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
	SaveConnectionSecret(
		ctx context.Context,
		provider string,
		reference string,
		secret string,
	) (string, error)
}

type ConnectionStore interface {
	SaveBankConnection(
		ctx context.Context,
		connection domain.BankConnection,
	) (domain.BankConnection, error)
	ListBankConnections(ctx context.Context, tenantID string) ([]domain.BankConnection, error)
}

type RawPayloadWriter interface {
	SaveRawPayload(ctx context.Context, payload domain.RawPayload) (domain.RawPayload, error)
}

type LinkCoordinatorArgs struct {
	ProviderProfileRegistry ProviderProfileRegistry
	ConnectorRegistry       ConnectorRegistry
	PendingStartStore       PendingStartStore
	ConnectionSecretWriter  ConnectionSecretWriter
	ConnectionStore         ConnectionStore
	RawPayloadWriter        RawPayloadWriter
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
	rawPayloadWriter        RawPayloadWriter
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
	if args.RawPayloadWriter == nil {
		return nil, ErrRawPayloadWriterRequired
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
		rawPayloadWriter:        args.RawPayloadWriter,
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
	result.RawPayloads, err = sanitizeProviderRawPayloads(result.RawPayloads)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectStart"),
			slog.String("failureStage", "sanitizePayload"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return StartLinkResult{}, err
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
			RawPayloads:      result.RawPayloads,
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
		slog.Int("rawPayloadCount", len(result.RawPayloads)),
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
			RawPayloads:      pendingStart.StartResult.RawPayloads,
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
	result.RawPayloads, err = sanitizeProviderRawPayloads(result.RawPayloads)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "redirectFinish"),
			slog.String("failureStage", "sanitizePayload"),
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
			"sanitize provider payload",
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
	result.RawPayloads, err = sanitizeProviderRawPayloads(result.RawPayloads)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", "token"),
			slog.String("failureStage", "sanitizePayload"),
			slog.String("tenantId", request.TenantID),
			slog.String("provider", string(request.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, err
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

func sanitizeProviderRawPayloads(
	payloads []domain.ProviderRawPayloadObservation,
) ([]domain.ProviderRawPayloadObservation, error) {
	sanitized := make([]domain.ProviderRawPayloadObservation, 0, len(payloads))
	for _, payload := range payloads {
		payloadJSON, err := domain.SanitizeProviderEvidenceJSON(payload.PayloadJSON)
		if err != nil {
			return nil, fmt.Errorf("sanitize provider payload: %w", err)
		}
		payload.PayloadJSON = payloadJSON
		sanitized = append(sanitized, payload)
	}
	return sanitized, nil
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

//nolint:funlen // Ordered persistence milestones remain together for traceable recovery.
func (c *LinkCoordinator) saveLinkedConnection(
	ctx context.Context,
	operation string,
	tenantID string,
	profile ProviderProfile,
	result LinkResult,
) (domain.BankConnection, error) {
	secretID, err := c.connectionSecretWriter.SaveConnectionSecret(
		ctx,
		string(profile.ProviderID),
		strings.TrimSpace(result.ProviderReference),
		strings.TrimSpace(result.Secret),
	)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", operation),
			slog.String("failureStage", "saveSecret"),
			slog.String("tenantId", tenantID),
			slog.String("provider", string(profile.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("save connection secret: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"bank link secret saved",
		slog.String("operation", "persist"),
		slog.String("tenantId", tenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
	)

	now := c.now()
	connection := domain.BankConnection{
		ID:                c.newID(),
		TenantID:          tenantID,
		Provider:          string(profile.ProviderID),
		ConnectorID:       profile.ConnectorID,
		DisplayName:       strings.TrimSpace(result.DisplayName),
		ProviderReference: strings.TrimSpace(result.ProviderReference),
		ExternalID:        strings.TrimSpace(result.ExternalID),
		SecretID:          secretID,
		State:             result.State,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if profile.ProviderID == domain.ProviderIDPKO {
		existingConnections, listErr := c.connectionStore.ListBankConnections(ctx, tenantID)
		if listErr != nil {
			c.logger.WarnContext(
				ctx,
				"bank link coordinator failed",
				slog.String("operation", operation),
				slog.String("failureStage", "pkoConnectionLookup"),
				slog.String("tenantId", tenantID),
				slog.String("provider", string(profile.ProviderID)),
				slog.String("connectorId", string(profile.ConnectorID)),
				slog.Any("err", listErr),
			)
			return domain.BankConnection{}, fmt.Errorf("list bank connections: %w", listErr)
		}
		for _, existingConnection := range existingConnections {
			if existingConnection.Provider == string(profile.ProviderID) {
				connection.ID = existingConnection.ID
				connection.CreatedAt = existingConnection.CreatedAt
				break
			}
		}
	}

	savedConnection, err := c.connectionStore.SaveBankConnection(ctx, connection)
	if err != nil {
		c.logger.WarnContext(
			ctx,
			"bank link coordinator failed",
			slog.String("operation", operation),
			slog.String("failureStage", "saveConnection"),
			slog.String("tenantId", tenantID),
			slog.String("provider", string(profile.ProviderID)),
			slog.String("connectorId", string(profile.ConnectorID)),
			slog.Any("err", err),
		)
		return domain.BankConnection{}, fmt.Errorf("save bank connection: %w", err)
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
	for _, payload := range result.RawPayloads {
		capturedAt := payload.CapturedAt
		if capturedAt.IsZero() {
			capturedAt = now
		}
		_, savePayloadErr := c.rawPayloadWriter.SaveRawPayload(ctx, domain.RawPayload{
			ID:               c.newID(),
			ConnectionID:     savedConnection.ID,
			Scope:            payload.Scope,
			ProviderObjectID: payload.ProviderObjectID,
			PayloadJSON:      payload.PayloadJSON,
			CapturedAt:       capturedAt,
		})
		if savePayloadErr != nil {
			c.logger.WarnContext(
				ctx,
				"bank link coordinator failed",
				slog.String("operation", operation),
				slog.String("failureStage", "saveRawPayload"),
				slog.String("tenantId", tenantID),
				slog.String("provider", string(profile.ProviderID)),
				slog.String("connectorId", string(profile.ConnectorID)),
				slog.Any("err", savePayloadErr),
			)
			return domain.BankConnection{}, fmt.Errorf("save raw payload: %w", savePayloadErr)
		}
	}
	c.logger.InfoContext(
		ctx,
		"bank link raw payloads saved",
		slog.String("operation", "persist"),
		slog.String("tenantId", tenantID),
		slog.String("provider", string(profile.ProviderID)),
		slog.String("connectorId", string(profile.ConnectorID)),
		slog.String("connectionId", savedConnection.ID),
		slog.Int("rawPayloadCount", len(result.RawPayloads)),
	)
	return savedConnection, nil
}
