package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetPersonalClientInfoParams contains request parameters.
type GetPersonalClientInfoParams struct {
	Token string
}

// GetPersonalClientInfoResponse contains the decoded response and raw payload.
type GetPersonalClientInfoResponse struct {
	ClientInfo *Info
	RawJSON    []byte
}

// GetPersonalClientInfo loads personal client metadata.
func (c *Client) GetPersonalClientInfo(
	ctx context.Context,
	params GetPersonalClientInfoParams,
) (*GetPersonalClientInfoResponse, error) {
	rawJSON, err := c.doRequest(ctx, http.MethodGet, "/personal/client-info", params.Token)
	if err != nil {
		return nil, fmt.Errorf("get personal client info: %w", err)
	}

	var body Info
	if decodeErr := json.Unmarshal(rawJSON, &body); decodeErr != nil {
		return nil, fmt.Errorf("get personal client info: decode response: %w", decodeErr)
	}

	return &GetPersonalClientInfoResponse{
		ClientInfo: &body,
		RawJSON:    rawJSON,
	}, nil
}
