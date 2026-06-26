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
	raw, err := c.DoRawObject(
		ctx,
		DoRawJSONParams{Method: http.MethodGet, Path: path},
	)
	if err != nil {
		return nil, fmt.Errorf("get account details failed: %w", err)
	}
	accountRaw := objectValue(raw, "account")
	if len(accountRaw) == 0 {
		accountRaw = raw
	}
	return &GetAccountDetailsResponse{
		OwnerName: stringValue(accountRaw, "owner_name", "ownerName"),
		Product:   stringValue(accountRaw, "product"),
		BIC:       stringValue(accountRaw, "bic"),
		Raw:       raw,
	}, nil
}
