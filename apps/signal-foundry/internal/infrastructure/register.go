package infrastructure

import (
	"context"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/di"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/infrastructure/httpclient"
	"go.uber.org/dig"
)

func Register(_ context.Context, container *dig.Container) error {
	return di.ProvideAll(container,
		httpclient.NewClientFactory,
	)
}
