package monobank

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	monobankclient "github.com/gemyago/sumweave/finance/internal/monobank/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
)

const statementChunkRange = 31*24*time.Hour + time.Hour

const (
	monobankCurrencyUAH = 980
	monobankCurrencyEUR = 978
	monobankCurrencyUSD = 840
	currencyUAH         = "UAH"
	currencyEUR         = "EUR"
	currencyUSD         = "USD"
)

var (
	ErrConnectorStartLinkUnsupported  = errors.New("monobank connector start link unsupported")
	ErrConnectorFinishLinkUnsupported = errors.New("monobank connector finish link unsupported")
	ErrSecretTokenResolverRequired    = errors.New("monobank connector secret token resolver required")
)

type secretTokenResolver func(ctx context.Context, secret domain.ConnectionSecret) (string, error)

type monobankAPI interface {
	GetPersonalClientInfo(
		ctx context.Context,
		params monobankclient.GetPersonalClientInfoParams,
	) (*monobankclient.GetPersonalClientInfoResponse, error)
	GetPersonalStatement(
		ctx context.Context,
		params monobankclient.GetPersonalStatementParams,
	) (*monobankclient.GetPersonalStatementResponse, error)
}

type Args struct {
	BaseURL    string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

type Option func(*Connector)

type Connector struct {
	api                 monobankAPI
	now                 func() time.Time
	secretTokenResolver secretTokenResolver
}

func WithNow(now func() time.Time) Option {
	return func(connector *Connector) {
		if now != nil {
			connector.now = now
		}
	}
}

func WithSecretTokenResolver(resolver func(context.Context, domain.ConnectionSecret) (string, error)) Option {
	return func(connector *Connector) {
		if resolver != nil {
			connector.secretTokenResolver = resolver
		}
	}
}

func WithAPI(api monobankAPI) Option {
	return func(connector *Connector) {
		if api != nil {
			connector.api = api
		}
	}
}

func NewConnector(args Args, opts ...Option) *Connector {
	httpClient := args.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	connector := &Connector{
		api: monobankclient.NewClient(
			monobankclient.Args{BaseURL: args.BaseURL},
			monobankclient.WithHTTPClient(httpClient),
			monobankclient.WithLogger(args.Logger),
		),
		now: time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(connector)
		}
	}
	return connector
}

func (c *Connector) ConnectorID() domain.ProviderConnectorID {
	return domain.ProviderConnectorIDMonobank
}

func (c *Connector) Capabilities() providers.ConnectorCapabilities {
	return providers.ConnectorCapabilities{SupportsTokenLink: true, SupportsFetch: true}
}

func (c *Connector) StartLink(context.Context, providers.StartLinkRequest) (providers.StartLinkResult, error) {
	return providers.StartLinkResult{}, ErrConnectorStartLinkUnsupported
}

func (c *Connector) FinishLink(context.Context, providers.FinishLinkRequest) (providers.LinkResult, error) {
	return providers.LinkResult{}, ErrConnectorFinishLinkUnsupported
}

func (c *Connector) LinkToken(ctx context.Context, request providers.LinkTokenRequest) (providers.LinkResult, error) {
	response, err := c.api.GetPersonalClientInfo(ctx, monobankclient.GetPersonalClientInfoParams{Token: request.Token})
	if err != nil {
		return providers.LinkResult{}, normalizeClientError(err)
	}
	capturedAt := c.now()
	body := response.ClientInfo
	providerObjectID := firstNonEmpty(body.Name, "monobank")
	return providers.LinkResult{
		DisplayName:       providerObjectID,
		ProviderReference: providerObjectID,
		ExternalID:        firstAccountID(body.Accounts),
		Secret:            strings.TrimSpace(request.Token),
		State:             domain.BankConnectionStateActive,
		RawPayloads: []domain.ProviderRawPayloadObservation{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: providerObjectID,
			PayloadJSON:      response.RawJSON,
			CapturedAt:       capturedAt,
		}},
	}, nil
}

