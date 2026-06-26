package client

// ASPSP models an Enable Banking ASPSP item.
type ASPSP struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Country string         `json:"country,omitempty"`
	Raw     map[string]any `json:"-"`
}

// ListASPSPsResponse models the ASPSP list response.
type ListASPSPsResponse struct {
	ASPSPs []ASPSP          `json:"aspsps"`
	Raw    []map[string]any `json:"-"`
}
