package infrastructure

import (
	"context"

	"github.com/gemyago/sumweave/apps/sumweave/internal/di"
	"github.com/gemyago/sumweave/apps/sumweave/internal/infrastructure/httpclient"
	"go.uber.org/dig"
)

func Register(_ context.Context, container *dig.Container) error {
	return di.ProvideAll(container,
		httpclient.NewClientFactory,
	)
}
