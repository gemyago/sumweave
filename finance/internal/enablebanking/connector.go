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
	"strings"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	enablebankingclient "github.com/gemyago/sumweave/finance/internal/enablebanking/client"
	"github.com/gemyago/sumweave/finance/internal/providers"
)

const (
	defaultValidDays = 90
	balanceAvailable = "available"
	decimalSplitPart = 3
	decimalBase      = 10
)

var (
	ErrConnectorTokenLinkUnsupported = errors.New(
		"enable banking connector token link unsupported",
	)
	ErrConnectorUnsupportedAuthBranch = errors.New(
		"enable banking connector unsupported auth branch",
	)
	ErrConnectorUnsupportedFetchBranch = errors.New(
		"enable banking connector unsupported fetch branch",
	)
)

type connectorClient interface {
	CreateAuth(
		ctx context.Context,
		params enablebankingclient.CreateAuthParams,
	) (*enablebankingclient.CreateAuthResponse, error)
	CreateSession(
		ctx context.Context,
		params enablebankingclient.CreateSessionParams,
	) (*enablebankingclient.CreateSessionResponse, error)
	GetSession(
		ctx context.Context,
		params enablebankingclient.GetSessionParams,
	) (*enablebankingclient.SessionResponse, error)
	GetAccountDetails(
		ctx context.Context,
		params enablebankingclient.GetAccountDetailsParams,
	) (*enablebankingclient.GetAccountDetailsResponse, error)
	GetAccountBalances(
		ctx context.Context,
		params enablebankingclient.GetAccountBalancesParams,
	) (*enablebankingclient.GetAccountBalancesResponse, error)
	GetAccountTransactions(
		ctx context.Context,
		params enablebankingclient.GetAccountTransactionsParams,
	) (*enablebankingclient.GetAccountTransactionsResponse, error)
}

var _ connectorClient = (*enablebankingclient.Client)(nil)

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
	api            connectorClient
	logger         *slog.Logger
	stateProvider  func() (string, error)
	appID          string
	privateKeyPath string
	aspspName      string
	country        string
	psuType        string
	validDays      int
	now            func() time.Time
}

func WithNow(now func() time.Time) Option {
	return func(connector *Connector) {
		if now != nil {
			connector.now = now
		}
	}
}

func WithAPI(api connectorClient) Option {
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
	stateProvider := args.StateProvider
	if stateProvider == nil {
		stateProvider = func() (string, error) {
			return fmt.Sprintf("state-%d", time.Now().UnixNano()), nil
		}
	}
	now := args.Now
	if now == nil {
		now = time.Now
	}
	validDays := args.ValidDays
	if validDays <= 0 {
		validDays = defaultValidDays
	}
	logger := args.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	connector := &Connector{
		api: enablebankingclient.NewClient(enablebankingclient.Args{
			BaseURL:        args.BaseURL,
			HTTPClient:     httpClient,
			Logger:         logger,
			AppID:          args.AppID,
			PrivateKeyPath: args.PrivateKeyPath,
			Now:            now,
		}),
		logger:         logger.WithGroup("enableBankingConnector"),
		stateProvider:  stateProvider,
		appID:          strings.TrimSpace(args.AppID),
		privateKeyPath: strings.TrimSpace(args.PrivateKeyPath),
		aspspName:      strings.TrimSpace(args.ASPSPName),
		country:        strings.TrimSpace(args.Country),
		psuType:        strings.TrimSpace(args.PSUType),
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
		SupportsStartLink:    true,
		SupportsFinishLink:   true,
		RequiresRedirectCode: true,
		SupportsFetch:        true,
	}
}

