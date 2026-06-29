package enablebanking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	enablebankingclient "github.com/gemyago/signal-foundry/finance/internal/enablebanking/client"
	"github.com/gemyago/signal-foundry/finance/internal/providers"
)

const (
	defaultASPSPName = "PKO Bank Polski"
	defaultCountry   = "PL"
	defaultPSUType   = "personal"
	defaultValidDays = 90
	fieldState       = "state"
	fieldCode        = "code"
	fieldName        = "name"
	fieldSecret      = "secret"
	fieldToken       = "token"
	balanceAvailable = "available"
	decimalSplitPart = 3
	decimalBase      = 10
)

var (
	ErrConnectorTokenLinkUnsupported   = errors.New("enable banking connector token link unsupported")
	ErrConnectorUnsupportedAuthBranch  = errors.New("enable banking connector unsupported auth branch")
	ErrConnectorUnsupportedFetchBranch = errors.New("enable banking connector unsupported fetch branch")
	ErrSecretResolverRequired          = errors.New("enable banking connector secret resolver required")

	privateKeyPattern = regexp.MustCompile(
		`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`,
	)
	bearerPattern    = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-+/=]+`)
	jwtPattern       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9._-]+\.[A-Za-z0-9._-]+`)
	jsonTokenPattern = regexp.MustCompile(`(?i)("(?:token|secret)"\s*:\s*")([^"]*)(")`)
	tokenPattern     = regexp.MustCompile(`(?i)\b(token|secret)\b([=:]\s*|\s+)([^\s,;]+)`)
)

type secretResolver func(ctx context.Context, secret domain.ConnectionSecret) (string, error)

type apiClient interface {
	DoRawObject(ctx context.Context, params enablebankingclient.DoRawJSONParams) (map[string]any, error)
}

type Args struct {
	BaseURL        string
	HTTPClient     *http.Client
	Logger         *slog.Logger
	StateProvider  func() (string, error)
	AppID          string
	PrivateKeyPath string
	ASPSPName      string
	Country        string
	PSUType        string
	ValidDays      int
	Now            func() time.Time
}

type Option func(*Connector)

type Connector struct {
	api            apiClient
	stateProvider  func() (string, error)
	appID          string
	privateKeyPath string
	aspspName      string
	country        string
	psuType        string
	validDays      int
	now            func() time.Time
	secretResolver secretResolver
}

type authBranch int

const (
	authBranchUnsupported authBranch = iota
	authBranchLegacy
	authBranchOfficial
)

func WithNow(now func() time.Time) Option {
	return func(connector *Connector) {
		if now != nil {
			connector.now = now
		}
	}
}

func WithAPI(api apiClient) Option {
	return func(connector *Connector) {
		if api != nil {
			connector.api = api
		}
	}
}

func WithSecretResolver(resolver func(context.Context, domain.ConnectionSecret) (string, error)) Option {
	return func(connector *Connector) {
		if resolver != nil {
			connector.secretResolver = resolver
		}
	}
}

func NewConnector(args Args, opts ...Option) *Connector {
	httpClient := args.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	stateProvider := args.StateProvider
	if stateProvider == nil {
		stateProvider = func() (string, error) {
			return fmt.Sprintf("state-%d", time.Now().UTC().UnixNano()), nil
		}
	}
	now := args.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	validDays := args.ValidDays
	if validDays <= 0 {
		validDays = defaultValidDays
	}
	connector := &Connector{
		api: enablebankingclient.NewClient(enablebankingclient.Args{
			BaseURL:        args.BaseURL,
			HTTPClient:     httpClient,
			Logger:         args.Logger,
			AppID:          args.AppID,
			PrivateKeyPath: args.PrivateKeyPath,
			Now:            now,
		}),
		stateProvider:  stateProvider,
		appID:          strings.TrimSpace(args.AppID),
		privateKeyPath: strings.TrimSpace(args.PrivateKeyPath),
		aspspName:      firstNonEmpty(args.ASPSPName, defaultASPSPName),
		country:        firstNonEmpty(args.Country, defaultCountry),
		psuType:        firstNonEmpty(args.PSUType, defaultPSUType),
		validDays:      validDays,
		now:            now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(connector)
		}
	}
	return connector
}

func (c *Connector) ConnectorID() domain.ProviderConnectorID {
	return domain.ProviderConnectorIDEnableBanking
}

