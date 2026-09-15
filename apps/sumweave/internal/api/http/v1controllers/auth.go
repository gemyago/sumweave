package v1controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/handlers"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/models"
	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
)

// AuthenticatingService is the auth dependency for AuthController.
type AuthenticatingService interface {
	Login(ctx context.Context, username, password string) (*auth.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (*auth.RefreshResult, error)
	CurrentUser(ctx context.Context, userID string) (*auth.UserInfo, error)
}

// accessTokenLifecycleService owns the narrow self-service token lifecycle.
type accessTokenLifecycleService interface {
	Create(context.Context, auth.CreateAccessTokenParams) (*auth.IssuedAccessToken, error)
	List(context.Context, string) ([]auth.AccessTokenMetadata, error)
	Revoke(context.Context, string, string) error
	Rotate(context.Context, auth.RotateAccessTokenRequest) (*auth.IssuedAccessToken, error)
}

var _ AuthenticatingService = (*auth.AuthService)(nil)

// AuthControllerDeps holds the dependencies for AuthController.
type AuthControllerDeps struct {
	AuthService         AuthenticatingService
	AuthMiddleware      middleware.AuthMiddleware
	TokenReadMiddleware middleware.AuthMiddleware
	AccessTokens        accessTokenLifecycleService
}

// AuthController handles authentication HTTP routes (apigen-generated contract).
type AuthController struct {
	deps AuthControllerDeps
}

// NewAuthController creates a new AuthController.
func NewAuthController(deps AuthControllerDeps) *AuthController {
	return &AuthController{deps: deps}
}

var _ handlers.AuthController = (*AuthController)(nil)

// AuthLogin implements handlers.AuthController.
func (c *AuthController) AuthLogin(
	builder handlers.HandlerBuilder[*models.AuthLoginParams, *models.AuthSessionResponse],
) http.Handler {
	return builder.HandleWith(func(
		ctx context.Context,
		params *models.AuthLoginParams,
	) (*models.AuthSessionResponse, error) {
		result, err := c.deps.AuthService.Login(ctx, params.Payload.Username, params.Payload.Password)
		if err != nil {
			return nil, fmt.Errorf("login: %w", err)
		}

		user := models.UserInfo{ID: result.User.ID, Username: result.User.Username}
		return &models.AuthSessionResponse{
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
			User:         &user,
		}, nil
	})
}

// AuthRefresh implements handlers.AuthController.
func (c *AuthController) AuthRefresh(
	builder handlers.HandlerBuilder[*models.AuthRefreshParams, *models.AuthSessionResponse],
) http.Handler {
	return builder.HandleWith(func(
		ctx context.Context,
		params *models.AuthRefreshParams,
	) (*models.AuthSessionResponse, error) {
		result, err := c.deps.AuthService.Refresh(ctx, params.Payload.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("refresh: %w", err)
		}

		user := models.UserInfo{ID: result.User.ID, Username: result.User.Username}
		return &models.AuthSessionResponse{
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
			User:         &user,
		}, nil
	})
}

// AuthMe implements handlers.AuthController.
func (c *AuthController) AuthMe(
	builder handlers.NoParamsHandlerBuilder[*models.UserInfo],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context) (*models.UserInfo, error) {
		caller, ok := auth.CallerFromContext(ctx)
		if !ok {
			return nil, app.NewErrUnauthorized("unauthorized")
		}

		userInfo, err := c.deps.AuthService.CurrentUser(ctx, caller.UserID)
		if err != nil {
			return nil, fmt.Errorf("get current user: %w", err)
		}

		response := &models.UserInfo{ID: userInfo.ID, Username: userInfo.Username}
		if caller.AccessToken != nil {
			response.AccessToken = tokenMetadataModel(auth.AccessTokenMetadata{
				ID: caller.AccessToken.TokenID, Name: caller.AccessToken.TokenName,
				Hint:       accessTokenHint(caller.AccessToken.TokenID),
				Permission: caller.AccessToken.Permission, ExpiresAt: caller.AccessToken.ExpiresAt,
				Status: caller.AccessToken.Status, CreatedAt: caller.AccessToken.CreatedAt,
				UpdatedAt: caller.AccessToken.UpdatedAt,
			})
		}
		return response, nil
	})
	if c.deps.TokenReadMiddleware == nil {
		return c.deps.AuthMiddleware(inner)
	}
	return c.deps.TokenReadMiddleware(inner)
}

// CreateAuthAccessToken implements handlers.AuthController.
func (c *AuthController) CreateAuthAccessToken(
	builder handlers.HandlerBuilder[*models.CreateAuthAccessTokenParams, *models.AccessTokenIssuedResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.CreateAuthAccessTokenParams,
	) (*models.AccessTokenIssuedResponse, error) {
		caller, err := sessionCaller(ctx)
		if err != nil {
			return nil, err
		}
		issued, err := c.deps.AccessTokens.Create(ctx, auth.CreateAccessTokenParams{
			UserID: caller.UserID, Name: params.Payload.Name,
			Permission: auth.AccessTokenPermission(params.Payload.Permission), ExpiresAt: params.Payload.ExpiresAt,
		})
		if err != nil {
			return nil, lifecycleError(err, "")
		}
		return issuedTokenResponse(*issued), nil
	})
	return c.deps.AuthMiddleware(inner)
}

