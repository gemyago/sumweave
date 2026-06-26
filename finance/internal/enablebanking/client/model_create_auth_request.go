package client

// CreateAuthRequest models an auth request payload.
type CreateAuthRequest struct {
	Access      CreateAuthAccess `json:"access"`
	ASPSP       CreateAuthASPSP  `json:"aspsp"`
	State       string           `json:"state,omitempty"`
	RedirectURL string           `json:"redirect_url,omitempty"`
	PSUType     string           `json:"psu_type,omitempty"`
}

// CreateAuthAccess models auth access settings.
type CreateAuthAccess struct {
	ValidUntil string `json:"valid_until,omitempty"`
}

// CreateAuthASPSP models auth ASPSP settings.
type CreateAuthASPSP struct {
	Name    string `json:"name,omitempty"`
	Country string `json:"country,omitempty"`
}
