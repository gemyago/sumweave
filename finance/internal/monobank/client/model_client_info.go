package client

// Info models /personal/client-info.
type Info struct {
	Name        string        `json:"name"`
	WebHookURL  string        `json:"webHookUrl,omitempty"`
	Permissions string        `json:"permissions,omitempty"`
	Accounts    []InfoAccount `json:"accounts"`
}
