package v1controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/stretchr/testify/require"
)

func TestAuthMeTokenReadPolicy(t *testing.T) {
	makeMiddleware := func(allowed bool) middleware.AuthMiddleware {
		return func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if allowed {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusForbidden)
			})
		}
	}
	makeHandler := func(defaultAllowed, tokenReadAllowed bool) http.Handler {
		return server.NewTestRootHandler().RegisterAuthRoutes(NewAuthController(AuthControllerDeps{
			AuthMiddleware:      makeMiddleware(defaultAllowed),
			TokenReadMiddleware: makeMiddleware(tokenReadAllowed),
		}))
	}

	for _, testCase := range []struct {
		name             string
		defaultAllowed   bool
		tokenReadAllowed bool
	}{
		{name: "session", defaultAllowed: true, tokenReadAllowed: true},
		{name: "read-only token", tokenReadAllowed: true},
		{name: "read-write token", tokenReadAllowed: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			makeHandler(testCase.defaultAllowed, testCase.tokenReadAllowed).ServeHTTP(
				response,
				httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil),
			)
			require.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}
