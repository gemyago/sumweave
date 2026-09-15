package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/gemyago/sumweave/runtime/httpapi"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCredentialMiddleware(t *testing.T) {
	fake := faker.New()
	makeMiddleware := func(t *testing.T, loggers ...*slog.Logger) (*CredentialMiddleware, *mockjwtValidator, *mockaccessTokenValidator) {
		t.Helper()
		jwt := newMockjwtValidator(t)
		access := newMockaccessTokenValidator(t)
		logger := telemetry.RootTestLogger()
		if len(loggers) > 0 {
			logger = loggers[0]
		}
		middleware, err := NewCredentialMiddleware(AuthMiddlewareDeps{
			JWTValidator: jwt, AccessTokenValidator: access, Logger: logger,
		})
		require.NoError(t, err)
		return middleware, jwt, access
	}
	newRequest := func(value string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(t.Context())
		if value != "" {
			req.Header.Set("Authorization", "Bearer "+value)
		}
		return req
	}

	t.Run("valid session sets only the application caller identity", func(t *testing.T) {
		middleware, jwt, _ := makeMiddleware(t)
		userID := fake.UUID().V4()
		value := fake.Lorem().Word()
		claims := &auth.JWTClaims{}
		claims.Subject = userID
		jwt.EXPECT().ValidateAccessToken(value).Return(claims, nil)

		response := httptest.NewRecorder()
		middleware.Require(SessionOnly, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			caller, ok := auth.CallerFromContext(req.Context())
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, auth.Caller{UserID: userID, Credential: auth.CredentialKindSession}, caller)
			assert.Nil(t, httpapi.CallerIdentityFromContext(req.Context()))
		})).ServeHTTP(response, newRequest(value))
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("runtime adapter exposes only the authenticated user identity", func(t *testing.T) {
		middleware, jwt, _ := makeMiddleware(t)
		userID := fake.UUID().V4()
		value := fake.Lorem().Word()
		claims := &auth.JWTClaims{}
		claims.Subject = userID
		jwt.EXPECT().ValidateAccessToken(value).Return(claims, nil)

		response := httptest.NewRecorder()
		runtimeHandler := middleware.RuntimeIdentity(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			identity := httpapi.CallerIdentityFromContext(req.Context())
			if assert.NotNil(t, identity) {
				assert.Equal(t, userID, identity.UserID())
			}
		}))
		middleware.Require(SessionOnly, runtimeHandler).ServeHTTP(response, newRequest(value))
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("access tokens select only access-token validation and enforce policy", func(t *testing.T) {
		for _, testCase := range []struct {
			name       string
			permission auth.AccessTokenPermission
			policy     CredentialPolicy
			wantStatus int
		}{
			{name: "read-only reads", permission: auth.AccessTokenPermissionReadOnly, policy: TokenRead, wantStatus: http.StatusOK},
			{name: "read-write reads", permission: auth.AccessTokenPermissionReadWrite, policy: TokenRead, wantStatus: http.StatusOK},
			{name: "read-write writes", permission: auth.AccessTokenPermissionReadWrite, policy: TokenWrite, wantStatus: http.StatusOK},
			{name: "read-only writes", permission: auth.AccessTokenPermissionReadOnly, policy: TokenWrite, wantStatus: http.StatusForbidden},
			{name: "session-only", permission: auth.AccessTokenPermissionReadWrite, policy: SessionOnly, wantStatus: http.StatusForbidden},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				middleware, jwt, access := makeMiddleware(t)
				value := "swat_" + fake.Lorem().Word()
				expiresAt := time.Now().Add(time.Hour)
				validated := &auth.ValidatedAccessToken{
					UserID: fake.UUID().V4(),
					AccessTokenMetadata: auth.AccessTokenMetadata{
						ID:         fake.UUID().V4(),
						Name:       fake.Lorem().Word(),
						Permission: testCase.permission,
						ExpiresAt:  &expiresAt,
					},
				}
				access.EXPECT().Validate(mock.Anything, value).Return(validated, nil)
				called := false
				response := httptest.NewRecorder()
				middleware.Require(testCase.policy, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					called = true
					caller, ok := auth.CallerFromContext(req.Context())
					if !assert.True(t, ok) {
						return
					}
					if !assert.NotNil(t, caller.AccessToken) {
						return
					}
					assert.Equal(t, validated.ID, caller.AccessToken.TokenID)
				})).ServeHTTP(response, newRequest(value))
				assert.Equal(t, testCase.wantStatus, response.Code)
				assert.Equal(t, testCase.wantStatus == http.StatusOK, called)
				jwt.AssertNotCalled(t, "ValidateAccessToken", mock.Anything)
			})
		}
	})

	t.Run(
		"malformed headers and every invalid token outcome use the same safe unauthorized response",
		func(t *testing.T) {
			for _, testCase := range []struct {
				name  string
				value string
			}{
				{name: "missing header"},
				{name: "malformed header", value: "malformed"},
				{name: "malformed access token", value: "swat_" + fake.Lorem().Word()},
				{name: "unknown access token", value: "swat_" + fake.Lorem().Word()},
				{name: "mismatched access token", value: "swat_" + fake.Lorem().Word()},
				{name: "expired access token", value: "swat_" + fake.Lorem().Word()},
				{name: "revoked access token", value: "swat_" + fake.Lorem().Word()},
				{name: "invalid permission access token", value: "swat_" + fake.Lorem().Word()},
				{name: "invalid session", value: fake.Lorem().Word()},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					middleware, jwt, access := makeMiddleware(t)
					if testCase.value != "" && testCase.value != "malformed" {
						if credentialKindForToken(testCase.value) == auth.CredentialKindAccessToken {
							access.EXPECT().Validate(mock.Anything, testCase.value).Return(
								nil,
								auth.ErrInvalidAccessToken,
							)
						} else {
							jwt.EXPECT().ValidateAccessToken(testCase.value).Return(
								nil,
								errors.New(fake.Lorem().Sentence(3)),
							)
						}
					}
					req := newRequest(testCase.value)
					if testCase.value == "malformed" {
						req.Header.Set("Authorization", testCase.value)
					}
					response := httptest.NewRecorder()
					middleware.Require(TokenRead, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
						t.Fatal("next handler must not run")
					})).ServeHTTP(response, req)
					assert.Equal(t, http.StatusUnauthorized, response.Code)
					assert.JSONEq(
						t,
						`{"code":"unauthorized","message":"Authentication is required.","correlationId":""}`,
						response.Body.String(),
					)
				})
			}
		},
	)

	t.Run("wrapped access-token validation failures are logged and safely return internal errors", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := telemetry.NewRootLogger(
			telemetry.NewRootLoggerOpts().
				WithJSONLogs(true).
				WithOutput(&logBuffer).
				WithLogLevel(slog.LevelDebug),
		)
		middleware, jwt, access := makeMiddleware(t, logger)
		value := "swat_" + fake.Lorem().Word()
		storeErr := errors.New(fake.Lorem().Sentence(3))
		validationErr := fmt.Errorf("get presented access token: %w", storeErr)
		access.EXPECT().Validate(mock.Anything, value).Return(nil, validationErr).Once()
		correlationID := fake.UUID().V4()
		request := newRequest(value)
		request.Header.Set(telemetry.CorrelationIDHeader, correlationID)
		response := httptest.NewRecorder()
		handlerCalled := false

		handler := NewCorrelationMiddleware(ident.NewDefaultGenerator())(
			middleware.Require(TokenRead, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				handlerCalled = true
			})),
		)
		handler.ServeHTTP(response, request)

		assert.False(t, handlerCalled)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.JSONEq(
			t,
			`{"code":"internal_error","message":"An internal error occurred.","correlationId":"`+correlationID+`"}`,
			response.Body.String(),
		)
		assert.NotContains(t, response.Body.String(), storeErr.Error())
		assert.NotContains(t, response.Body.String(), validationErr.Error())
		assert.Equal(t, correlationID, response.Header().Get(telemetry.CorrelationIDHeader))
		jwt.AssertNotCalled(t, "ValidateAccessToken", mock.Anything)

		var entry struct {
			Level          string `json:"level"`
			Message        string `json:"msg"`
			Error          string `json:"err"`
			CorrelationID  string `json:"correlationId"`
			CredentialKind string `json:"credentialKind"`
		}
		require.NoError(t, json.Unmarshal(logBuffer.Bytes(), &entry))
		assert.Equal(t, "ERROR", entry.Level)
		assert.Equal(t, "access token validation failed", entry.Message)
		assert.Equal(t, validationErr.Error(), entry.Error)
		assert.Equal(t, correlationID, entry.CorrelationID)
		assert.Equal(t, string(auth.CredentialKindAccessToken), entry.CredentialKind)
	})

	t.Run("constructor requires all dependencies", func(t *testing.T) {
		middleware, jwt, access := makeMiddleware(t)
		assert.NotNil(t, middleware)
		for _, deps := range []AuthMiddlewareDeps{
			{AccessTokenValidator: access, Logger: telemetry.RootTestLogger()},
			{JWTValidator: jwt, Logger: telemetry.RootTestLogger()},
			{JWTValidator: jwt, AccessTokenValidator: access},
		} {
			_, err := NewCredentialMiddleware(deps)
			require.Error(t, err)
		}
	})
}
