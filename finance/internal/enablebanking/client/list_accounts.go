package client

import (
	"context"
	"fmt"
	"net/http"
)

// ListAccountsParams contains account list parameters.
type ListAccountsParams struct{}

// ListAccounts lists accounts for the current token.
func (c *Client) ListAccounts(ctx context.Context, _ ListAccountsParams) (*ListAccountsResponse, error) {
	raw, err := c.DoRawObject(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/accounts"})
	if err != nil {
		return nil, fmt.Errorf("list accounts failed: %w", err)
	}
	return &ListAccountsResponse{
		State:            stringValue(raw, "state"),
		ReauthReason:     stringValue(raw, "reauthReason"),
		ReauthRequiredAt: stringValue(raw, "reauthRequiredAt"),
		Accounts:         extractAccounts(raw),
		Raw:              raw,
	}, nil
}
