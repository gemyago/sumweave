package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GetAccountTransactionsParams contains transaction lookup parameters.
type GetAccountTransactionsParams struct {
	AccountID       string
	DateFrom        time.Time
	DateTo          time.Time
	Strategy        string
	Status          string
	ContinuationKey string
}

// GetAccountTransactions gets transactions for an account.
func (c *Client) GetAccountTransactions(
	ctx context.Context,
	params GetAccountTransactionsParams,
) (*GetAccountTransactionsResponse, error) {
	query := url.Values{}
	if !params.DateFrom.IsZero() {
		query.Set("date_from", params.DateFrom.Format(time.DateOnly))
	}
	if !params.DateTo.IsZero() {
		query.Set("date_to", params.DateTo.Format(time.DateOnly))
	}
	if strings.TrimSpace(params.Strategy) != "" {
		query.Set("strategy", strings.TrimSpace(params.Strategy))
	}
	if strings.TrimSpace(params.Status) != "" {
		query.Set("transaction_status", strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.ContinuationKey) != "" {
		query.Set("continuation_key", strings.TrimSpace(params.ContinuationKey))
	}
	path := "/accounts/" + url.PathEscape(params.AccountID) + "/transactions"
	result, err := sendJSON[struct{}, GetAccountTransactionsResponse](ctx, c, sendJSONParams[struct{}]{
		Method: http.MethodGet,
		Path:   path,
		Query:  query,
	})
	if err != nil {
		return nil, fmt.Errorf("get account transactions failed: %w", err)
	}
	return result, nil
}