func (c *Connector) Fetch(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	token, err := c.resolveToken(ctx, request.Secret)
	if err != nil {
		return domain.ProviderSyncBatch{}, err
	}
	clientInfoResponse, err := c.api.GetPersonalClientInfo(
		ctx,
		monobankclient.GetPersonalClientInfoParams{Token: token},
	)
	if err != nil {
		return domain.ProviderSyncBatch{}, normalizeClientError(err)
	}
	capturedAt := c.now()
	clientInfo := clientInfoResponse.ClientInfo
	batch := domain.ProviderSyncBatch{
		Connection:      request.Connection,
		RequestedWindow: request.RequestedWindow,
		Accounts:        make([]domain.ProviderAccountObservation, 0, len(clientInfo.Accounts)),
		Balances:        make([]domain.ProviderBalanceObservation, 0, len(clientInfo.Accounts)),
		Transactions:    []domain.ProviderTransactionObservation{},
		RawPayloads: []domain.ProviderRawPayloadObservation{{
			Connection:       request.Connection,
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: firstNonEmpty(request.Connection.ExternalID, "client-info"),
			PayloadJSON:      clientInfoResponse.RawJSON,
			CapturedAt:       capturedAt,
		}},
	}
	for _, account := range clientInfo.Accounts {
		batch.Accounts = append(batch.Accounts, normalizeAccount(request.Connection, account))
		batch.Balances = append(batch.Balances, normalizeBalance(request.Connection, account, capturedAt))
		for _, chunk := range makeChunks(account.ID, request.RequestedWindow) {
			statementResponse, statementErr := c.api.GetPersonalStatement(
				ctx,
				monobankclient.GetPersonalStatementParams{
					Token:   token,
					Account: chunk.accountID,
					From:    chunk.fromUnix,
					To:      chunk.toUnix,
				},
			)
			if statementErr != nil {
				return domain.ProviderSyncBatch{}, normalizeClientError(statementErr)
			}
			batch.RawPayloads = append(batch.RawPayloads, domain.ProviderRawPayloadObservation{
				Connection:       request.Connection,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: chunk.accountID,
				PayloadJSON:      statementResponse.RawJSON,
				CapturedAt:       capturedAt,
			})
			for _, item := range statementResponse.Items {
				transaction, normalizeErr := normalizeTransaction(request.Connection, chunk.accountID, item)
				if normalizeErr != nil {
					return domain.ProviderSyncBatch{}, fmt.Errorf(
						"serialize monobank transaction evidence: %w",
						normalizeErr,
					)
				}
				batch.Transactions = append(
					batch.Transactions,
					transaction,
				)
			}
		}
	}
	return batch, nil
}

type statementChunk struct {
	accountID string
	fromUnix  int64
	toUnix    int64
}

func (c *Connector) resolveToken(
	ctx context.Context,
	secret domain.ConnectionSecret,
) (string, error) {
	if c.secretTokenResolver == nil {
		return "", ErrSecretTokenResolverRequired
	}
	token, err := c.secretTokenResolver(ctx, secret)
	if err != nil {
		return "", fmt.Errorf("resolve monobank access token: %w", err)
	}
	return strings.TrimSpace(token), nil
}

func normalizeAccount(
	connection domain.ProviderConnectionRef,
	account monobankclient.InfoAccount,
) domain.ProviderAccountObservation {
	return domain.ProviderAccountObservation{
		Connection:        connection,
		ProviderAccountID: strings.TrimSpace(account.ID),
		Name:              firstNonEmpty(account.Type, account.ID),
		Currency:          currencyCodeToISO(account.CurrencyCode),
		IBAN:              strings.TrimSpace(account.IBAN),
		MaskedPAN:         firstMaskedPAN(account.MaskedPAN),
	}
}

