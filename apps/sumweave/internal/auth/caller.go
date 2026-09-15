package auth

import (
	"context"
	"time"
)

type CredentialKind string

const (
	CredentialKindSession     CredentialKind = "session"
	CredentialKindAccessToken CredentialKind = "access-token"
)

// AccessTokenCaller is the safe token metadata associated with a request caller.
type AccessTokenCaller struct {
	TokenID    string
	TokenName  string
	Permission AccessTokenPermission
	ExpiresAt  *time.Time
}

// Caller is the authenticated application caller for one request.
type Caller struct {
	UserID      string
	Credential  CredentialKind
	AccessToken *AccessTokenCaller
}

type callerContextKey struct{}

// ContextWithCaller records the authenticated caller for an application request.
func ContextWithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller)
}

// CallerFromContext returns the authenticated application caller.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	caller, ok := ctx.Value(callerContextKey{}).(Caller)
	return caller, ok
}
