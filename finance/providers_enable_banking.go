package finance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	clientpkg "github.com/gemyago/signal-foundry/finance/internal/enablebanking/client"
)

const (
	EnableBankingDefaultBaseURL   = "https://api.enablebanking.com"
	EnableBankingDefaultASPSPName = "PKO Bank Polski"
	EnableBankingDefaultCountry   = "PL"
	EnableBankingDefaultPSUType   = "personal"
	EnableBankingDefaultValidDays = 90

	enableBankingJWTIssuer   = "enablebanking.com"
	enableBankingJWTAudience = "api.enablebanking.com"
	enableBankingJWTLifetime = 5 * time.Minute
	enableBankingFieldState  = "state"
	enableBankingFieldName   = "name"
	enableBankingSplitParts  = 3
)

type EnableBankingProviderConfig struct {
	BaseURL        string
	HTTPClient     *http.Client
	StateProvider  func() (string, error)
	AppID          string
	PrivateKeyPath string
	ASPSPName      string
	Country        string
	PSUType        string
	ValidDays      int
	Now            func() time.Time
}

type EnableBankingProvider struct {
	baseURL        string
	client         *clientpkg.Client
	httpClient     *http.Client
	stateProvider  func() (string, error)
	appID          string
	privateKeyPath string
	aspspName      string
	country        string
	psuType        string
	validDays      int
	now            func() time.Time
}

