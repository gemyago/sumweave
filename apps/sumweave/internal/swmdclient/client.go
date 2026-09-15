package swmdclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/models"
)

const defaultRequestTimeout = 30 * time.Second

// APIError is the safe application error returned by the HTTP API.
type APIError struct {
	Status        int
	Code          string
	Message       string
	CorrelationID string
}

func (e *APIError) Error() string {
	if e.CorrelationID == "" {
		return fmt.Sprintf("API request failed (%d %s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("API request failed (%d %s, correlationId=%s): %s", e.Status, e.Code, e.CorrelationID, e.Message)
}

// Client makes authenticated JSON requests to the existing Sumweave API.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

// NewClient constructs a client with standard TLS verification and a finite timeout.
func NewClient(config Config, httpClient *http.Client) (*Client, error) {
	if err := validateConfigValues(config); err != nil {
		return nil, err
	}
	baseURL, err := parseBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	if httpClient.Timeout <= 0 {
		return nil, errors.New("HTTP client must have a finite timeout")
	}

	return &Client{baseURL: baseURL, token: config.APIToken, httpClient: httpClient}, nil
}

// Get performs a GET request and decodes its JSON response.
func (c *Client) Get(ctx context.Context, route string, query url.Values, output any) error {
	return c.do(ctx, http.MethodGet, route, query, nil, output, "")
}

// Post performs a JSON POST request and decodes its JSON response.
func (c *Client) Post(ctx context.Context, route string, requestBody, output any, idempotencyKey string) error {
	return c.do(ctx, http.MethodPost, route, nil, requestBody, output, idempotencyKey)
}

// CurrentUser calls the existing current-user route.
func (c *Client) CurrentUser(ctx context.Context) (*models.UserInfo, error) {
	output := &models.UserInfo{}
	return output, c.Get(ctx, "/api/v1/auth/me", nil, output)
}

// GetJob returns one observed job.
func (c *Client) GetJob(ctx context.Context, jobID string) (*models.JobDetailResponse, error) {
	output := &models.JobDetailResponse{}
	return output, c.Get(ctx, route("api", "v1", "jobs", jobID), nil, output)
}

// WaitOptions controls bounded observed-job polling.
type WaitOptions struct {
	Interval time.Duration
	Timeout  time.Duration
	Now      func() time.Time
	Sleep    func(context.Context, time.Duration) error
}

// WaitForJob polls an explicitly supplied expected job ID until it reaches a terminal state.
//
//nolint:govet // Error scopes are intentionally limited to individual polling operations.
func (c *Client) WaitForJob(ctx context.Context, jobID string, options WaitOptions) (*models.JobDetailResponse, error) {
	if jobID == "" {
		return nil, errors.New("job ID must not be empty")
	}
	if options.Interval <= 0 {
		return nil, errors.New("wait interval must be positive")
	}
	if options.Timeout <= 0 {
		return nil, errors.New("wait timeout must be positive")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Sleep == nil {
		options.Sleep = sleepContext
	}

	startedAt := options.Now()
	deadline := startedAt.Add(options.Timeout)
	graceDeadline := startedAt.Add(30 * time.Second)
	for {
		job, err := c.GetJob(ctx, jobID)
		if err == nil {
			switch job.Status {
			case "succeeded":
				return job, nil
			case "failed":
				return job, errors.New("job failed")
			}
		} else {
			var apiError *APIError
			if !errors.As(err, &apiError) || apiError.Status != http.StatusNotFound ||
				!options.Now().Before(graceDeadline) {
				return nil, err
			}
		}

		now := options.Now()
		if !now.Before(deadline) {
			return nil, errors.New("job wait timed out")
		}
		remaining := deadline.Sub(now)
		waitFor := min(options.Interval, remaining)
		if err := options.Sleep(ctx, waitFor); err != nil {
			return nil, fmt.Errorf("wait for job update: %w", err)
		}
	}
}

//nolint:govet // Error scopes are intentionally limited to individual HTTP operations.
func (c *Client) do(
	ctx context.Context,
	method, routePath string,
	query url.Values,
	requestBody, output any,
	idempotencyKey string,
) error {
	relativeURL, err := url.Parse(strings.TrimPrefix(routePath, "/"))
	if err != nil {
		return fmt.Errorf("parse API route: %w", err)
	}
	relativeURL.RawQuery = query.Encode()
	requestURL := c.baseURL.ResolveReference(relativeURL)

	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode API request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send API request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(response)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

func decodeAPIError(response *http.Response) error {
	apiError := &APIError{Status: response.StatusCode, CorrelationID: response.Header.Get("X-Correlation-ID")}
	var payload models.Error
	if err := json.NewDecoder(response.Body).Decode(&payload); err == nil {
		apiError.Code = payload.Code
		apiError.Message = payload.Message
		if payload.CorrelationID != "" {
			apiError.CorrelationID = payload.CorrelationID
		}
	}
	if apiError.Code == "" {
		apiError.Code = "http_error"
	}
	if apiError.Message == "" {
		apiError.Message = response.Status
	}
	return apiError
}

func parseBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("base URL must use http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("base URL must contain only an origin and optional path")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return nil, errors.New("plain HTTP is allowed only for localhost or loopback addresses")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func route(parts ...string) string {
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, url.PathEscape(part))
	}
	return "/" + path.Join(encoded...)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
