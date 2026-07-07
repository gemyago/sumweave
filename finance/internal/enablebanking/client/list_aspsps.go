package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ListASPSPsParams contains ASPSP query parameters.
type ListASPSPsParams struct {
	Country string
}

// ListASPSPs lists available ASPSPs.
func (c *Client) ListASPSPs(ctx context.Context, params ListASPSPsParams) (*ListASPSPsResponse, error) {
	query := url.Values{}
	if params.Country != "" {
		query.Set("country", params.Country)
	}
	result, err := sendJSON[struct{}, ListASPSPsResponse](ctx, c, sendJSONParams[struct{}]{
		Method: http.MethodGet,
		Path:   "/aspsps",
		Query:  query,
	})
	if err != nil {
		return nil, fmt.Errorf("list aspsps failed: %w", err)
	}
	response := result.Value
	for index := range response.ASPSPs {
		response.ASPSPs[index].Country = strings.TrimSpace(response.ASPSPs[index].Country)
	}
	return response, nil
}
