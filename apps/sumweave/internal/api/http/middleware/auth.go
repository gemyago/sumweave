package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/httpapi"
)

// jwtValidator validates JWT access tokens and returns the parsed claims.
type jwtValidator interface {
	ValidateAccessToken(tokenStr string) (*auth.JWTClaims, error)
}

type accessTokenValidator interface {
	Validate(context.Context, string) (*auth.ValidatedAccessToken, error)
}

// AuthMiddleware applies the default session-only policy to an HTTP handler.
type AuthMiddleware func(http.Handler) http.Handler

type CredentialPolicy string

const (
	SessionOnly CredentialPolicy = "session-only"
	TokenRead   CredentialPolicy = "token-read"
	TokenWrite  CredentialPolicy = "token-write"
)

// AuthMiddlewareDeps holds the dependencies for NewCredentialMiddleware.
type AuthMiddlewareDeps struct {
	JWTValidator         jwtValidator
	AccessTokenValidator accessTokenValidator
	Logger               *slog.Logger
}

// CredentialMiddleware validates bearer credentials and applies explicit policies.
type CredentialMiddleware struct {
	jwtValidator         jwtValidator
	accessTokenValidator accessTokenValidator
	logger               *slog.Logger
}

// runtimeCallerIdentity exposes only a caller user identity to the runtime module.
type runtimeCallerIdentity struct{ userID string }

func (i runtimeCallerIdentity) UserID() string { return i.userID }

// NewCredentialMiddleware constructs credential validation with required dependencies.
func NewCredentialMiddleware(deps AuthMiddlewareDeps) (*CredentialMiddleware, error) {
	if deps.JWTValidator == nil {
		return nil, errors.New("credential middleware JWT validator is required")
	}
	if deps.AccessTokenValidator == nil {
		return nil, errors.New("credential middleware access token validator is required")
	}
	if deps.Logger == nil {
		return nil, errors.New("credential middleware logger is required")
	}
	return &CredentialMiddleware{
		jwtValidator: deps.JWTValidator, accessTokenValidator: deps.AccessTokenValidator,
		logger: deps.Logger.WithGroup("credential-middleware"),
	}, nil
}

// SessionOnly returns the compatibility middleware for protected session-only routes.
func (m *CredentialMiddleware) SessionOnly() AuthMiddleware {
	return func(next http.Handler) http.Handler { return m.Require(SessionOnly, next) }
}

// Require authenticates a bearer credential and enforces the supplied policy.
func (m *CredentialMiddleware) Require(policy CredentialPolicy, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			WriteError(w, r, APIErrorUnauthorized())
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			WriteError(w, r, APIErrorUnauthorized())
			return
		}

		tokenStr := parts[1]
		credentialKind := credentialKindForToken(tokenStr)
		caller, err := m.authenticate(r.Context(), tokenStr)
		if err != nil {
			if credentialKind == auth.CredentialKindAccessToken && !errors.Is(err, auth.ErrInvalidAccessToken) {
				m.logger.ErrorContext(
					r.Context(),
					"access token validation failed",
					telemetry.ErrAttr(err),
					slog.String("credentialKind", string(credentialKind)),
				)
				WriteError(w, r, apiErrorInternal())
				return
			}
			m.logger.DebugContext(
				r.Context(),
				"credential validation failed",
				slog.String("credentialKind", string(credentialKind)),
			)
			WriteError(w, r, APIErrorUnauthorized())
			return
		}
		if !allows(policy, caller) {
			WriteError(w, r, APIErrorInsufficientPermission())
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithCaller(r.Context(), caller)))
	})
}

// RuntimeIdentity adapts the application caller to the runtime's user-only identity.
func (m *CredentialMiddleware) RuntimeIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, ok := auth.CallerFromContext(r.Context())
		if !ok {
			WriteError(w, r, APIErrorUnauthorized())
			return
		}
		ctx := httpapi.ContextWithCallerIdentity(r.Context(), runtimeCallerIdentity{userID: caller.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *CredentialMiddleware) authenticate(ctx context.Context, value string) (auth.Caller, error) {
	if credentialKindForToken(value) == auth.CredentialKindAccessToken {
		validated, err := m.accessTokenValidator.Validate(ctx, value)
		if err != nil {
			return auth.Caller{}, err
		}
		return auth.Caller{
			UserID: validated.UserID, Credential: auth.CredentialKindAccessToken,
			AccessToken: &auth.AccessTokenCaller{
				TokenID:    validated.ID,
				TokenName:  validated.Name,
				Permission: validated.Permission,
				ExpiresAt:  validated.ExpiresAt,
				Status:     validated.Status,
				CreatedAt:  validated.CreatedAt,
				UpdatedAt:  validated.UpdatedAt,
			},
		}, nil
	}
	claims, err := m.jwtValidator.ValidateAccessToken(value)
	if err != nil {
		return auth.Caller{}, err
	}
	return auth.Caller{UserID: claims.Subject, Credential: auth.CredentialKindSession}, nil
}

func credentialKindForToken(value string) auth.CredentialKind {
	if strings.HasPrefix(value, "swat_") {
		return auth.CredentialKindAccessToken
	}
	return auth.CredentialKindSession
}

func allows(policy CredentialPolicy, caller auth.Caller) bool {
	if caller.Credential == auth.CredentialKindSession {
		return true
	}
	if caller.AccessToken == nil {
		return false
	}
	switch policy {
	case SessionOnly:
		return false
	case TokenRead:
		return caller.AccessToken.Permission == auth.AccessTokenPermissionReadOnly ||
			caller.AccessToken.Permission == auth.AccessTokenPermissionReadWrite
	case TokenWrite:
		return caller.AccessToken.Permission == auth.AccessTokenPermissionReadWrite
	default:
		return false
	}
}
