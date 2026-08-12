package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultBaseURL   = "https://api.enablebanking.com"
	authPath         = "/auth"
	jwtIssuer        = "enablebanking.com"
	jwtAudience      = "api.enablebanking.com"
	jwtLifetime      = 5 * time.Minute
	decimalSplitPart = 3
)

type bearerTokenContextKey struct{}

// Client calls the Enable Banking HTTP API.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	logger         *slog.Logger
	appID          string
	privateKeyPath string
	now            func() time.Time
}

// Args configures a Client.
type Args struct {
	BaseURL        string
	HTTPClient     *http.Client
	Logger         *slog.Logger
	AppID          string
	PrivateKeyPath string
	Now            func() time.Time
}

type sendJSONParams[TBody any] struct {
	Method string
	Path   string
	Query  url.Values
	Body   *TBody
}

type sendJSONResult[TTarget any] struct {
	Value *TTarget
	Body  []byte
}

// ResponseError reports a non-2xx upstream response.
type ResponseError struct {
	Operation  string
	StatusCode int
	Code       string
	Message    string
	Body       []byte
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf(
		"enable banking %s failed with status %d: %s",
		e.Operation,
		e.StatusCode,
		e.Message,
	)
}

// NewClient creates an Enable Banking client.
func NewClient(args Args) *Client {
	httpClient := args.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	logger := args.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	now := args.Now
	if now == nil {
		now = time.Now
	}
	baseURL := strings.TrimRight(strings.TrimSpace(args.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		httpClient:     httpClient,
		baseURL:        baseURL,
		logger:         logger.WithGroup("enable-banking-http-client"),
		appID:          strings.TrimSpace(args.AppID),
		privateKeyPath: strings.TrimSpace(args.PrivateKeyPath),
		now:            now,
	}
}

// WithBearerToken stores a bearer token in context.
func WithBearerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, bearerTokenContextKey{}, strings.TrimSpace(token))
}

func sendJSON[TBody any, TTarget any](
	ctx context.Context,
	client *Client,
	params sendJSONParams[TBody],
) (*sendJSONResult[TTarget], error) {
	body, err := doJSONRequest(ctx, client, params)
	if err != nil {
		return nil, err
	}
	var target TTarget
	if err = json.Unmarshal(body, &target); err != nil {
		return nil, fmt.Errorf("enable banking response decode: %w", err)
	}
	return &sendJSONResult[TTarget]{
		Value: &target,
		Body:  append([]byte(nil), body...),
	}, nil
}