func (c *Connector) Capabilities() providers.ConnectorCapabilities {
	return providers.ConnectorCapabilities{
		SupportsStartLink:  true,
		SupportsFinishLink: true,
		SupportsFetch:      true,
	}
}

func (c *Connector) StartLink(
	ctx context.Context,
	request providers.StartLinkRequest,
) (providers.StartLinkResult, error) {
	branch := c.selectedAuthBranch()
	if branch == authBranchUnsupported {
		return providers.StartLinkResult{}, ErrConnectorUnsupportedAuthBranch
	}
	state, err := c.stateProvider()
	if err != nil {
		return providers.StartLinkResult{}, fmt.Errorf("enable banking start link: %w", err)
	}
	payload := map[string]any{
		"redirectUrl": strings.TrimSpace(request.RedirectURL),
		fieldState:    strings.TrimSpace(state),
	}
	if branch == authBranchOfficial {
		payload = c.buildOfficialStartLinkPayload(strings.TrimSpace(request.RedirectURL), strings.TrimSpace(state))
	}
	raw, err := c.doJSON(ctx, http.MethodPost, "/auth", nil, payload, "")
	if err != nil {
		return providers.StartLinkResult{}, err
	}
	authorizationURL := firstNonEmpty(
		stringValue(raw, "authorizationUrl", "authorization_url", "url"),
		stringValue(raw, "authorizationURL"),
	)
	if authorizationURL == "" {
		return providers.StartLinkResult{}, errors.New("enable banking auth response missing authorization URL")
	}
	providerObjectID := firstNonEmpty(
		stringValue(raw, "providerReference", "provider_reference"),
		extractSessionIdentifier(raw, "authorization_id", "auth_id", "id", "session_id"),
		"auth",
	)
	return providers.StartLinkResult{
		State:            strings.TrimSpace(state),
		AuthorizationURL: authorizationURL,
		RawPayloads: []domain.ProviderRawPayloadObservation{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: providerObjectID,
			PayloadJSON:      mustJSON(redactRawPayload(raw)),
			CapturedAt:       c.now().UTC(),
		}},
	}, nil
}

func (c *Connector) FinishLink(
	ctx context.Context,
	request providers.FinishLinkRequest,
) (providers.LinkResult, error) {
	branch := c.selectedAuthBranch()
	if branch == authBranchUnsupported {
		return providers.LinkResult{}, ErrConnectorUnsupportedAuthBranch
	}
	payload := map[string]any{fieldCode: strings.TrimSpace(request.Code)}
	if branch == authBranchLegacy {
		providerReference := providerReferenceFromStart(request.Start)
		if providerReference == "" ||
			strings.TrimSpace(request.State) == "" ||
			strings.TrimSpace(request.Code) == "" {
			return providers.LinkResult{}, ErrConnectorUnsupportedAuthBranch
		}
		payload = map[string]any{
			fieldState:          strings.TrimSpace(request.State),
			fieldCode:           strings.TrimSpace(request.Code),
			"providerReference": providerReference,
		}
	}
	raw, err := c.doJSON(ctx, http.MethodPost, "/sessions", nil, payload, "")
	if err != nil {
		return providers.LinkResult{}, err
	}
	externalID := firstNonEmpty(
		stringValue(raw, "externalId", "external_id"),
		extractSessionIdentifier(raw, "id", "session_id"),
	)
	if externalID == "" {
		return providers.LinkResult{}, errors.New("enable banking session response missing session ID")
	}
	providerReference := firstNonEmpty(
		stringValue(raw, "providerReference", "provider_reference"),
		externalID,
	)
	if branch == authBranchLegacy {
		providerReference = firstNonEmpty(
			stringValue(raw, "providerReference", "provider_reference"),
			providerReferenceFromStart(request.Start),
			externalID,
		)
	}
	providerObjectID := firstNonEmpty(externalID, providerReference, "session")
	return providers.LinkResult{
		DisplayName: firstNonEmpty(
			stringValue(raw, "displayName", "display_name"),
			c.aspspName,
			"Enable Banking",
		),
		ProviderReference: providerReference,
		ExternalID:        externalID,
		Secret:            stringValue(raw, fieldSecret),
		State: domain.BankConnectionState(firstNonEmpty(
			stringValue(raw, "state"),
			string(domain.BankConnectionStateActive),
		)),
		RawPayloads: []domain.ProviderRawPayloadObservation{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: providerObjectID,
			PayloadJSON:      mustJSON(redactRawPayload(raw)),
			CapturedAt:       c.now().UTC(),
		}},
	}, nil
}

