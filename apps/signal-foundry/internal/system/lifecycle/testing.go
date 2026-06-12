//go:build !release

package lifecycle

import (
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
)

const defaultTestShutdownTimeout = 30 * time.Second

// NewTestShutdownHooks constructor for shutdown Hooks that can be used in tests.
func NewTestShutdownHooks() *ShutdownHooks {
	return NewShutdownHooks(ShutdownHooksDeps{
		RootLogger:              telemetry.RootTestLogger(),
		GracefulShutdownTimeout: defaultTestShutdownTimeout,
	})
}
