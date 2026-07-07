package client

// CreateAuthResponse models an auth response.
type CreateAuthResponse struct {
	URL               string `json:"url,omitempty"`
	AuthorizationID   string `json:"authorization_id,omitempty"`
	PSUIDHash         string `json:"psu_id_hash,omitempty"`
	AuthorizationURL  string `json:"-"`
	ID                string `json:"-"`
	ProviderReference string `json:"-"`
}
