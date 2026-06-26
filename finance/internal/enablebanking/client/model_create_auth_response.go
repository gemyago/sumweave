package client

// CreateAuthResponse models an auth response.
type CreateAuthResponse struct {
	AuthorizationURL  string         `json:"authorizationUrl,omitempty"`
	ID                string         `json:"id,omitempty"`
	ProviderReference string         `json:"providerReference,omitempty"`
	Raw               map[string]any `json:"-"`
}
