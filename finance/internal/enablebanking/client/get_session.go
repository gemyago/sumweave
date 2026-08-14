package client

import (
	"context"
	"fmt"
	"log/slog"
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
	result, err := sendJSON[struct{}, SessionResponse](ctx, c, sendJSONParams[struct{}]{
		Method: http.MethodGet,
		Path:   path,
	})
	if err != nil {
		return nil, fmt.Errorf("get session failed: %w", err)
	}
	response := result.Value
	c.logger.DebugContext(ctx, "fetched enable banking session", slog.Int("accountCount", len(response.AccountsData)))
	return response, nil
}
