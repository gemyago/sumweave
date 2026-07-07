package client

// ASPSP models an Enable Banking ASPSP item.
type ASPSP struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Country string `json:"country,omitempty"`
}

// ListASPSPsResponse models the ASPSP list response.
type ListASPSPsResponse struct {
	ASPSPs []ASPSP `json:"aspsps,omitempty"`
}
