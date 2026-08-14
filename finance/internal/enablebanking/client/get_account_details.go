package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetAccountDetailsParams contains account details parameters.
type GetAccountDetailsParams struct {
	AccountID string
}

// GetAccountDetails gets account details.
func (c *Client) GetAccountDetails(
	ctx context.Context,
	params GetAccountDetailsParams,
) (*GetAccountDetailsResponse, error) {
	path := "/accounts/" + url.PathEscape(params.AccountID) + "/details"
	result, err := sendJSON[struct{}, GetAccountDetailsResponse](ctx, c, sendJSONParams[struct{}]{
		Method: http.MethodGet,
		Path:   path,
	})
	if err != nil {
		return nil, fmt.Errorf("get account details failed: %w", err)
	}
	return result.Value, nil
}
