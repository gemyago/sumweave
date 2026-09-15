package v1controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/models"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newAuthHTTPHandler mounts auth routes the same way as apigen RegisterAuthRoutes.
func newAuthHTTPHandler(ctrl *AuthController) http.Handler {
	return server.NewTestRootHandler().RegisterAuthRoutes(ctrl)
}

func TestAuthController(t *testing.T) {
	fake := faker.New()

	passthroughAuthMiddleware := middleware.AuthMiddleware(func(next http.Handler) http.Handler {
		return next
	})

	newControllerWithTokens := func(svc AuthenticatingService, tokens accessTokenLifecycleService, authMiddleware middleware.AuthMiddleware) *AuthController {
		return NewAuthController(AuthControllerDeps{
			AuthService:         svc,
			AuthMiddleware:      authMiddleware,
			TokenReadMiddleware: passthroughAuthMiddleware,
			AccessTokens:        tokens,
		})
	}
	newController := func(svc AuthenticatingService) *AuthController {
		return newControllerWithTokens(svc, newMockaccessTokenLifecycleService(t), passthroughAuthMiddleware)
	}

	t.Run("Login", func(t *testing.T) {
		t.Run("valid credentials - 200 with tokens and user", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			username := fake.Internet().User()
			password := fake.Internet().Password()
			userID := fake.UUID().V4()
			accessToken := fake.Lorem().Word()
			refreshToken := fake.Lorem().Word()

			svc.EXPECT().Login(mock.Anything, username, password).Return(&auth.LoginResult{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
				User:         auth.UserInfo{ID: userID, Username: username},
			}, nil)

			body, err := json.Marshal(map[string]string{"username": username, "password": password})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp models.AuthSessionResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, accessToken, resp.AccessToken)
			assert.Equal(t, refreshToken, resp.RefreshToken)
			require.NotNil(t, resp.User)
			assert.Equal(t, userID, resp.User.ID)
			assert.Equal(t, username, resp.User.Username)
		})

		t.Run("invalid credentials - 401", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			username := fake.Internet().User()
			password := fake.Internet().Password()

			svc.EXPECT().Login(mock.Anything, username, password).Return(nil, auth.ErrInvalidCredentials)

			body, err := json.Marshal(map[string]string{"username": username, "password": password})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.NotEmpty(t, w.Body.String())
		})

		t.Run("missing username - 400", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			body, err := json.Marshal(map[string]string{"password": fake.Internet().Password()})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("missing password - 400", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			body, err := json.Marshal(map[string]string{"username": fake.Internet().User()})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("invalid JSON body - 400", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("not-json")))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("service error - 500", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			username := fake.Internet().User()
			password := fake.Internet().Password()

			svc.EXPECT().Login(mock.Anything, username, password).Return(nil, errors.New("unexpected error"))

			body, err := json.Marshal(map[string]string{"username": username, "password": password})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Refresh", func(t *testing.T) {
		t.Run("valid token - 200 with new tokens", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			oldRefreshToken := fake.Lorem().Word()
			newAccessToken := fake.Lorem().Word()
			newRefreshToken := fake.Lorem().Word()
			userID := fake.UUID().V4()
			username := fake.Internet().User()

			svc.EXPECT().Refresh(mock.Anything, oldRefreshToken).Return(&auth.RefreshResult{
				AccessToken:  newAccessToken,
				RefreshToken: newRefreshToken,
				User:         auth.UserInfo{ID: userID, Username: username},
			}, nil)

			body, err := json.Marshal(map[string]string{"refreshToken": oldRefreshToken})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp models.AuthSessionResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, newAccessToken, resp.AccessToken)
			assert.Equal(t, newRefreshToken, resp.RefreshToken)
			require.NotNil(t, resp.User)
			assert.Equal(t, userID, resp.User.ID)
			assert.Equal(t, username, resp.User.Username)
		})

		t.Run("invalid token - 401", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			refreshToken := fake.Lorem().Word()

			svc.EXPECT().Refresh(mock.Anything, refreshToken).Return(nil, auth.ErrInvalidRefreshToken)

			body, err := json.Marshal(map[string]string{"refreshToken": refreshToken})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.NotEmpty(t, w.Body.String())
		})

		t.Run("missing refreshToken - 400", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			body, err := json.Marshal(map[string]string{})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("invalid JSON body - 400", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte("not-json")))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("service error - 500", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			refreshToken := fake.Lorem().Word()

			svc.EXPECT().Refresh(mock.Anything, refreshToken).Return(nil, errors.New("unexpected error"))

			body, err := json.Marshal(map[string]string{"refreshToken": refreshToken})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Me", func(t *testing.T) {
		t.Run("access token caller returns safe token metadata", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			userID := fake.UUID().V4()
			tokenID := fake.UUID().V4()
			now := time.Now()
			svc.EXPECT().CurrentUser(mock.Anything, userID).Return(&auth.UserInfo{
				ID: userID, Username: fake.Internet().User(),
			}, nil)
			ctrl := newController(svc)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
			request = request.WithContext(auth.ContextWithCaller(
				t.Context(),
				auth.Caller{
					UserID: userID, Credential: auth.CredentialKindAccessToken,
					AccessToken: &auth.AccessTokenCaller{
						TokenID: tokenID, TokenName: fake.Lorem().Word(),
						Permission: auth.AccessTokenPermissionReadOnly,
						Status:     auth.AccessTokenStatusActive,
						CreatedAt:  now,
						UpdatedAt:  now,
					},
				},
			))
			response := httptest.NewRecorder()
			newAuthHTTPHandler(ctrl).ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			var body models.UserInfo
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.NotNil(t, body.AccessToken)
			assert.Equal(t, "swat_"+tokenID[:8]+"...", body.AccessToken.Hint)
		})
		t.Run("with CallerIdentity - 200 with user info", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			userID := fake.UUID().V4()
			username := fake.Internet().User()

			svc.EXPECT().CurrentUser(mock.Anything, userID).Return(&auth.UserInfo{
				ID:       userID,
				Username: username,
			}, nil)

			ctx := auth.ContextWithCaller(
				httpapi.ContextWithCallerIdentity(t.Context(), &testCallerIdentity{userID: userID}),
				auth.Caller{UserID: userID, Credential: auth.CredentialKindSession},
			)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp models.UserInfo
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, userID, resp.ID)
			assert.Equal(t, username, resp.Username)
		})

		t.Run("without CallerIdentity - 401", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
			req = req.WithContext(t.Context())
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("user not found - 401", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			userID := fake.UUID().V4()

			svc.EXPECT().CurrentUser(mock.Anything, userID).Return(nil, auth.ErrUserNotFound)

			ctx := auth.ContextWithCaller(
				httpapi.ContextWithCallerIdentity(t.Context(), &testCallerIdentity{userID: userID}),
				auth.Caller{UserID: userID, Credential: auth.CredentialKindSession},
			)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("service error - 500", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			ctrl := newController(svc)

			userID := fake.UUID().V4()

			svc.EXPECT().CurrentUser(mock.Anything, userID).Return(nil, errors.New("unexpected error"))

			ctx := auth.ContextWithCaller(
				httpapi.ContextWithCallerIdentity(t.Context(), &testCallerIdentity{userID: userID}),
				auth.Caller{UserID: userID, Credential: auth.CredentialKindSession},
			)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", http.NoBody)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			newAuthHTTPHandler(ctrl).ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Access tokens", func(t *testing.T) {
		makeMetadata := func() auth.AccessTokenMetadata {
			return auth.AccessTokenMetadata{
				ID: fake.UUID().V4(), Name: fake.Lorem().Word(), Hint: "swat_018f...",
				Permission: auth.AccessTokenPermissionReadOnly, Status: auth.AccessTokenStatusActive,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
		}
		withSessionCaller := func(request *http.Request, userID string) *http.Request {
			return request.WithContext(auth.ContextWithCaller(
				t.Context(),
				auth.Caller{UserID: userID, Credential: auth.CredentialKindSession},
			))
		}

		t.Run("create returns 201 with camel case one-time value", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			tokens := newMockaccessTokenLifecycleService(t)
			userID := fake.UUID().V4()
			metadata := makeMetadata()
			apiToken := fake.Internet().Password()
			tokens.EXPECT().Create(mock.Anything, mock.MatchedBy(func(params auth.CreateAccessTokenParams) bool {
				return params.UserID == userID &&
					params.Name == metadata.Name &&
					params.Permission == metadata.Permission
			})).Return(&auth.IssuedAccessToken{AccessTokenMetadata: metadata, APIToken: apiToken}, nil)
			body, err := json.Marshal(map[string]string{
				"name":       metadata.Name,
				"permission": string(metadata.Permission),
			})
			require.NoError(t, err)
			request := withSessionCaller(
				httptest.NewRequest(http.MethodPost, "/api/v1/auth/access-tokens", bytes.NewReader(body)),
				userID,
			)
			response := httptest.NewRecorder()
			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(response, request)
			require.Equal(t, http.StatusCreated, response.Code)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			assert.Equal(t, apiToken, payload["apiToken"])
			assert.NotContains(t, payload, "userId")
			tokenPayload := payload["token"].(map[string]any)
			assert.Equal(t, metadata.Hint, tokenPayload["hint"])
			assert.NotContains(t, tokenPayload, "secretHash")
		})

		t.Run("list returns metadata only", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			tokens := newMockaccessTokenLifecycleService(t)
			userID := fake.UUID().V4()
			metadata := makeMetadata()
			tokens.EXPECT().List(mock.Anything, userID).Return([]auth.AccessTokenMetadata{metadata}, nil)
			response := httptest.NewRecorder()
			request := withSessionCaller(
				httptest.NewRequest(http.MethodGet, "/api/v1/auth/access-tokens", http.NoBody),
				userID,
			)
			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			assert.NotContains(t, response.Body.String(), "apiToken")
			assert.NotContains(t, response.Body.String(), "secretHash")
		})

		t.Run("missing caller and invalid create input return safe errors", func(t *testing.T) {
			listResponse := httptest.NewRecorder()
			newAuthHTTPHandler(newControllerWithTokens(
				NewMockAuthenticatingService(t), newMockaccessTokenLifecycleService(t), passthroughAuthMiddleware,
			)).ServeHTTP(
				listResponse,
				httptest.NewRequest(http.MethodGet, "/api/v1/auth/access-tokens", http.NoBody).WithContext(t.Context()),
			)
			require.Equal(t, http.StatusUnauthorized, listResponse.Code)

			svc := NewMockAuthenticatingService(t)
			tokens := newMockaccessTokenLifecycleService(t)
			userID := fake.UUID().V4()
			metadata := makeMetadata()
			tokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, auth.ErrInvalidAccessTokenInput)
			body, err := json.Marshal(map[string]string{
				"name":       metadata.Name,
				"permission": string(metadata.Permission),
			})
			require.NoError(t, err)
			createResponse := httptest.NewRecorder()
			createRequest := withSessionCaller(
				httptest.NewRequest(http.MethodPost, "/api/v1/auth/access-tokens", bytes.NewReader(body)),
				userID,
			)
			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(createResponse, createRequest)
			require.Equal(t, http.StatusBadRequest, createResponse.Code)
		})

		t.Run("rotate and revoke use exact statuses", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			tokens := newMockaccessTokenLifecycleService(t)
			userID := fake.UUID().V4()
			metadata := makeMetadata()
			tokens.EXPECT().Rotate(mock.Anything, auth.RotateAccessTokenRequest{
				UserID: userID, TokenID: metadata.ID, ExpiresAt: nil,
			}).Return(&auth.IssuedAccessToken{
				AccessTokenMetadata: metadata, APIToken: fake.Internet().Password(),
			}, nil)
			rotateResponse := httptest.NewRecorder()
			rotatePath := "/api/v1/auth/access-tokens/" + metadata.ID + "/rotate"
			rotateRequest := withSessionCaller(
				httptest.NewRequest(http.MethodPost, rotatePath, bytes.NewBufferString(`{"expiresAt":null}`)),
				userID,
			)
			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(rotateResponse, rotateRequest)
			require.Equal(t, http.StatusOK, rotateResponse.Code)

			tokens.EXPECT().Revoke(mock.Anything, userID, metadata.ID).Return(nil)
			revokeResponse := httptest.NewRecorder()
			revokeRequest := withSessionCaller(
				httptest.NewRequest(http.MethodDelete, "/api/v1/auth/access-tokens/"+metadata.ID, http.NoBody),
				userID,
			)
			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(revokeResponse, revokeRequest)
			require.Equal(t, http.StatusNoContent, revokeResponse.Code)
		})

		t.Run("rotate rejects an omitted nullable expiry", func(t *testing.T) {
			svc := NewMockAuthenticatingService(t)
			tokens := newMockaccessTokenLifecycleService(t)
			userID := fake.UUID().V4()
			tokenID := fake.UUID().V4()
			rotatePath := "/api/v1/auth/access-tokens/" + tokenID + "/rotate"
			response := httptest.NewRecorder()
			request := withSessionCaller(
				httptest.NewRequest(http.MethodPost, rotatePath, bytes.NewBufferString(`{}`)),
				userID,
			)

			newAuthHTTPHandler(newControllerWithTokens(
				svc, tokens, passthroughAuthMiddleware,
			)).ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			assert.JSONEq(
				t,
				`{"code":"invalid_request","message":"The request is invalid.","correlationId":""}`,
				response.Body.String(),
			)
		})
	})
	t.Run("lifecycle error mapping", func(t *testing.T) {
		for _, lifecycleErr := range []error{
			auth.ErrInvalidAccessTokenInput,
			auth.ErrAccessTokenNotFound,
			auth.ErrAccessTokenConflict,
			errors.New(fake.Lorem().Sentence(2)),
		} {
			require.Error(t, lifecycleError(lifecycleErr, fake.UUID().V4()))
		}
		assert.Equal(t, "swat_", accessTokenHint(fake.Letter()))
	})
}

// testCallerIdentity is a simple CallerIdentity implementation for testing.
type testCallerIdentity struct {
	userID string
}

func (c *testCallerIdentity) UserID() string { return c.userID }