func (c *Connector) LinkToken(context.Context, providers.LinkTokenRequest) (providers.LinkResult, error) {
	return providers.LinkResult{}, ErrConnectorTokenLinkUnsupported
}

func (c *Connector) Fetch(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	branch := c.selectedAuthBranch()
	switch branch {
	case authBranchLegacy:
		return c.fetchLegacy(ctx, request)
	case authBranchOfficial:
		return c.fetchOfficial(ctx, request)
	case authBranchUnsupported:
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	default:
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	}
}

func (c *Connector) fetchLegacy(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	secret, err := c.resolveSecret(ctx, request.Secret)
	if err != nil {
		return domain.ProviderSyncBatch{}, err
	}
	if secret == "" {
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	}
	accountsRaw, err := c.doJSON(ctx, http.MethodGet, "/accounts", nil, nil, secret)
	if err != nil {
		return domain.ProviderSyncBatch{}, err
	}
	return c.mapBatch(ctx, request, secret, accountsRaw, false)
}

func (c *Connector) fetchOfficial(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	if strings.TrimSpace(request.Connection.ExternalID) == "" ||
		request.Secret.ID != "" ||
		request.Secret.Reference != "" {
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	}
	sessionRaw, err := c.doJSON(
		ctx,
		http.MethodGet,
		"/sessions/"+url.PathEscape(strings.TrimSpace(request.Connection.ExternalID)),
		nil,
		nil,
		"",
	)
	if err != nil {
		return domain.ProviderSyncBatch{}, err
	}
	return c.mapBatch(ctx, request, "", sessionRaw, true)
}

func (c *Connector) mapBatch(
	ctx context.Context,
	request providers.FetchRequest,
	secret string,
	connectionRaw map[string]any,
	official bool,
) (domain.ProviderSyncBatch, error) {
	capturedAt := c.now().UTC()
	accountItems := objectSlice(connectionRaw, "accounts")
	batch := domain.ProviderSyncBatch{
		Connection:      request.Connection,
		RequestedWindow: request.RequestedWindow,
		Accounts:        make([]domain.ProviderAccountObservation, 0, len(accountItems)),
		Balances:        make([]domain.ProviderBalanceObservation, 0, len(accountItems)),
		Transactions:    []domain.ProviderTransactionObservation{},
		RawPayloads: []domain.ProviderRawPayloadObservation{{
			Connection:       request.Connection,
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: connectionPayloadProviderObjectID(request.Connection, official),
			PayloadJSON:      mustJSON(redactRawPayload(connectionRaw)),
			CapturedAt:       capturedAt,
		}},
	}
	for _, accountRaw := range accountItems {
		accountID := firstNonEmpty(stringValue(accountRaw, "uid", "id"), stringValue(accountRaw, "account_id"))
		if accountID == "" {
			continue
		}
		account := normalizeAccount(request.Connection, accountID, accountRaw)
		batch.Accounts = append(batch.Accounts, account)

		balancesRaw, err := c.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+url.PathEscape(accountID)+"/balances",
			nil,
			nil,
			secret,
		)
		if err != nil {
			return domain.ProviderSyncBatch{}, err
		}
		balance := normalizeBalance(request.Connection, account, balancesRaw, capturedAt)
		batch.Balances = append(batch.Balances, balance)
		batch.RawPayloads = append(batch.RawPayloads, domain.ProviderRawPayloadObservation{
			Connection:       request.Connection,
			Scope:            domain.RawPayloadScopeAccount,
			ProviderObjectID: accountID,
			PayloadJSON:      mustJSON(redactRawPayload(balancesRaw)),
			CapturedAt:       capturedAt,
		})

		transactionPages, transactionsRaw, err := c.fetchTransactionPages(ctx, request, accountID, secret, official)
		if err != nil {
			return domain.ProviderSyncBatch{}, err
		}
		for _, page := range transactionPages {
			batch.RawPayloads = append(batch.RawPayloads, domain.ProviderRawPayloadObservation{
				Connection:       request.Connection,
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: accountID,
				PayloadJSON:      mustJSON(redactRawPayload(page)),
				CapturedAt:       capturedAt,
			})
		}
		for _, transactionRaw := range transactionsRaw {
			batch.Transactions = append(
				batch.Transactions,
				normalizeTransaction(request.Connection, accountID, transactionRaw),
			)
		}
	}
	return batch, nil
}

