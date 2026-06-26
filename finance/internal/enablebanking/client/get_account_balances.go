package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	raw, err := c.DoRawObject(
		ctx,
		DoRawJSONParams{Method: http.MethodGet, Path: path},
	)
	if err != nil {
		return nil, fmt.Errorf("get account balances failed: %w", err)
	}
	items := objectSlice(raw, "balances")
	balances := make([]AccountBalance, 0, len(items))
	for _, item := range items {
		amount := amountObject(item)
		balances = append(balances, AccountBalance{
			Type:                  stringValue(item, "type"),
			CurrentBalanceMinor:   int64Value(item, "currentBalanceMinor"),
			AvailableBalanceMinor: int64Value(item, "availableBalanceMinor"),
			BalanceAmount: &BalanceAmount{
				Amount:   stringValue(amount, "amount"),
				Currency: strings.ToUpper(stringValue(amount, "currency")),
				Raw:      amount,
			},
			Raw: item,
		})
	}
	return &GetAccountBalancesResponse{Balances: balances, Raw: raw}, nil
}
