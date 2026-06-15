package v1controllers

import (
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/runtime/data"
	"go.uber.org/dig"
)

func Register(container *dig.Container) error {
	return di.ProvideAll(container,
		di.ProvideValue(&HealthController{}),
		di.ProvideImplementation[*auth.AuthService, AuthenticatingService],
		di.ProvideImplementation[*data.ReadService, replayReadService],
		di.ProvideImplementation[*data.LineageService, lineageBrowserService],
		NewAuthController,
		NewDataController,
	)
}