// ListAuthAccessTokens implements handlers.AuthController.
func (c *AuthController) ListAuthAccessTokens(
	builder handlers.NoParamsHandlerBuilder[*models.AccessTokenListResponse],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context) (*models.AccessTokenListResponse, error) {
		caller, err := sessionCaller(ctx)
		if err != nil {
			return nil, err
		}
		items, err := c.deps.AccessTokens.List(ctx, caller.UserID)
		if err != nil {
			return nil, lifecycleError(err, "")
		}
		response := &models.AccessTokenListResponse{Items: make([]*models.AccessTokenMetadata, 0, len(items))}
		for _, item := range items {
			response.Items = append(response.Items, tokenMetadataModel(item))
		}
		return response, nil
	})
	return c.deps.AuthMiddleware(inner)
}

// RevokeAuthAccessToken implements handlers.AuthController.
func (c *AuthController) RevokeAuthAccessToken(
	builder handlers.NoResponseHandlerBuilder[*models.RevokeAuthAccessTokenParams],
) http.Handler {
	inner := builder.HandleWith(func(ctx context.Context, params *models.RevokeAuthAccessTokenParams) error {
		caller, err := sessionCaller(ctx)
		if err != nil {
			return err
		}
		if revokeErr := c.deps.AccessTokens.Revoke(ctx, caller.UserID, params.TokenID); revokeErr != nil {
			return lifecycleError(revokeErr, params.TokenID)
		}
		return nil
	})
	return c.deps.AuthMiddleware(inner)
}

// RotateAuthAccessToken implements handlers.AuthController.
func (c *AuthController) RotateAuthAccessToken(
	builder handlers.HandlerBuilder[*models.RotateAuthAccessTokenParams, *models.AccessTokenIssuedResponse],
) http.Handler {
	inner := builder.HandleWith(func(
		ctx context.Context,
		params *models.RotateAuthAccessTokenParams,
	) (*models.AccessTokenIssuedResponse, error) {
		caller, err := sessionCaller(ctx)
		if err != nil {
			return nil, err
		}
		issued, err := c.deps.AccessTokens.Rotate(ctx, auth.RotateAccessTokenRequest{
			UserID: caller.UserID, TokenID: params.TokenID, ExpiresAt: params.Payload.ExpiresAt,
		})
		if err != nil {
			return nil, lifecycleError(err, params.TokenID)
		}
		return issuedTokenResponse(*issued), nil
	})
	return c.deps.AuthMiddleware(requireRotateExpiresAt(inner))
}

// requireRotateExpiresAt preserves the generated request boundary while
// enforcing OpenAPI's required-but-nullable expiresAt property. Apigen's
// generated pointer model otherwise represents both omission and JSON null as
// nil. The request body is restored for the generated parser after inspection.
func requireRotateExpiresAt(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalBody := r.Body
		defer originalBody.Close()

		body, err := io.ReadAll(originalBody)
		if err != nil {
			middleware.WriteError(w, r, middleware.APIErrorInvalidRequest())
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		var fields map[string]json.RawMessage
		if json.Unmarshal(body, &fields) == nil {
			if _, ok := fields["expiresAt"]; !ok {
				middleware.WriteError(w, r, middleware.APIErrorInvalidRequest())
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func sessionCaller(ctx context.Context) (auth.Caller, error) {
	caller, ok := auth.CallerFromContext(ctx)
	if !ok {
		return auth.Caller{}, app.NewErrUnauthorized("unauthorized")
	}
	return caller, nil
}

func lifecycleError(err error, tokenID string) error {
	switch {
	case errors.Is(err, auth.ErrInvalidAccessTokenInput):
		return app.NewErrInvalidInput("accessToken", "invalid")
	case errors.Is(err, auth.ErrAccessTokenNotFound):
		return app.NewErrNotFound("accessToken", tokenID)
	case errors.Is(err, auth.ErrAccessTokenConflict),
		errors.Is(err, auth.ErrAccessTokenInactive),
		errors.Is(err, auth.ErrAccessTokenNameExists):
		return app.NewErrConflict("accessToken", "conflict")
	default:
		return fmt.Errorf("access token lifecycle: %w", err)
	}
}

func issuedTokenResponse(issued auth.IssuedAccessToken) *models.AccessTokenIssuedResponse {
	return &models.AccessTokenIssuedResponse{
		Token:    tokenMetadataModel(issued.AccessTokenMetadata),
		ApiToken: issued.APIToken,
	}
}

func tokenMetadataModel(metadata auth.AccessTokenMetadata) *models.AccessTokenMetadata {
	return &models.AccessTokenMetadata{
		ID: metadata.ID, Name: metadata.Name, Hint: metadata.Hint,
		Permission: models.AccessTokenMetadataPermission(metadata.Permission),
		Status:     models.AccessTokenMetadataStatus(metadata.Status), ExpiresAt: metadata.ExpiresAt,
		RevokedAt: metadata.RevokedAt, CreatedAt: metadata.CreatedAt, UpdatedAt: metadata.UpdatedAt,
	}
}

func accessTokenHint(tokenID string) string {
	if len(tokenID) < 8 {
		return "swat_"
	}
	return "swat_" + tokenID[:8] + "..."
}