func NewEnableBankingProvider(config EnableBankingProviderConfig) *EnableBankingProvider {
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	stateProvider := config.StateProvider
	if stateProvider == nil {
		stateProvider = func() (string, error) { return fmt.Sprintf("state-%d", time.Now().UTC().UnixNano()), nil }
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	validDays := config.ValidDays
	if validDays <= 0 {
		validDays = EnableBankingDefaultValidDays
	}
	return &EnableBankingProvider{
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		client: clientpkg.NewClient(clientpkg.Args{
			BaseURL:        config.BaseURL,
			HTTPClient:     client,
			AppID:          config.AppID,
			PrivateKeyPath: config.PrivateKeyPath,
			Now:            now,
		}),
		httpClient:     client,
		stateProvider:  stateProvider,
		appID:          strings.TrimSpace(config.AppID),
		privateKeyPath: strings.TrimSpace(config.PrivateKeyPath),
		aspspName:      firstNonEmpty(config.ASPSPName, EnableBankingDefaultASPSPName),
		country:        firstNonEmpty(config.Country, EnableBankingDefaultCountry),
		psuType:        firstNonEmpty(config.PSUType, EnableBankingDefaultPSUType),
		validDays:      validDays,
		now:            now,
	}
}

func (p *EnableBankingProvider) Name() string { return bankConnectorEnableBanking }

func (p *EnableBankingProvider) StartLink(
	ctx context.Context,
	params ProviderStartLinkParams,
) (ProviderLinkStart, error) {
	state, err := p.stateProvider()
	if err != nil {
		return ProviderLinkStart{}, fmt.Errorf("enable banking start link: %w", err)
	}
	payload := map[string]any{"redirectUrl": params.RedirectURL, enableBankingFieldState: state}
	if p.usesSignedRequests() {
		payload = p.buildOfficialStartLinkPayload(strings.TrimSpace(params.RedirectURL), strings.TrimSpace(state))
	}
	body, err := p.doJSON(
		ctx,
		http.MethodPost,
		"/auth",
		nil,
		payload,
		"",
	)
	if err != nil {
		return ProviderLinkStart{}, err
	}
	authorizationURL := firstNonEmpty(
		stringValue(body, "authorizationUrl", "authorization_url", "url"),
		stringValue(body, "authorizationURL"),
	)
	if authorizationURL == "" {
		return ProviderLinkStart{}, errors.New("enable banking auth response missing authorization URL")
	}
	return ProviderLinkStart{
		State:            state,
		AuthorizationURL: authorizationURL,
		ProviderReference: firstNonEmpty(
			stringValue(body, "providerReference", "provider_reference"),
			enableBankingSessionIdentifier(body, "authorization_id", "auth_id", "id", "session_id"),
		),
	}, nil
}

func (p *EnableBankingProvider) FinishLink(
	ctx context.Context,
	params ProviderFinishLinkParams,
) (ProviderLinkResult, error) {
	payload := map[string]any{
		enableBankingFieldState: params.State,
		"code":                  params.Code,
		"providerReference":     params.Start.ProviderReference,
	}
	if p.usesSignedRequests() {
		payload = map[string]any{"code": params.Code}
	}
	body, err := p.doJSON(ctx, http.MethodPost, "/sessions", nil, payload, "")
	if err != nil {
		return ProviderLinkResult{}, err
	}
	externalID := firstNonEmpty(
		stringValue(body, "externalId", "external_id"),
		enableBankingSessionIdentifier(body, "id", "session_id"),
	)
	if externalID == "" {
		return ProviderLinkResult{}, errors.New("enable banking session response missing session ID")
	}
	providerReference := firstNonEmpty(
		stringValue(body, "providerReference", "provider_reference"),
		params.Start.ProviderReference,
		externalID,
	)
	providerObjectID := firstNonEmpty(externalID, providerReference, params.Start.ProviderReference, "session")
	return ProviderLinkResult{
		DisplayName: firstNonEmpty(
			stringValue(body, "displayName", "display_name"),
			p.aspspName,
			"Enable Banking",
		),
		ProviderReference: providerReference,
		ExternalID:        externalID,
		Secret:            stringValue(body, "secret"),
		State: domain.BankConnectionState(
			firstNonEmpty(stringValue(body, "state"), string(domain.BankConnectionStateActive)),
		),
		RawPayloads: []ProviderRawPayload{{
			Scope:            domain.RawPayloadScopeConnection,
			ProviderObjectID: providerObjectID,
			PayloadJSON:      mustJSON(body),
		}},
	}, nil
}

func (p *EnableBankingProvider) LinkToken(
	context.Context,
	ProviderTokenLinkParams,
) (ProviderTokenLinkResult, error) {
	return ProviderTokenLinkResult{}, errors.New("enable banking token linking unsupported")
}

func (p *EnableBankingProvider) Sync(
	ctx context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	if p.usesSignedRequests() {
		return p.syncOfficial(ctx, params)
	}
	return p.syncLegacy(ctx, params)
}

//nolint:funlen // Legacy Enable Banking sync keeps the HTTP-to-domain mapping in one place.
func (p *EnableBankingProvider) syncLegacy(
	ctx context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	accountsBody, err := p.doJSON(ctx, http.MethodGet, "/accounts", nil, nil, params.Secret)
	if err != nil {
		return ProviderSyncResult{}, err
	}
	accountItems := objectSlice(accountsBody, "accounts")
	accounts := make([]ProviderNormalizedAccount, 0, len(accountItems))
	transactions := make([]ProviderNormalizedTransaction, 0)
	rawPayloads := []ProviderRawPayload{{
		Scope:            domain.RawPayloadScopeConnection,
		ProviderObjectID: firstNonEmpty(params.ExternalID, "accounts"),
		PayloadJSON:      mustJSON(accountsBody),
	}}
	for _, item := range accountItems {
		providerAccountID := stringValue(item, "id", "uid")
		account := ProviderNormalizedAccount{
			ProviderAccountID: providerAccountID,
			Name:              stringValue(item, "name"),
			Currency:          strings.ToUpper(stringValue(item, "currency")),
			IBAN:              stringValue(item, "iban"),
		}
		balancesBody, balanceErr := p.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+providerAccountID+"/balances",
			nil,
			nil,
			params.Secret,
		)
		if balanceErr != nil {
			return ProviderSyncResult{}, balanceErr
		}
		balanceItems := objectSlice(balancesBody, "balances")
		if len(balanceItems) > 0 {
			current := int64Value(balanceItems[0], "currentBalanceMinor")
			available := int64Value(balanceItems[0], "availableBalanceMinor")
			account.CurrentBalanceMinor = &current
			account.AvailableBalanceMinor = &available
		}
		rawPayloads = append(
			rawPayloads,
			ProviderRawPayload{
				Scope:            domain.RawPayloadScopeAccount,
				ProviderObjectID: providerAccountID,
				PayloadJSON:      mustJSON(balancesBody),
			},
		)
		transactionsBody, transactionsErr := p.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+providerAccountID+"/transactions",
			nil,
			nil,
			params.Secret,
		)
		if transactionsErr != nil {
			return ProviderSyncResult{}, transactionsErr
		}
		for _, txItem := range objectSlice(transactionsBody, "transactions") {
			description := stringValue(txItem, "description")
			txn := ProviderNormalizedTransaction{
				ProviderAccountID:     providerAccountID,
				ProviderTransactionID: stringValue(txItem, "transactionId"),
				Status: domain.TransactionStatus(
					firstNonEmpty(
						stringValue(txItem, "status"),
						string(domain.TransactionStatusBooked),
					),
				),
				AmountMinor: int64Value(txItem, "amountMinor"),
				Currency:    strings.ToUpper(stringValue(txItem, "currency")),
				Description: description,
				EffectiveAt: timeValue(txItem, "effectiveAt"),
				Fingerprint: providerFingerprint(
					providerAccountID,
					description,
					int64Value(txItem, "amountMinor"),
					strings.ToUpper(stringValue(txItem, "currency")),
					timeValue(txItem, "effectiveAt"),
				),
				ProviderOriginal: &domain.ProviderTransactionOriginal{
					AmountMinor: int64Value(txItem, "amountMinor"),
					Currency:    strings.ToUpper(stringValue(txItem, "currency")),
					Description: description,
				},
				RawPayloadJSON: mustJSON(txItem),
			}
			transactions = append(transactions, txn)
		}
		rawPayloads = append(
			rawPayloads,
			ProviderRawPayload{
				Scope:            domain.RawPayloadScopeTransaction,
				ProviderObjectID: providerAccountID,
				PayloadJSON:      mustJSON(transactionsBody),
			},
		)
		accounts = append(accounts, account)
	}
	return ProviderSyncResult{
		SyncKey: providerFingerprint(
			"enable-banking",
			params.ExternalID,
			mustJSON(accountsBody),
			int64(len(transactions)),
			params.WindowStart.UTC(),
			params.WindowEnd.UTC(),
		),
		Accounts:     accounts,
		Transactions: transactions,
		RawPayloads:  rawPayloads,
		Reauth:       enableBankingReauth(accountsBody),
	}, nil
}