func (c *Connector) StartLink(
	ctx context.Context,
	request providers.StartLinkRequest,
) (providers.StartLinkResult, error) {
	if !c.hasOfficialCredentials() {
		return providers.StartLinkResult{}, ErrConnectorUnsupportedAuthBranch
	}
	state, err := c.stateProvider()
	if err != nil {
		return providers.StartLinkResult{}, fmt.Errorf("enable banking start link: %w", err)
	}
	response, err := c.api.CreateAuth(ctx, enablebankingclient.CreateAuthParams{
		Request: c.buildOfficialStartLinkRequest(request.RedirectURL, state),
	})
	if err != nil {
		c.logLinkFailure(ctx, "redirectStart", "createAuth", err)
		return providers.StartLinkResult{}, fmt.Errorf("enable banking create auth: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"enable banking redirect authorization created",
		slog.String("operation", "redirectStart"),
	)
	authorizationURL := response.URL
	if authorizationURL == "" {
		return providers.StartLinkResult{}, errors.New(
			"enable banking auth response missing authorization URL",
		)
	}
	return providers.StartLinkResult{
		State:            state,
		AuthorizationURL: authorizationURL,
		PendingDocument:  mustJSON(response),
	}, nil
}

func (c *Connector) FinishLink(
	ctx context.Context,
	request providers.FinishLinkRequest,
) (providers.LinkResult, error) {
	if !c.hasOfficialCredentials() {
		return providers.LinkResult{}, ErrConnectorUnsupportedAuthBranch
	}
	response, err := c.api.CreateSession(ctx, enablebankingclient.CreateSessionParams{
		Request: &enablebankingclient.CreateSessionRequest{
			Code: request.Code,
		},
	})
	if err != nil {
		c.logLinkFailure(ctx, "redirectFinish", "createSession", err)
		return providers.LinkResult{}, fmt.Errorf("enable banking create session: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"enable banking redirect session created",
		slog.String("operation", "redirectFinish"),
	)
	providerReference := response.SessionID
	if providerReference == "" {
		return providers.LinkResult{}, errors.New(
			"enable banking session response missing session ID",
		)
	}
	providerObjectID := firstNonEmpty(providerReference, "session")
	return providers.LinkResult{
		DisplayName:       firstNonEmpty(aspspName(response.ASPSP), c.aspspName, "Enable Banking"),
		ProviderReference: providerReference,
		State:             domain.BankConnectionStateActive,
		ConnectionSnapshot: &domain.ProviderSnapshotObservation{
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: providerObjectID,
			DocumentJSON:     mustJSON(response),
			CapturedAt:       c.now(),
		},
	}, nil
}

func (c *Connector) LinkToken(
	context.Context,
	providers.LinkTokenRequest,
) (providers.LinkResult, error) {
	return providers.LinkResult{}, ErrConnectorTokenLinkUnsupported
}

func (c *Connector) Fetch(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	if !c.hasOfficialCredentials() {
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	}
	return c.fetchOfficial(ctx, request)
}

func (c *Connector) fetchOfficial(
	ctx context.Context,
	request providers.FetchRequest,
) (domain.ProviderSyncBatch, error) {
	if request.Connection.ProviderReference == "" ||
		request.Secret.ID != "" ||
		request.Secret.Reference != "" {
		return domain.ProviderSyncBatch{}, ErrConnectorUnsupportedFetchBranch
	}
	session, err := c.api.GetSession(ctx, enablebankingclient.GetSessionParams{
		SessionID: request.Connection.ProviderReference,
	})
	if err != nil {
		return domain.ProviderSyncBatch{}, fmt.Errorf("enable banking get session: %w", err)
	}
	c.logger.InfoContext(
		ctx,
		"fetched enable banking session",
		slog.String("connectionId", request.Connection.ConnectionID),
		slog.String("providerReference", request.Connection.ProviderReference),
		slog.Time("requestedStart", request.RequestedWindow.Start),
		slog.Time("requestedEnd", request.RequestedWindow.End),
		slog.Int("accountCount", len(session.Accounts)),
	)
	return c.mapBatch(ctx, request, session)
}

//nolint:funlen // Account enrichment, balances, and item snapshots stay aligned in one provider mapping flow.
func (c *Connector) mapBatch(
	ctx context.Context,
	request providers.FetchRequest,
	session *enablebankingclient.SessionResponse,
) (domain.ProviderSyncBatch, error) {
	capturedAt := c.now()
	accountItems := sessionAccounts(session)
	batch := newSyncBatch(request, session, capturedAt, len(accountItems))
	for _, typedAccount := range accountItems {
		accountID := typedAccount.UID
		if accountID == "" {
			continue
		}
		c.logger.InfoContext(
			ctx,
			"fetching enable banking account data",
			slog.String("connectionId", request.Connection.ConnectionID),
			slog.String("accountId", accountID),
		)
		enrichedAccount, accountSnapshot, hasAccountSnapshot, err := c.enrichAccountMetadata(
			ctx,
			accountID,
			typedAccount,
			capturedAt,
		)
		if err != nil {
			return domain.ProviderSyncBatch{}, err
		}
		typedAccount = enrichedAccount
		accountDocument := mustJSON(typedAccount)
		if hasAccountSnapshot {
			accountDocument = accountSnapshot.DocumentJSON
		}
		batch.Snapshots = append(batch.Snapshots, domain.ProviderSnapshotObservation{
			Kind:              domain.ProviderSnapshotKindAccount,
			ProviderObjectID:  accountID,
			ProviderAccountID: accountID,
			DocumentJSON:      accountDocument,
			CapturedAt:        capturedAt,
		})
		account := normalizeAccount(request.Connection, accountID, typedAccount)
		batch.Accounts = append(batch.Accounts, account)

		balancesResponse, err := c.api.GetAccountBalances(
			ctx,
			enablebankingclient.GetAccountBalancesParams{
				AccountID: accountID,
			},
		)
		if err != nil {
			return domain.ProviderSyncBatch{}, fmt.Errorf(
				"enable banking get account balances: %w",
				err,
			)
		}
		balance := normalizeBalance(
			request.Connection,
			account,
			balancesResponse.Balances,
			capturedAt,
		)
		batch.Balances = append(batch.Balances, balance)
		batch.Snapshots = append(batch.Snapshots, domain.ProviderSnapshotObservation{
			Kind:              domain.ProviderSnapshotKindAccountBalance,
			ProviderObjectID:  accountID,
			ProviderAccountID: accountID,
			DocumentJSON:      mustJSON(balancesResponse),
			CapturedAt:        capturedAt,
		})

		transactionPages, transactions, err := c.fetchTransactionPages(ctx, request, accountID)
		if err != nil {
			return domain.ProviderSyncBatch{}, err
		}
		c.logger.InfoContext(
			ctx,
			"fetched enable banking account transactions",
			slog.String("connectionId", request.Connection.ConnectionID),
			slog.String("accountId", accountID),
			slog.Int("pageCount", len(transactionPages)),
			slog.Int("transactionCount", len(transactions)),
		)
		for _, transaction := range transactions {
			normalized := normalizeTransaction(request.Connection, accountID, transaction)
			batch.Transactions = append(
				batch.Transactions,
				normalized,
			)
			batch.Snapshots = append(batch.Snapshots, domain.ProviderSnapshotObservation{
				Kind:                  domain.ProviderSnapshotKindTransaction,
				ProviderObjectID:      normalized.ProviderTransactionID,
				ProviderAccountID:     accountID,
				ProviderTransactionID: normalized.ProviderTransactionID,
				DocumentJSON:          mustJSON(transaction),
				CapturedAt:            capturedAt,
			})
		}
	}
	c.logger.InfoContext(
		ctx,
		"fetched enable banking sync batch",
		slog.String("connectionId", request.Connection.ConnectionID),
		slog.Int("accountCount", len(batch.Accounts)),
		slog.Int("balanceCount", len(batch.Balances)),
		slog.Int("transactionCount", len(batch.Transactions)),
		slog.Int("snapshotCount", len(batch.Snapshots)),
	)
	return batch, nil
}

func sessionAccounts(session *enablebankingclient.SessionResponse) []enablebankingclient.Account {
	if len(session.AccountsData) > 0 {
		return session.AccountsData
	}
	accounts := make([]enablebankingclient.Account, 0, len(session.Accounts))
	for _, accountID := range session.Accounts {
		accounts = append(accounts, enablebankingclient.Account{UID: accountID})
	}
	return accounts
}

func (c *Connector) logLinkFailure(
	ctx context.Context,
	operation string,
	failureStage string,
	err error,
) {
	attributes := []slog.Attr{
		slog.String("operation", operation),
		slog.String("failureStage", failureStage),
		slog.String("errorClass", "upstreamRequest"),
	}
	var responseErr *enablebankingclient.ResponseError
	if errors.As(err, &responseErr) {
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
	c.logger.LogAttrs(ctx, slog.LevelWarn, "enable banking link failed", attributes...)
}

func newSyncBatch(
	request providers.FetchRequest,
	session *enablebankingclient.SessionResponse,
	capturedAt time.Time,
	accountCount int,
) domain.ProviderSyncBatch {
	return domain.ProviderSyncBatch{
		Connection:      request.Connection,
		RequestedWindow: request.RequestedWindow,
		Accounts:        make([]domain.ProviderAccountObservation, 0, accountCount),
		Balances:        make([]domain.ProviderBalanceObservation, 0, accountCount),
		Transactions:    []domain.ProviderTransactionObservation{},
		Snapshots: []domain.ProviderSnapshotObservation{{
			Kind:             domain.ProviderSnapshotKindConnection,
			ProviderObjectID: firstNonEmpty(request.Connection.ProviderReference, "session"),
			DocumentJSON:     mustJSON(session),
			CapturedAt:       capturedAt,
		}},
	}
}

func (c *Connector) enrichAccountMetadata(
	ctx context.Context,
	accountID string,
	account enablebankingclient.Account,
	capturedAt time.Time,
) (enablebankingclient.Account, domain.ProviderSnapshotObservation, bool, error) {
	if !accountDetailsNeeded(account) {
		return account, domain.ProviderSnapshotObservation{}, false, nil
	}
	details, err := c.api.GetAccountDetails(ctx, enablebankingclient.GetAccountDetailsParams{
		AccountID: accountID,
	})
	if err != nil {
		return account, domain.ProviderSnapshotObservation{}, false,
			fmt.Errorf("enable banking get account details: %w", err)
	}
	account = mergeAccountDetails(account, details)
	return account, domain.ProviderSnapshotObservation{
		Kind:              domain.ProviderSnapshotKindAccount,
		ProviderObjectID:  accountID,
		ProviderAccountID: accountID,
		DocumentJSON:      mustJSON(details),
		CapturedAt:        capturedAt,
	}, true, nil
}

func accountDetailsNeeded(account enablebankingclient.Account) bool {
	return strings.TrimSpace(account.Name) == "" || strings.TrimSpace(account.Currency) == ""
}

func mergeAccountDetails(
	account enablebankingclient.Account,
	details *enablebankingclient.GetAccountDetailsResponse,
) enablebankingclient.Account {
	if details == nil {
		return account
	}
	if strings.TrimSpace(account.Name) == "" {
		account.Name = details.Name
	}
	if strings.TrimSpace(account.Details) == "" {
		account.Details = details.Details
	}
	if strings.TrimSpace(account.Product) == "" {
		account.Product = details.Product
	}
	if strings.TrimSpace(account.Currency) == "" {
		account.Currency = details.Currency
	}
	if account.AccountID == nil {
		account.AccountID = details.AccountID
	}
	return account
}

func (c *Connector) fetchTransactionPages(
	ctx context.Context,
	request providers.FetchRequest,
	accountID string,
) ([]*enablebankingclient.GetAccountTransactionsResponse, []enablebankingclient.AccountTransaction, error) {
	pages := make([]*enablebankingclient.GetAccountTransactionsResponse, 0, 1)
	transactions := make([]enablebankingclient.AccountTransaction, 0)
	continuationKey := ""
	for {
		page, err := c.api.GetAccountTransactions(
			ctx,
			enablebankingclient.GetAccountTransactionsParams{
				AccountID:       accountID,
				DateFrom:        request.RequestedWindow.Start,
				DateTo:          request.RequestedWindow.End,
				ContinuationKey: continuationKey,
			},
		)
		if err != nil {
			return nil, nil, fmt.Errorf("enable banking get account transactions: %w", err)
		}
		pages = append(pages, page)
		transactions = append(transactions, page.Transactions...)
		continuationKey = page.ContinuationKey
		if continuationKey == "" {
			break
		}
	}
	return pages, transactions, nil
}

func (c *Connector) hasOfficialCredentials() bool {
	return c.appID != "" && c.privateKeyPath != ""
}

func (c *Connector) buildOfficialStartLinkRequest(
	redirectURL string,
	state string,
) *enablebankingclient.CreateAuthRequest {
	validUntil := c.now().Add(time.Duration(c.validDays) * 24 * time.Hour)
	return &enablebankingclient.CreateAuthRequest{
		Access: enablebankingclient.CreateAuthAccess{
			ValidUntil: validUntil.Format(time.RFC3339),
		},
		ASPSP: enablebankingclient.CreateAuthASPSP{
			Name:    c.aspspName,
			Country: c.country,
		},
		State:       state,
		RedirectURL: redirectURL,
		PSUType:     c.psuType,
	}
}

func normalizeAccount(
	connection domain.ProviderConnectionRef,
	accountID string,
	account enablebankingclient.Account,
) domain.ProviderAccountObservation {
	return domain.ProviderAccountObservation{
		Connection:        connection,
		ProviderAccountID: accountID,
		Name:              firstNonEmpty(account.Name, account.Details, account.Product, accountID),
		Currency:          strings.ToUpper(account.Currency),
		IBAN:              accountIBAN(account),
	}
}

func accountIBAN(account enablebankingclient.Account) string {
	if account.AccountID == nil {
		return ""
	}
	return strings.TrimSpace(account.AccountID.IBAN)
}

func aspspName(aspsp *enablebankingclient.ASPSP) string {
	if aspsp == nil {
		return ""
	}
	return aspsp.Name
}

func normalizeBalance(
	connection domain.ProviderConnectionRef,
	account domain.ProviderAccountObservation,
	balances []enablebankingclient.AccountBalance,
	capturedAt time.Time,
) domain.ProviderBalanceObservation {
	current, available, currency := selectBalanceAmounts(balances)
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
	transaction enablebankingclient.AccountTransaction,
) domain.ProviderTransactionObservation {
	effectiveAt := transactionTime(transaction)
	description := firstNonEmpty(transactionNote(transaction), firstSliceValue(transaction.RemittanceInformation))
	currency := strings.ToUpper(transactionAmountCurrency(transaction.TransactionAmount))
	amountMinor := amountMinor(transaction)
	transactionID := firstNonEmpty(
		transaction.EntryReference,
		transaction.TransactionID,
		providerFingerprint(accountID, mustJSON(transaction)),
	)
	status := normalizeTransactionStatus(transaction.Status)
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
		ProviderAccountID:     accountID,
		ProviderTransactionID: transactionID,
		Status:                status,
		AmountMinor:           amountMinor,
		Currency:              currency,
		Description:           description,
		EffectiveAt:           effectiveAt,
		Fingerprint: providerFingerprint(
			accountID,
			description,
			amountMinor,
			currency,
			effectiveAt,
		),
		ProviderOriginal: providerOriginal,
	}
}

func selectBalanceAmounts(items []enablebankingclient.AccountBalance) (int64, *int64, string) {
	var current *int64
	var available *int64
	currency := ""
	for _, item := range items {
		amountMinor := decimalToMinor(balanceAmountValue(item.BalanceAmount))
		balanceType := strings.ToLower(strings.TrimSpace(item.BalanceType))
		switch balanceType {
		case "interimavailable", balanceAvailable, "availablebalance", "expectedavailable":
			value := amountMinor
			available = &value
		case "closingbooked", "booked", "current", "currentbalance":
			value := amountMinor
			current = &value
		}
		if currency == "" {
			currency = strings.ToUpper(balanceAmountCurrency(item.BalanceAmount))
		}
	}
	if current == nil && len(items) > 0 {
		fallback := decimalToMinor(balanceAmountValue(items[0].BalanceAmount))
		current = &fallback
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

func transactionTime(transaction enablebankingclient.AccountTransaction) time.Time {
	for _, value := range []string{
		transaction.TransactionDate,
		transaction.BookingDate,
		transaction.ValueDate,
	} {
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, time.DateOnly} {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func amountMinor(transaction enablebankingclient.AccountTransaction) int64 {
	amount := decimalToMinor(transactionAmountValue(transaction.TransactionAmount))
	if amount > 0 && strings.EqualFold(transaction.CreditDebitIndicator, "DBIT") {
		return -amount
	}
	return amount
}

func balanceAmountValue(amount *enablebankingclient.BalanceAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Amount
}

func balanceAmountCurrency(amount *enablebankingclient.BalanceAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Currency
}

func transactionAmountValue(amount *enablebankingclient.TransactionAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Amount
}

func transactionAmountCurrency(amount *enablebankingclient.TransactionAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Currency
}

func transactionNote(transaction enablebankingclient.AccountTransaction) string {
	if transaction.Note == nil {
		return ""
	}
	return *transaction.Note
}

func firstSliceValue(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

const enableBankingBookedStatus = "BOOKED"

func normalizeTransactionStatus(raw string) domain.TransactionStatus {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "BOOK", enableBankingBookedStatus:
		return domain.TransactionStatusBooked
	case "PDNG", "PENDING":
		return domain.TransactionStatusPending
	}
	if strings.TrimSpace(raw) == "" {
		return domain.TransactionStatusBooked
	}
	return domain.TransactionStatus(strings.ToLower(strings.TrimSpace(raw)))
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

func providerFingerprint(parts ...any) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = io.WriteString(hasher, fmt.Sprint(part, "|"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
