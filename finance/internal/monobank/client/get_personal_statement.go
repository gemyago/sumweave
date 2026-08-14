package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetPersonalStatementParams contains request parameters.
type GetPersonalStatementParams struct {
	Token   string
	Account string
	From    int64
	To      int64
}

// GetPersonalStatementResponse contains decoded statement items.
type GetPersonalStatementResponse struct {
	Items []PersonalStatementItem
}

// GetPersonalStatement loads account statement items for the given range.
func (c *Client) GetPersonalStatement(
	ctx context.Context,
	params GetPersonalStatementParams,
) (*GetPersonalStatementResponse, error) {
	path := fmt.Sprintf(
		"/personal/statement/%s/%d/%d",
		url.PathEscape(params.Account),
		params.From,
		params.To,
	)
	rawJSON, err := c.doRequest(ctx, http.MethodGet, path, params.Token)
	if err != nil {
		return nil, fmt.Errorf("get personal statement: %w", err)
	}

	var body []PersonalStatementItem
	if decodeErr := json.Unmarshal(rawJSON, &body); decodeErr != nil {
		return nil, fmt.Errorf("get personal statement: decode response: %w", decodeErr)
	}

	return &GetPersonalStatementResponse{Items: body}, nil
}
