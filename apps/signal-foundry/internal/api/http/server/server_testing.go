//go:build !release

package server

import (
	"net/http"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
)

func NewTestRootHandler() *handlers.RootHandler {
	return NewRootHandler(RootHandlerDeps{
		RootLogger: telemetry.RootTestLogger(),
		Router: NewHTTPRouter(HTTPRouterDeps{
			Middleware: func(h http.Handler) http.Handler {
				return h
			},
		}),
	})
}
