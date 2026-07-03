package http

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	signalfoundryinternal "github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/middleware"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1controllers"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/handlers"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"go.uber.org/dig"
)

// Use apigen to generate v1routes.
// The follow-up patch keeps generated validators buildable for required map fields
// until the upstream generator emits a map-safe EnsureNonDefault helper.
//go:generate sh -c "go run github.com/gemyago/apigen server ./v1routes.yaml ./v1routes && go run ./apigenpatch"

type V1RoutesDeps struct {
	dig.In

	*v1controllers.HealthController
	*v1controllers.AuthController
	*v1controllers.DataController
	*v1controllers.JobsController
	*v1controllers.FinanceController
	*v1controllers.StrategiesController
	*v1controllers.EvaluationsController

	RootHandler *handlers.RootHandler

	HTTPRouter     *server.HTTPRouter
	AuthMiddleware middleware.AuthMiddleware

	Runtime               *signalfoundryinternal.Runtime
	RootLogger            *slog.Logger
	FinanceService        *financepkg.Service
	BankConnectionService *financepkg.BankConnectionService

	// UILocation is an optional path to the directory containing pre-built UI static assets.
	// When set and the directory is readable, the backend serves index.html at GET /
	// and static assets from that directory. When empty or invalid, the server runs in API-only mode.
	UILocation string `name:"config.httpServer.uiLocation" optional:"true"`
}

func SetupV1Routes(deps V1RoutesDeps) { // coverage-ignore // Little value in testing wireup code.
	rootHandler := deps.RootHandler
	rootHandler.RegisterHealthRoutes(deps.HealthController)
	rootHandler.RegisterAuthRoutes(deps.AuthController)
	rootHandler.RegisterDataRoutes(deps.DataController)
	rootHandler.RegisterJobsRoutes(deps.JobsController)
	rootHandler.RegisterFinanceRoutes(deps.FinanceController)
	rootHandler.RegisterStrategiesRoutes(deps.StrategiesController)
	rootHandler.RegisterEvaluationsRoutes(deps.EvaluationsController)

	// Runtime routes — protected
	deps.HTTPRouter.Handle(
		"/api/v1/runtime/",
		deps.AuthMiddleware(http.StripPrefix("/api/v1/runtime", deps.Runtime.HTTPHandler)),
	)
	deps.HTTPRouter.HandleRoute(
		http.MethodGet,
		enableBankingCallbackPath,
		newEnableBankingCallbackHandler(deps.BankConnectionService),
	)

	mountUIRoutes(deps.RootLogger, deps.HTTPRouter, deps.UILocation)
}

// mountUIRoutes mounts UI static file serving when uiLocation is a valid readable directory.
// If uiLocation is empty or the directory cannot be opened, the server continues in API-only mode.
func mountUIRoutes(logger *slog.Logger, router *server.HTTPRouter, uiLocation string) {
	if uiLocation == "" {
		return
	}
	if _, err := os.Stat(uiLocation); err != nil {
		logger.Warn("UI location is not accessible, running in API-only mode",
			slog.String("uiLocation", uiLocation),
			slog.Any("err", err),
		)
		return
	}
	fs := http.FileServerFS(os.DirFS(uiLocation))
	router.Handle("/", fs)
}

func Register(container *dig.Container) error {
	return errors.Join(
		v1controllers.Register(container),
		container.Invoke(SetupV1Routes),
	)
}