func doJSONRequest[TBody any](
	ctx context.Context,
	client *Client,
	params sendJSONParams[TBody],
) ([]byte, error) {
	var body io.Reader
	if params.Body != nil {
		encoded, err := json.Marshal(params.Body)
		if err != nil {
			return nil, fmt.Errorf("enable banking request encode: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	endpoint, err := url.Parse(client.baseURL + params.Path)
	if err != nil {
		return nil, fmt.Errorf("enable banking request build: %w", err)
	}
	if len(params.Query) > 0 {
		endpoint.RawQuery = params.Query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, params.Method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("enable banking request build: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if params.Body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if err = client.applyAuthorization(ctx, request); err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("enable banking request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("enable banking response read: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		message, code := parseResponseBody(responseBody)
		if message == "" {
			message = "provider request failed"
		}
		return nil, &ResponseError{
			Operation:  strings.TrimPrefix(params.Path, "/"),
			StatusCode: response.StatusCode,
			Code:       code,
			Message:    message,
			Body:       append([]byte(nil), responseBody...),
		}
	}

	return responseBody, nil
}

func (c *Client) applyAuthorization(ctx context.Context, request *http.Request) error {
	if c.usesSignedRequests() {
		token, err := c.newSignedAccessToken(c.now())
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	token, _ := ctx.Value(bearerTokenContextKey{}).(string)
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	return nil
}

func (c *Client) usesSignedRequests() bool {
	return c.appID != "" || c.privateKeyPath != ""
}

func (c *Client) newSignedAccessToken(now time.Time) (string, error) {
	if c.appID == "" {
		return "", errors.New("enable banking app ID is required")
	}
	if c.privateKeyPath == "" {
		return "", errors.New("enable banking private key path is required")
	}
	privateKeyPEM, err := os.ReadFile(c.privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("read enable banking private key file: %w", err)
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("parse enable banking private key file: %w", err)
	}
	claims := jwt.MapClaims{
		"iss": jwtIssuer,
		"aud": jwtAudience,
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(jwtLifetime)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = c.appID
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign enable banking JWT: %w", err)
	}
	return signedToken, nil
}

func parseResponseBody(body []byte) (string, string) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	message := firstNonEmpty(
		stringValue(payload, "message"),
		stringValue(payload, "error_description"),
		stringValue(payload, "detail"),
		stringValue(payload, "error"),
	)
	return message, stringValue(payload, "error")
}

func stringValue(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if stringValue, okCast := value.(string); okCast {
			trimmed := strings.TrimSpace(stringValue)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
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
	whole, _ := strconv.ParseInt(firstNonEmpty(parts[0], "0"), 10, 64)
	frac := "00"
	if len(parts) > 1 {
		frac = parts[1] + "00"
	}
	fracValue, _ := strconv.ParseInt(frac[:2], 10, 64)
	return sign * (whole*100 + fracValue)
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

func normalizeAccount(account Account) Account {
	account.Currency = strings.ToUpper(account.Currency)
	if account.AccountID != nil {
		account.IBAN = strings.TrimSpace(account.AccountID.IBAN)
	}
	if account.ID == "" {
		account.ID = account.UID
	}
	return account
}

func normalizeAccounts(accounts []Account) []Account {
	if len(accounts) == 0 {
		return nil
	}
	result := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, normalizeAccount(account))
	}
	return result
}

func accountIDsFromAccounts(accounts []Account) []string {
	if len(accounts) == 0 {
		return nil
	}
	result := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountID := firstNonEmpty(account.UID, account.ID)
		if accountID == "" {
			continue
		}
		result = append(result, accountID)
	}
	return result
}

func normalizeSessionResponse(response *SessionResponse) *SessionResponse {
	if response == nil {
		return nil
	}
	response.ID = response.SessionID
	response.ProviderReference = response.SessionID
	response.DisplayName = ""
	if response.ASPSP != nil {
		response.DisplayName = response.ASPSP.Name
	}
	response.State = response.Status
	response.AccountsData = normalizeAccounts(response.AccountsData)
	response.Accounts = normalizeAccounts(response.Accounts)
	if len(response.Accounts) == 0 {
		switch {
		case len(response.AccountsData) > 0:
			response.Accounts = append([]Account(nil), response.AccountsData...)
		case len(response.AccountIDs) > 0:
			response.Accounts = make([]Account, 0, len(response.AccountIDs))
			for _, accountID := range response.AccountIDs {
				trimmed := strings.TrimSpace(accountID)
				if trimmed == "" {
					continue
				}
				response.Accounts = append(response.Accounts, Account{UID: trimmed, ID: trimmed})
			}
		}
	}
	return response
}

func normalizeCreateAuthResponse(response *CreateAuthResponse) *CreateAuthResponse {
	if response == nil {
		return nil
	}
	response.AuthorizationURL = response.URL
	response.ID = response.AuthorizationID
	response.ProviderReference = response.AuthorizationID
	return response
}

func normalizeAccountDetailsResponse(response *GetAccountDetailsResponse) *GetAccountDetailsResponse {
	if response == nil {
		return nil
	}
	response.Currency = strings.ToUpper(response.Currency)
	if response.AccountID != nil {
		response.IBAN = strings.TrimSpace(response.AccountID.IBAN)
	}
	response.OwnerName = response.Name
	if response.AccountServicer != nil {
		response.BIC = response.AccountServicer.BICFI
	}
	return response
}

func normalizeBalances(response *GetAccountBalancesResponse) *GetAccountBalancesResponse {
	if response == nil {
		return nil
	}
	for index := range response.Balances {
		response.Balances[index].Type = response.Balances[index].BalanceType
		if response.Balances[index].BalanceAmount != nil {
			response.Balances[index].BalanceAmount.Currency = strings.ToUpper(
				response.Balances[index].BalanceAmount.Currency,
			)
		}
	}
	return response
}

func normalizeTransactions(response *GetAccountTransactionsResponse) *GetAccountTransactionsResponse {
	if response == nil {
		return nil
	}
	for index := range response.Transactions {
		transaction := &response.Transactions[index]
		transaction.Amount = transaction.TransactionAmount
		transaction.Currency = strings.ToUpper(firstNonEmpty(
			transaction.Currency,
			transactionAmountCurrencyValue(transaction.TransactionAmount),
		))
		transaction.Description = firstNonEmpty(transaction.Note, firstSliceValue(transaction.RemittanceInformation))
		transaction.RemittanceInformationUnstructured = firstNonEmpty(
			firstSliceValue(transaction.RemittanceInformation),
			transaction.Note,
		)
		transaction.EffectiveAt = firstNonEmpty(
			transaction.TransactionDate,
			transaction.BookingDate,
			transaction.ValueDate,
		)
		transaction.AmountMinor = signedTransactionAmountMinor(*transaction)
		if transaction.ID == "" {
			transaction.ID = transaction.EntryReference
		}
	}
	return response
}

func signedTransactionAmountMinor(transaction AccountTransaction) int64 {
	amount := decimalToMinor(transactionAmountValue(transaction.TransactionAmount))
	if amount > 0 && strings.EqualFold(transaction.CreditDebitIndicator, "DBIT") {
		return -amount
	}
	return amount
}

func transactionAmountValue(amount *TransactionAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Amount
}

func transactionAmountCurrencyValue(amount *TransactionAmount) string {
	if amount == nil {
		return ""
	}
	return amount.Currency
}

func firstSliceValue(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
