package client

// CreateSessionRequest models a session creation payload.
type CreateSessionRequest struct {
	Code              string `json:"code,omitempty"`
	State             string `json:"state,omitempty"`
	ProviderReference string `json:"providerReference,omitempty"`
}
