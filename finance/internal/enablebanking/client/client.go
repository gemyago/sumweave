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

// DoRawJSONParams describes a raw JSON request.
type DoRawJSONParams struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
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
		logger = slog.Default()
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

// DoRawObject executes a request and decodes a JSON object.
func (c *Client) DoRawObject(ctx context.Context, params DoRawJSONParams) (map[string]any, error) {
	body, err := c.do(ctx, params)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if decodeErr := json.Unmarshal(body, &raw); decodeErr != nil {
		return nil, fmt.Errorf("enable banking response decode: %w", decodeErr)
	}
	return raw, nil
}

// DoRawArray executes a request and decodes a JSON array.
func (c *Client) DoRawArray(ctx context.Context, params DoRawJSONParams) ([]map[string]any, error) {
	body, err := c.do(ctx, params)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if decodeErr := json.Unmarshal(body, &raw); decodeErr != nil {
		return nil, fmt.Errorf("enable banking response decode: %w", decodeErr)
	}
	return raw, nil
}

func (c *Client) do(ctx context.Context, params DoRawJSONParams) ([]byte, error) {
	var body io.Reader
	if params.Body != nil {
		encoded, err := json.Marshal(params.Body)
		if err != nil {
			return nil, fmt.Errorf("enable banking request encode: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	endpoint, err := url.Parse(c.baseURL + params.Path)
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
	if err = c.applyAuthorization(ctx, request); err != nil {
		return nil, err
	}

	response, err := c.httpClient.Do(request)
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
		token, err := c.newSignedAccessToken(c.now().UTC())
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
		"iat": jwt.NewNumericDate(now.UTC()),
		"exp": jwt.NewNumericDate(now.UTC().Add(jwtLifetime)),
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

func intValue(raw map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case int:
			return value
		case int32:
			return int(value)
		case int64:
			return int(value)
		case float64:
			return int(value)
		}
	}
	return 0
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

func objectValue(raw map[string]any, key string) map[string]any {
	value, _ := raw[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
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

func extractAccount(raw map[string]any) Account {
	return Account{
		UID:      firstNonEmpty(stringValue(raw, "uid"), stringValue(raw, "id")),
		ID:       firstNonEmpty(stringValue(raw, "id"), stringValue(raw, "uid")),
		Name:     stringValue(raw, "name"),
		IBAN:     stringValue(raw, "iban"),
		Currency: strings.ToUpper(stringValue(raw, "currency")),
		Raw:      raw,
	}
}

func extractAccounts(raw map[string]any) []Account {
	items := objectSlice(raw, "accounts")
	accounts := make([]Account, 0, len(items))
	for _, item := range items {
		accounts = append(accounts, extractAccount(item))
	}
	return accounts
}

func extractSessionResponse(raw map[string]any) *SessionResponse {
	return &SessionResponse{
		ID:        stringValue(raw, "id"),
		SessionID: firstNonEmpty(stringValue(raw, "session_id"), stringValue(raw, "id")),
		ExternalID: firstNonEmpty(
			stringValue(raw, "externalId"),
			stringValue(raw, "external_id"),
			stringValue(raw, "id"),
		),
		ProviderReference: firstNonEmpty(
			stringValue(raw, "providerReference", "provider_reference"),
			extractSessionIdentifier(raw, "id", "session_id"),
		),
		DisplayName: stringValue(raw, "displayName", "display_name"),
		Secret:      stringValue(raw, "secret"),
		State:       stringValue(raw, "state"),
		Access:      extractSessionAccess(raw),
		Accounts:    extractAccounts(raw),
		Raw:         raw,
	}
}

func extractSessionAccess(raw map[string]any) *SessionAccess {
	access := objectValue(raw, "access")
	if len(access) == 0 {
		return nil
	}
	validForDays := intValue(access, "valid_for_days")
	validUntil := stringValue(access, "valid_until")
	if validForDays == 0 && validUntil == "" {
		return nil
	}
	return &SessionAccess{
		ValidForDays: validForDays,
		ValidUntil:   validUntil,
		Raw:          access,
	}
}

func extractSessionIdentifier(raw map[string]any, keys ...string) string {
	identifier := stringValue(raw, keys...)
	if identifier != "" {
		return identifier
	}
	return stringValue(objectValue(raw, "session"), keys...)
}

func amountObject(raw map[string]any) map[string]any {
	if amount := objectValue(raw, "amount"); len(amount) > 0 {
		return amount
	}
	if amount := objectValue(raw, "balance_amount"); len(amount) > 0 {
		return amount
	}
	return map[string]any{}
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
