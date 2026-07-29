package app

import (
	"github.com/gemyago/sumweave/apps/sumweave/internal/di"
	"go.uber.org/dig"
)

func Register(container *dig.Container) error {
	return di.ProvideAll(
		container,
		NewUserDirectory,
	)
}