func (p *EnableBankingProvider) syncOfficial(
	ctx context.Context,
	params ProviderSyncParams,
) (ProviderSyncResult, error) {
	sessionID := strings.TrimSpace(params.ExternalID)
	if sessionID == "" {
		return ProviderSyncResult{}, errors.New("enable banking session ID is required")
	}
	sessionBody, err := p.doJSON(
		ctx,
		http.MethodGet,
		"/sessions/"+url.PathEscape(sessionID),
		nil,
		nil,
		"",
	)
	if err != nil {
		return ProviderSyncResult{}, err
	}
	accountItems := objectSlice(sessionBody, "accounts")
	accounts := make([]ProviderNormalizedAccount, 0, len(accountItems))
	transactions := make([]ProviderNormalizedTransaction, 0)
	rawPayloads := []ProviderRawPayload{{
		Scope:            domain.RawPayloadScopeConnection,
		ProviderObjectID: sessionID,
		PayloadJSON:      mustJSON(sessionBody),
	}}
	for _, item := range accountItems {
		providerAccountID := firstNonEmpty(stringValue(item, "uid", "id"), stringValue(item, "account_id"))
		if providerAccountID == "" {
			continue
		}
		account := ProviderNormalizedAccount{
			ProviderAccountID: providerAccountID,
			Name:              firstNonEmpty(stringValue(item, "name"), providerAccountID),
			Currency:          strings.ToUpper(stringValue(item, "currency")),
			IBAN:              stringValue(item, "iban"),
		}
		balancesBody, balanceErr := p.doJSON(
			ctx,
			http.MethodGet,
			"/accounts/"+url.PathEscape(providerAccountID)+"/balances",
			nil,
			nil,
			"",
		)
		if balanceErr != nil {
			return ProviderSyncResult{}, balanceErr
		}
		applyEnableBankingBalances(&account, balancesBody)
		rawPayloads = append(
			rawPayloads,
			ProviderRawPayload{
				Scope:            domain.RawPayloadScopeAccount,
				ProviderObjectID: providerAccountID,
				PayloadJSON:      mustJSON(balancesBody),
			},
		)
		pages, items, transactionsErr := p.fetchOfficialTransactionPages(
			ctx,
			providerAccountID,
			params.WindowStart,
			params.WindowEnd,
		)
		if transactionsErr != nil {
			return ProviderSyncResult{}, transactionsErr
		}
		for _, page := range pages {
			rawPayloads = append(
				rawPayloads,
				ProviderRawPayload{
					Scope:            domain.RawPayloadScopeTransaction,
					ProviderObjectID: providerAccountID,
					PayloadJSON:      mustJSON(page),
				},
			)
		}
		for _, item := range items {
			transactions = append(transactions, normalizeOfficialEnableBankingTransaction(providerAccountID, item))
		}
		accounts = append(accounts, account)
	}
	return ProviderSyncResult{
		SyncKey: providerFingerprint(
			"enable-banking",
			sessionID,
			mustJSON(sessionBody),
			int64(len(transactions)),
			params.WindowStart.UTC(),
			params.WindowEnd.UTC(),
		),
		Accounts:     accounts,
		Transactions: transactions,
		RawPayloads:  rawPayloads,
		Reauth:       enableBankingReauth(sessionBody),
	}, nil
}

