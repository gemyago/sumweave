package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetAccountBalancesParams contains balance lookup parameters.
type GetAccountBalancesParams struct {
	AccountID string
}

// GetAccountBalances gets balances for an account.
func (c *Client) GetAccountBalances(
	ctx context.Context,
	params GetAccountBalancesParams,
) (*GetAccountBalancesResponse, error) {
	path := "/accounts/" + url.PathEscape(params.AccountID) + "/balances"
	result, err := sendJSON[struct{}, GetAccountBalancesResponse](ctx, c, sendJSONParams[struct{}]{
		Method: http.MethodGet,
		Path:   path,
	})
	if err != nil {
		return nil, fmt.Errorf("get account balances failed: %w", err)
	}
	return result, nil
}
