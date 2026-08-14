package client

import (
	"context"
	"fmt"
	"net/http"
)

// CreateAuthParams contains auth creation parameters.
type CreateAuthParams struct {
	Request *CreateAuthRequest
}

// CreateAuth starts an authorization flow.
func (c *Client) CreateAuth(ctx context.Context, params CreateAuthParams) (*CreateAuthResponse, error) {
	result, err := sendJSON[CreateAuthRequest, CreateAuthResponse](ctx, c, sendJSONParams[CreateAuthRequest]{
		Method: http.MethodPost,
		Path:   authPath,
		Body:   params.Request,
	})
	if err != nil {
		return nil, fmt.Errorf("create auth failed: %w", err)
	}
	return result.Value, nil
}
