package app

import (
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"go.uber.org/dig"
)

func Register(container *dig.Container) error {
	return di.ProvideAll(
		container,
		NewUserDirectory,
	)
}
