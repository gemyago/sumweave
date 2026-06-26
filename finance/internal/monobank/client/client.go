package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Client calls the Monobank personal API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// Args contains client construction arguments.
type Args struct {
	BaseURL string
}

type clientOpts struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// Option configures the client.
type Option func(*clientOpts)

// APIError represents a non-success Monobank HTTP response.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("monobank request failed with status %d", e.StatusCode)
}

// WithHTTPClient sets the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(opts *clientOpts) {
		if httpClient != nil {
			opts.httpClient = httpClient
		}
	}
}

// WithLogger sets the logger used by the client.
func WithLogger(logger *slog.Logger) Option {
	return func(opts *clientOpts) {
		if logger != nil {
			opts.logger = logger
		}
	}
}

// NewClient builds a Monobank client.
func NewClient(args Args, opts ...Option) *Client {
	resolvedOpts := clientOpts{
		httpClient: http.DefaultClient,
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(&resolvedOpts)
	}

	return &Client{
		httpClient: resolvedOpts.httpClient,
		baseURL:    strings.TrimRight(strings.TrimSpace(args.BaseURL), "/"),
		logger:     resolvedOpts.logger,
	}
}

func (c *Client) doRequest(
	ctx context.Context,
	method string,
	path string,
	token string,
) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Token", strings.TrimSpace(token))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, &APIError{StatusCode: response.StatusCode, Body: body}
	}

	return body, nil
}