func (c *Connector) fetchTransactionPages(
	ctx context.Context,
	request providers.FetchRequest,
	accountID string,
	secret string,
	official bool,
) ([]map[string]any, []map[string]any, error) {
	if !official {
		page, err := c.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+url.PathEscape(accountID)+"/transactions",
			nil,
			nil,
			secret,
		)
		if err != nil {
			return nil, nil, err
		}
		return []map[string]any{page}, objectSlice(page, "transactions"), nil
	}
	pages := make([]map[string]any, 0, 1)
	transactions := make([]map[string]any, 0)
	continuationKey := ""
	for {
		query := url.Values{}
		if !request.RequestedWindow.Start.IsZero() {
			query.Set("date_from", request.RequestedWindow.Start.UTC().Format(time.DateOnly))
		}
		if !request.RequestedWindow.End.IsZero() {
			query.Set("date_to", request.RequestedWindow.End.UTC().Format(time.DateOnly))
		}
		if continuationKey != "" {
			query.Set("continuation_key", continuationKey)
		}
		page, err := c.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+url.PathEscape(accountID)+"/transactions",
			query,
			nil,
			"",
		)
		if err != nil {
			return nil, nil, err
		}
		pages = append(pages, page)
		transactions = append(transactions, objectSlice(page, "transactions")...)
		continuationKey = stringValue(page, "continuation_key", "continuationKey")
		if continuationKey == "" {
			break
		}
	}
	return pages, transactions, nil
}

func (c *Connector) doJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	payload any,
	secret string,
) (map[string]any, error) {
	if strings.TrimSpace(secret) != "" {
		ctx = enablebankingclient.WithBearerToken(ctx, secret)
	}
	raw, err := c.api.DoRawObject(ctx, enablebankingclient.DoRawJSONParams{
		Method: method,
		Path:   path,
		Query:  query,
		Body:   payload,
	})
	if err != nil {
		return nil, sanitizeClientError(err)
	}
	return raw, nil
}

func (c *Connector) selectedAuthBranch() authBranch {
	hasAppID := c.appID != ""
	hasPrivateKey := c.privateKeyPath != ""
	switch {
	case !hasAppID && !hasPrivateKey:
		return authBranchLegacy
	case hasAppID && hasPrivateKey:
		return authBranchOfficial
	default:
		return authBranchUnsupported
	}
}

func (c *Connector) buildOfficialStartLinkPayload(redirectURL string, state string) map[string]any {
	validUntil := c.now().UTC().Add(time.Duration(c.validDays) * 24 * time.Hour)
	return map[string]any{
		"access": map[string]any{"valid_until": validUntil.Format(time.RFC3339)},
		"aspsp": map[string]any{
			fieldName: c.aspspName,
			"country": c.country,
		},
		fieldState:     state,
		"redirect_url": redirectURL,
		"psu_type":     c.psuType,
	}
}

func (c *Connector) resolveSecret(ctx context.Context, secret domain.ConnectionSecret) (string, error) {
	if c.secretResolver == nil {
		if secret.ID == "" && secret.Reference == "" {
			return "", ErrConnectorUnsupportedFetchBranch
		}
		return "", ErrSecretResolverRequired
	}
	resolved, err := c.secretResolver(ctx, secret)
	if err != nil {
		return "", fmt.Errorf("resolve enable banking access secret: %w", err)
	}
	return strings.TrimSpace(resolved), nil
}

func normalizeAccount(
	connection domain.ProviderConnectionRef,
	accountID string,
	raw map[string]any,
) domain.ProviderAccountObservation {
	return domain.ProviderAccountObservation{
		Connection:        connection,
		ProviderAccountID: strings.TrimSpace(accountID),
		Name:              firstNonEmpty(stringValue(raw, "name"), accountID),
		Currency:          strings.ToUpper(stringValue(raw, "currency")),
		IBAN:              stringValue(raw, "iban"),
	}
}

func normalizeBalance(
	connection domain.ProviderConnectionRef,
	account domain.ProviderAccountObservation,
	raw map[string]any,
	capturedAt time.Time,
) domain.ProviderBalanceObservation {
	current, available, currency := selectBalanceAmounts(raw)
	if currency == "" {
		currency = account.Currency
	}
	balance := domain.ProviderBalanceObservation{
		Connection:          connection,
		ProviderAccountID:   account.ProviderAccountID,
		Currency:            currency,
		CurrentBalanceMinor: current,
		CapturedAt:          capturedAt,
	}
	if available != nil {
		balance.AvailableBalanceMinor = available
	}
	return balance
}