func normalizeBalance(
	connection domain.ProviderConnectionRef,
	account monobankclient.InfoAccount,
	capturedAt time.Time,
) domain.ProviderBalanceObservation {
	balance := domain.ProviderBalanceObservation{
		Connection:          connection,
		ProviderAccountID:   strings.TrimSpace(account.ID),
		Currency:            currencyCodeToISO(account.CurrencyCode),
		CurrentBalanceMinor: account.Balance,
		CapturedAt:          capturedAt,
	}
	if account.CreditLimit != 0 {
		available := account.Balance + account.CreditLimit
		balance.AvailableBalanceMinor = &available
	}
	return balance
}

func normalizeTransaction(
	connection domain.ProviderConnectionRef,
	providerAccountID string,
	item monobankclient.PersonalStatementItem,
) (domain.ProviderTransactionObservation, error) {
	rawPayloadJSON, err := json.Marshal(item)
	if err != nil {
		return domain.ProviderTransactionObservation{}, fmt.Errorf("marshal monobank transaction: %w", err)
	}
	effectiveAt := time.Unix(item.Time, 0)
	description := strings.TrimSpace(item.Description)
	currency := currencyCodeToISO(item.CurrencyCode)
	return domain.ProviderTransactionObservation{
		Connection:            connection,
		ProviderAccountID:     strings.TrimSpace(providerAccountID),
		ProviderTransactionID: strings.TrimSpace(item.ID),
		Status:                statusFromHold(item.Hold),
		AmountMinor:           item.Amount,
		Currency:              currency,
		Description:           description,
		EffectiveAt:           effectiveAt,
		Fingerprint:           providerFingerprint(providerAccountID, description, item.Amount, currency, effectiveAt),
		ProviderOriginal: &domain.ProviderTransactionOriginal{
			AmountMinor: item.Amount,
			Currency:    currency,
			Description: description,
			EffectiveAt: &effectiveAt,
		},
		RawPayloadJSON: rawPayloadJSON,
	}, nil
}

func makeChunks(accountID string, window domain.ProviderSyncWindow) []statementChunk {
	if window.Start.IsZero() || window.End.IsZero() || window.End.Before(window.Start) {
		return []statementChunk{{accountID: firstNonEmpty(accountID, "0")}}
	}
	items := []statementChunk{}
	chunkFrom := window.Start
	windowEnd := window.End
	for {
		chunkTo := chunkFrom.Add(statementChunkRange)
		if chunkTo.After(windowEnd) {
			chunkTo = windowEnd
		}
		items = append(items, statementChunk{
			accountID: firstNonEmpty(accountID, "0"),
			fromUnix:  chunkFrom.Unix(),
			toUnix:    chunkTo.Unix(),
		})
		if !chunkTo.Before(windowEnd) {
			break
		}
		chunkFrom = chunkTo.Add(time.Second)
	}
	return items
}

func statusFromHold(hold bool) domain.TransactionStatus {
	if hold {
		return domain.TransactionStatusPending
	}
	return domain.TransactionStatusBooked
}

func firstAccountID(accounts []monobankclient.InfoAccount) string {
	if len(accounts) == 0 {
		return ""
	}
	return strings.TrimSpace(accounts[0].ID)
}

func firstMaskedPAN(maskedPANs []string) string {
	for _, maskedPAN := range maskedPANs {
		if value := strings.TrimSpace(maskedPAN); value != "" {
			return value
		}
	}
	return ""
}

func normalizeClientError(err error) error {
	var apiErr *monobankclient.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return errors.New("monobank connector rate limit reached")
		}
		return fmt.Errorf("monobank connector request failed with status %d", apiErr.StatusCode)
	}
	return err
}

func currencyCodeToISO(code int) string {
	switch code {
	case monobankCurrencyUAH:
		return currencyUAH
	case monobankCurrencyEUR:
		return currencyEUR
	case monobankCurrencyUSD:
		return currencyUSD
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func providerFingerprint(parts ...any) string {
	var joined strings.Builder
	for _, part := range parts {
		_, _ = fmt.Fprintf(&joined, "%v\n", part)
	}
	hash := sha256.Sum256([]byte(joined.String()))
	return hex.EncodeToString(hash[:16])
}