func enableBankingReauth(body map[string]any) *domain.ConnectionReauthMetadata {
	state := domain.BankConnectionState(
		firstNonEmpty(stringValue(body, "state"), string(domain.BankConnectionStateActive)),
	)
	reason := strings.TrimSpace(stringValue(body, "reauthReason"))
	requiredAt := timePtrUTC(timeValue(body, "reauthRequiredAt"))
	if state != domain.BankConnectionStateReauthRequired && reason == "" && requiredAt == nil {
		return nil
	}
	return &domain.ConnectionReauthMetadata{RequiredAt: requiredAt, Reason: reason}
}

func (p *EnableBankingProvider) doJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	payload any,
	secret string,
) (map[string]any, error) {
	if strings.TrimSpace(secret) != "" {
		ctx = clientpkg.WithBearerToken(ctx, secret)
	}
	raw, err := p.client.DoRawObject(ctx, clientpkg.DoRawJSONParams{
		Method: method,
		Path:   path,
		Query:  query,
		Body:   payload,
	})
	if err != nil {
		var responseErr *clientpkg.ResponseError
		if errors.As(err, &responseErr) {
			return nil, newProviderResponseError(
				"enable-banking",
				responseErr.Operation,
				responseErr.StatusCode,
				responseErr.Body,
			)
		}
		return nil, err
	}
	return raw, nil
}

func (p *EnableBankingProvider) usesSignedRequests() bool {
	return p.appID != "" || p.privateKeyPath != ""
}

func (p *EnableBankingProvider) buildOfficialStartLinkPayload(
	redirectURL string,
	state string,
) map[string]any {
	validUntil := p.now().UTC().Add(time.Duration(p.validDays) * 24 * time.Hour)
	return map[string]any{
		"access": map[string]any{
			"valid_until": validUntil.Format(time.RFC3339),
		},
		"aspsp": map[string]any{
			enableBankingFieldName: p.aspspName,
			"country":              p.country,
		},
		enableBankingFieldState: state,
		"redirect_url":          redirectURL,
		"psu_type":              p.psuType,
	}
}