func normalizeTransaction(
	connection domain.ProviderConnectionRef,
	accountID string,
	raw map[string]any,
) domain.ProviderTransactionObservation {
	effectiveAt := transactionTime(raw)
	description := firstNonEmpty(
		stringValue(raw, "description"),
		stringValue(raw, "remittance_information_unstructured", "remittanceInformationUnstructured"),
	)
	currency := strings.ToUpper(firstNonEmpty(
		stringValue(raw, "currency"),
		stringValue(amountObject(raw), "currency"),
	))
	amountMinor := amountMinor(raw)
	transactionID := firstNonEmpty(
		stringValue(raw, "transactionId", "transaction_id", "id"),
		providerFingerprint(accountID, mustJSON(raw)),
	)
	status := domain.TransactionStatus(firstNonEmpty(
		strings.ToLower(stringValue(raw, "status")),
		string(domain.TransactionStatusBooked),
	))
	providerOriginal := &domain.ProviderTransactionOriginal{
		AmountMinor: amountMinor,
		Currency:    currency,
		Description: description,
	}
	if !effectiveAt.IsZero() {
		providerOriginal.EffectiveAt = &effectiveAt
	}
	return domain.ProviderTransactionObservation{
		Connection:            connection,
		ProviderAccountID:     strings.TrimSpace(accountID),
		ProviderTransactionID: transactionID,
		Status:                status,
		AmountMinor:           amountMinor,
		Currency:              currency,
		Description:           description,
		EffectiveAt:           effectiveAt,
		Fingerprint:           providerFingerprint(accountID, description, amountMinor, currency, effectiveAt),
		ProviderOriginal:      providerOriginal,
	}
}

func connectionPayloadProviderObjectID(connection domain.ProviderConnectionRef, official bool) string {
	if official {
		return firstNonEmpty(connection.ExternalID, "session")
	}
	return firstNonEmpty(connection.ExternalID, "accounts")
}

func providerReferenceFromStart(start providers.StartLinkResult) string {
	for _, rawPayload := range start.RawPayloads {
		if rawPayload.Scope != domain.RawPayloadScopeConnection {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(rawPayload.PayloadJSON, &raw); err != nil {
			continue
		}
		providerReference := firstNonEmpty(
			stringValue(raw, "providerReference", "provider_reference"),
			extractSessionIdentifier(raw, "authorization_id", "auth_id", "id", "session_id"),
		)
		if providerReference != "" {
			return providerReference
		}
	}
	return ""
}

func sanitizeClientError(err error) error {
	if err == nil {
		return nil
	}
	var responseErr *enablebankingclient.ResponseError
	if errors.As(err, &responseErr) {
		return fmt.Errorf(
			"enable banking %s failed with status %d: %s",
			strings.TrimSpace(responseErr.Operation),
			responseErr.StatusCode,
			sanitizeSecretText(responseErr.Message),
		)
	}
	return fmt.Errorf("%s", sanitizeSecretText(err.Error()))
}

func redactRawPayload(raw map[string]any) map[string]any {
	redacted := make(map[string]any, len(raw))
	for key, value := range raw {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if lowerKey == fieldSecret || lowerKey == fieldToken {
			continue
		}
		switch typedValue := value.(type) {
		case map[string]any:
			redacted[key] = redactRawPayload(typedValue)
		case []any:
			items := make([]any, 0, len(typedValue))
			for _, item := range typedValue {
				if objectItem, ok := item.(map[string]any); ok {
					items = append(items, redactRawPayload(objectItem))
					continue
				}
				items = append(items, item)
			}
			redacted[key] = items
		default:
			redacted[key] = value
		}
	}
	return redacted
}

func selectBalanceAmounts(raw map[string]any) (int64, *int64, string) {
	items := objectSlice(raw, "balances")
	var current *int64
	var available *int64
	currency := ""
	for _, item := range items {
		amount := amountObject(item)
		amountMinor := firstNonZeroInt64(
			int64Value(item, "currentBalanceMinor"),
			decimalToMinor(stringValue(amount, "amount")),
		)
		balanceType := strings.ToLower(strings.TrimSpace(stringValue(item, "type")))
		switch balanceType {
		case "interimavailable", balanceAvailable, "availablebalance", "expectedavailable":
			value := amountMinor
			available = &value
		case "closingbooked", "booked", "current", "currentbalance":
			value := amountMinor
			current = &value
		}
		if currency == "" {
			currency = strings.ToUpper(stringValue(amount, "currency"))
		}
	}
	if current == nil {
		if len(items) > 0 {
			fallback := firstNonZeroInt64(
				int64Value(items[0], "currentBalanceMinor", "availableBalanceMinor"),
				decimalToMinor(stringValue(amountObject(items[0]), "amount")),
			)
			current = &fallback
		}
	}
	if available == nil && current != nil {
		fallback := *current
		available = &fallback
	}
	if current == nil {
		return 0, available, currency
	}
	return *current, available, currency
}

func transactionTime(raw map[string]any) time.Time {
	for _, value := range []string{
		stringValue(raw, "effectiveAt"),
		stringValue(raw, "booking_date", "bookingDate"),
		stringValue(raw, "value_date", "valueDate"),
	} {
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, time.DateOnly} {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				return parsed.UTC()
			}
		}
	}
	return time.Time{}
}

