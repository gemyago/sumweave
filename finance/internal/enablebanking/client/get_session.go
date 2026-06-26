package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GetSessionParams contains session lookup parameters.
type GetSessionParams struct {
	SessionID string
}

// GetSession gets a session by ID.
func (c *Client) GetSession(ctx context.Context, params GetSessionParams) (*SessionResponse, error) {
	path := "/sessions/" + url.PathEscape(params.SessionID)
	raw, err := c.DoRawObject(ctx, DoRawJSONParams{Method: http.MethodGet, Path: path})
	if err != nil {
		return nil, fmt.Errorf("get session failed: %w", err)
	}
	return extractSessionResponse(raw), nil
}
