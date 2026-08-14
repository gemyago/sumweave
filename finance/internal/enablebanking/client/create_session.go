package client

import (
	"context"
	"fmt"
	"net/http"
)

// CreateSessionParams contains session creation parameters.
type CreateSessionParams struct {
	Request *CreateSessionRequest
}

// CreateSession exchanges an auth code for a session.
func (c *Client) CreateSession(ctx context.Context, params CreateSessionParams) (*CreateSessionResponse, error) {
	result, err := sendJSON[CreateSessionRequest, CreateSessionResponse](ctx, c, sendJSONParams[CreateSessionRequest]{
		Method: http.MethodPost,
		Path:   "/sessions",
		Body:   params.Request,
	})
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}
	return result, nil
}
