package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	rawItems, err := c.DoRawArray(ctx, DoRawJSONParams{Method: http.MethodGet, Path: "/aspsps", Query: query})
	if err != nil {
		return nil, fmt.Errorf("list aspsps failed: %w", err)
	}
	response := &ListASPSPsResponse{Raw: rawItems, ASPSPs: make([]ASPSP, 0, len(rawItems))}
	for _, item := range rawItems {
		response.ASPSPs = append(response.ASPSPs, ASPSP{
			ID:      stringValue(item, "id"),
			Name:    stringValue(item, "name"),
			Country: stringValue(item, "country"),
			Raw:     item,
		})
	}
	return response, nil
}
