package client

// GetAccountDetailsResponse models account details.
type GetAccountDetailsResponse struct {
	OwnerName string         `json:"ownerName,omitempty"`
	Product   string         `json:"product,omitempty"`
	BIC       string         `json:"bic,omitempty"`
	Raw       map[string]any `json:"-"`
}
