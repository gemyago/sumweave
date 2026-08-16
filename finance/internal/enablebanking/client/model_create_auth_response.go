package client

// CreateAuthResponse models an auth response.
type CreateAuthResponse struct {
	URL             string `json:"url"`
	AuthorizationID string `json:"authorization_id"`
	PSUIDHash       string `json:"psu_id_hash"`
}