func (p *EnableBankingProvider) fetchOfficialTransactionPages(
	ctx context.Context,
	accountID string,
	windowStart time.Time,
	windowEnd time.Time,
) ([]map[string]any, []map[string]any, error) {
	pages := make([]map[string]any, 0, 1)
	transactions := make([]map[string]any, 0)
	continuationKey := ""
	for {
		query := makeOfficialEnableBankingTransactionQuery(windowStart, windowEnd, continuationKey)
		page, err := p.doJSON(
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
		continuationKey = stringValue(page, "continuation_key")
		if continuationKey == "" {
			break
		}
	}
	return pages, transactions, nil
}

func makeOfficialEnableBankingTransactionQuery(
	windowStart time.Time,
	windowEnd time.Time,
	continuationKey string,
) url.Values {
	query := url.Values{}
	if !windowStart.IsZero() {
		query.Set("date_from", windowStart.UTC().Format(time.DateOnly))
	}
	if !windowEnd.IsZero() {
		query.Set("date_to", windowEnd.UTC().Format(time.DateOnly))
	}
	if continuationKey != "" {
		query.Set("continuation_key", continuationKey)
	}
	return query
}

func normalizeOfficialEnableBankingTransaction(
	providerAccountID string,
	raw map[string]any,
) ProviderNormalizedTransaction {
	effectiveAt := enableBankingTransactionTime(raw)
	description := firstNonEmpty(
		stringValue(raw, "description"),
		stringValue(raw, "remittance_information_unstructured", "remittanceInformationUnstructured"),
	)
	currency := strings.ToUpper(firstNonEmpty(
		stringValue(raw, "currency"),
		stringValue(enableBankingAmountObject(raw), "currency"),
	))
	amountMinor := enableBankingAmountMinor(raw)
	providerOriginal := &domain.ProviderTransactionOriginal{
		AmountMinor: amountMinor,
		Currency:    currency,
		Description: description,
	}
	if !effectiveAt.IsZero() {
		providerOriginal.EffectiveAt = &effectiveAt
	}
	return ProviderNormalizedTransaction{
		ProviderAccountID: providerAccountID,
		ProviderTransactionID: firstNonEmpty(
			stringValue(raw, "transactionId", "transaction_id", "id"),
			providerFingerprint(providerAccountID, mustJSON(raw)),
		),
		Status: domain.TransactionStatus(firstNonEmpty(
			strings.ToLower(stringValue(raw, "status")),
			string(domain.TransactionStatusBooked),
		)),
		AmountMinor:      amountMinor,
		Currency:         currency,
		Description:      description,
		EffectiveAt:      effectiveAt,
		Fingerprint:      providerFingerprint(providerAccountID, description, amountMinor, currency, effectiveAt),
		ProviderOriginal: providerOriginal,
		RawPayloadJSON:   mustJSON(raw),
	}
}

func applyEnableBankingBalances(account *ProviderNormalizedAccount, raw map[string]any) {
	if account == nil {
		return
	}
	items := objectSlice(raw, "balances")
	if len(items) == 0 {
		return
	}
	var current *int64
	var available *int64
	for _, item := range items {
		amountObject := enableBankingAmountObject(item)
		amountMinor := enableBankingDecimalToMinor(stringValue(amountObject, "amount"))
		if amountMinor == 0 {
			amountMinor = int64Value(item, "currentBalanceMinor")
		}
		balanceType := strings.ToLower(strings.TrimSpace(stringValue(item, "type")))
		switch balanceType {
		case "interimavailable", "available", "availablebalance", "expectedavailable":
			value := amountMinor
			available = &value
		case "closingbooked", "booked", "current", "currentbalance":
			value := amountMinor
			current = &value
		}
		if account.Currency == "" {
			account.Currency = strings.ToUpper(stringValue(amountObject, "currency"))
		}
	}
	if current == nil {
		fallback := enableBankingDecimalToMinor(stringValue(enableBankingAmountObject(items[0]), "amount"))
		current = &fallback
	}
	if available == nil && current != nil {
		fallback := *current
		available = &fallback
	}
	account.CurrentBalanceMinor = current
	account.AvailableBalanceMinor = available
}

func enableBankingSessionIdentifier(raw map[string]any, keys ...string) string {
	identifier := stringValue(raw, keys...)
	if identifier != "" {
		return identifier
	}
	parent, ok := raw["session"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(parent, keys...)
}

func enableBankingAmountMinor(raw map[string]any) int64 {
	if value := int64Value(raw, "amountMinor"); value != 0 {
		return value
	}
	amountMinor := enableBankingDecimalToMinor(stringValue(enableBankingAmountObject(raw), "amount"))
	indicator := strings.ToUpper(stringValue(raw, "credit_debit_indicator", "creditDebitIndicator"))
	if amountMinor > 0 && indicator == "DBIT" {
		return -amountMinor
	}
	return amountMinor
}

func enableBankingAmountObject(raw map[string]any) map[string]any {
	amountObject, _ := raw["amount"].(map[string]any)
	if amountObject != nil {
		return amountObject
	}
	balanceAmount, _ := raw["balance_amount"].(map[string]any)
	if balanceAmount != nil {
		return balanceAmount
	}
	return map[string]any{}
}

func enableBankingTransactionTime(raw map[string]any) time.Time {
	for _, value := range []string{
		stringValue(raw, "effectiveAt"),
		stringValue(raw, "booking_date", "bookingDate"),
		stringValue(raw, "value_date", "valueDate"),
	} {
		parsed := parseEnableBankingTime(value)
		if !parsed.IsZero() {
			return parsed
		}
	}
	return time.Time{}
}

func parseEnableBankingTime(raw string) time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func enableBankingDecimalToMinor(raw string) int64 {
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
	parts := strings.SplitN(trimmed, ".", enableBankingSplitParts)
	whole, _ := strconv.ParseInt(firstNonEmpty(parts[0], "0"), 10, 64)
	frac := "00"
	if len(parts) > 1 {
		frac = parts[1] + "00"
	}
	fracValue, _ := strconv.ParseInt(frac[:2], 10, 64)
	return sign * (whole*100 + fracValue)
}
