package http

import (
	"errors"
	"log/slog"
	"net/http"

	sumweaveinternal "github.com/gemyago/sumweave/apps/sumweave/internal"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/middleware"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1controllers"
	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/v1routes/handlers"
	financepkg "github.com/gemyago/sumweave/finance"
	"go.uber.org/dig"
)

//go:generate go run github.com/gemyago/apigen server ./v1routes.yaml ./v1routes

type V1RoutesDeps struct {
	dig.In

	*v1controllers.HealthController
	*v1controllers.AuthController
	*v1controllers.JobsController
	*v1controllers.FinanceController

	RootHandler *handlers.RootHandler

	HTTPRouter     *server.HTTPRouter
	AuthMiddleware middleware.AuthMiddleware

	Runtime               *sumweaveinternal.Runtime
	RootLogger            *slog.Logger
	BankConnectionService *financepkg.BankConnectionService
}

func SetupV1Routes(deps V1RoutesDeps) { // coverage-ignore // Little value in testing wireup code.
	rootHandler := deps.RootHandler
	rootHandler.RegisterHealthRoutes(deps.HealthController)
	rootHandler.RegisterAuthRoutes(deps.AuthController)
	rootHandler.RegisterJobsRoutes(deps.JobsController)
	rootHandler.RegisterFinanceRoutes(deps.FinanceController)

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

	mountUIRoutes(deps.RootLogger, deps.HTTPRouter)
}

func Register(container *dig.Container) error {
	return errors.Join(
		v1controllers.Register(container),
		container.Invoke(SetupV1Routes),
	)
}
