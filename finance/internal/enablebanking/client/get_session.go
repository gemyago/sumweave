package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
	if strings.TrimSpace(response.SessionID) == "" {
		response.SessionID = strings.TrimSpace(params.SessionID)
	}
	response.AccountsData = normalizeAccounts(response.AccountsData)
	c.logger.DebugContext(ctx, "fetched enable banking session",
		slog.String("sessionId", params.SessionID),
		slog.Any("session", response),
	)
	return normalizeSessionResponse(response), nil
}
