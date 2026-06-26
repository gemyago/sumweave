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
func (c *Client) CreateSession(ctx context.Context, params CreateSessionParams) (*SessionResponse, error) {
	raw, err := c.DoRawObject(ctx, DoRawJSONParams{Method: http.MethodPost, Path: "/sessions", Body: params.Request})
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}
	return extractSessionResponse(raw), nil
}
