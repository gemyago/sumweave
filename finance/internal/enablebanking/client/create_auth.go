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
	raw, err := c.DoRawObject(
		ctx,
		DoRawJSONParams{Method: http.MethodPost, Path: authPath, Body: params.Request},
	)
	if err != nil {
		return nil, fmt.Errorf("create auth failed: %w", err)
	}
	return &CreateAuthResponse{
		AuthorizationURL: firstNonEmpty(
			stringValue(raw, "authorizationUrl", "authorization_url", "url"),
			stringValue(raw, "authorizationURL"),
		),
		ID: extractSessionIdentifier(
			raw,
			"authorization_id",
			"auth_id",
			"id",
		),
		ProviderReference: firstNonEmpty(
			stringValue(raw, "providerReference", "provider_reference"),
			extractSessionIdentifier(
				raw,
				"authorization_id",
				"auth_id",
				"id",
				"session_id",
			),
		),
		Raw: raw,
	}, nil
}
