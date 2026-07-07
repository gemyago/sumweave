package client

import (
	"context"
	"fmt"
	"net/http"
)

type createSessionResponse struct {
	SessionID string         `json:"session_id,omitempty"`
	Access    *SessionAccess `json:"access,omitempty"`
	Accounts  []Account      `json:"accounts,omitempty"`
	ASPSP     *ASPSP         `json:"aspsp,omitempty"`
	PSUType   string         `json:"psu_type,omitempty"`
}

// CreateSessionParams contains session creation parameters.
type CreateSessionParams struct {
	Request *CreateSessionRequest
}

// CreateSession exchanges an auth code for a session.
func (c *Client) CreateSession(ctx context.Context, params CreateSessionParams) (*SessionResponse, error) {
	result, err := sendJSON[CreateSessionRequest, createSessionResponse](ctx, c, sendJSONParams[CreateSessionRequest]{
		Method: http.MethodPost,
		Path:   "/sessions",
		Body:   params.Request,
	})
	if err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}
	wireResponse := result.Value
	response := &SessionResponse{
		SessionID: wireResponse.SessionID,
		Access:    wireResponse.Access,
		Accounts:  normalizeAccounts(wireResponse.Accounts),
		ASPSP:     wireResponse.ASPSP,
		PSUType:   wireResponse.PSUType,
	}
	response.Accounts = normalizeAccounts(response.Accounts)
	response.AccountIDs = accountIDsFromAccounts(response.Accounts)
	return normalizeSessionResponse(response), nil
}