func amountMinor(raw map[string]any) int64 {
	if value := int64Value(raw, "amountMinor"); value != 0 {
		return value
	}
	amount := decimalToMinor(stringValue(amountObject(raw), "amount"))
	if amount > 0 && strings.EqualFold(stringValue(raw, "credit_debit_indicator", "creditDebitIndicator"), "DBIT") {
		return -amount
	}
	return amount
}

func amountObject(raw map[string]any) map[string]any {
	if value, ok := raw["amount"].(map[string]any); ok && value != nil {
		return value
	}
	if value, ok := raw["balance_amount"].(map[string]any); ok && value != nil {
		return value
	}
	if value, ok := raw["balanceAmount"].(map[string]any); ok && value != nil {
		return value
	}
	return map[string]any{}
}

func extractSessionIdentifier(raw map[string]any, keys ...string) string {
	identifier := stringValue(raw, keys...)
	if identifier != "" {
		return identifier
	}
	parent, _ := raw["session"].(map[string]any)
	return stringValue(parent, keys...)
}

func objectSlice(raw map[string]any, key string) []map[string]any {
	items, _ := raw[key].([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		objectItem, ok := item.(map[string]any)
		if ok {
			result = append(result, objectItem)
		}
	}
	return result
}

func stringValue(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		stringValue, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(stringValue)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func int64Value(raw map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case int:
			return int64(value)
		case int32:
			return int64(value)
		case int64:
			return value
		case float64:
			return int64(value)
		}
	}
	return 0
}

func decimalToMinor(raw string) int64 {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	if trimmed == "" {
		return 0
	}
	sign := int64(1)
	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimPrefix(trimmed, "-")
	}
	trimmed = strings.TrimPrefix(trimmed, "+")
	parts := strings.SplitN(trimmed, ".", decimalSplitPart)
	whole := parts[0]
	frac := "00"
	if len(parts) > 1 {
		frac = parts[1] + "00"
	}
	var wholeValue int64
	for _, r := range whole {
		if r < '0' || r > '9' {
			return 0
		}
		wholeValue = wholeValue*decimalBase + int64(r-'0')
	}
	fracValue := int64(0)
	for _, r := range frac[:2] {
		if r < '0' || r > '9' {
			return 0
		}
		fracValue = fracValue*decimalBase + int64(r-'0')
	}
	return sign * (wholeValue*100 + fracValue)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return encoded
}

func sanitizeSecretText(value string) string {
	sanitized := privateKeyPattern.ReplaceAllString(strings.TrimSpace(value), "[REDACTED_PRIVATE_KEY]")
	sanitized = bearerPattern.ReplaceAllString(sanitized, "Bearer [REDACTED]")
	sanitized = jwtPattern.ReplaceAllString(sanitized, "[REDACTED_JWT]")
	sanitized = jsonTokenPattern.ReplaceAllString(sanitized, `$1[REDACTED]$3`)
	sanitized = tokenPattern.ReplaceAllString(sanitized, "$1$2[REDACTED]")
	return sanitized
}

func providerFingerprint(parts ...any) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = io.WriteString(hasher, fmt.Sprint(part, "|"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
