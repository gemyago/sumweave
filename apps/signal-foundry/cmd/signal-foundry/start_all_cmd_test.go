package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestStartAllRuntime(t *testing.T) {
	fake := faker.New()
	makeBlockingRuntime := func(t *testing.T, scheduler jobsSchedulerRunner) *startAllRuntime {
		t.Helper()
		server := newMockstartAllHTTPServer(t)
		server.EXPECT().
			StartHTTPServer(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, _ ...startHTTPServerOpt) error { <-ctx.Done(); return nil })
		worker := newMockjobsWorkerRunner(t)
		worker.EXPECT().
			Run(mock.Anything).
			RunAndReturn(func(ctx context.Context) error { <-ctx.Done(); return nil })
		return &startAllRuntime{
			logger:                slog.Default(),
			engine:                server,
			worker:                worker,
			scheduler:             scheduler,
			schedulerLoopInterval: time.Millisecond,
		}
	}
	t.Run("command delegates its resolver and preserves resolver errors", func(t *testing.T) {
		runner := newMockstartAllRunner(t)
		runner.EXPECT().Run(mock.Anything).Return(nil)
		cmd := newStartAllCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container, startServerParams) (startAllRunner, error) { return runner, nil },
		)
		require.NoError(t, cmd.ExecuteContext(t.Context()))
		want := errors.New(fake.Lorem().Sentence(3))
		cmd = newStartAllCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container, startServerParams) (startAllRunner, error) { return nil, want },
		)
		require.ErrorIs(t, cmd.ExecuteContext(t.Context()), want)
	})
	t.Run(
		"runtime starts application components, repeats scheduler ticks, and cancels cleanly",
		func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			scheduler := newMockjobsSchedulerRunner(t)
			scheduler.EXPECT().
				EnqueueDue(mock.Anything).
				RunAndReturn(func(context.Context) (int, error) { cancel(); return 1, nil }).
				Once()
			runtime := makeBlockingRuntime(t, scheduler)
			require.NoError(t, runtime.Run(ctx))
		},
	)
	t.Run(
		"runtime reports component failures and validates all required dependencies",
		func(t *testing.T) {
			missing := []startAllRuntime{
				{},
				{engine: newMockstartAllHTTPServer(t)},
				{engine: newMockstartAllHTTPServer(t), worker: newMockjobsWorkerRunner(t)},
				{
					engine:    newMockstartAllHTTPServer(t),
					worker:    newMockjobsWorkerRunner(t),
					scheduler: newMockjobsSchedulerRunner(t),
				},
			}
			for _, runtime := range missing {
				require.Error(t, runtime.validate())
			}
			server := newMockstartAllHTTPServer(t)
			server.EXPECT().
				StartHTTPServer(mock.Anything, mock.Anything).
				Return(errors.New(fake.Lorem().Sentence(3)))
			worker := newMockjobsWorkerRunner(t)
			worker.EXPECT().
				Run(mock.Anything).
				RunAndReturn(func(ctx context.Context) error { <-ctx.Done(); return nil })
			scheduler := newMockjobsSchedulerRunner(t)
			scheduler.EXPECT().EnqueueDue(mock.Anything).Return(0, nil).Maybe()
			runtime := &startAllRuntime{
				engine:                server,
				worker:                worker,
				scheduler:             scheduler,
				schedulerLoopInterval: time.Hour,
			}
			require.Error(t, runtime.Run(t.Context()))
			require.Error(
				t,
				runtime.componentError(
					t.Context(),
					startAllComponentResult{name: fake.Lorem().Word()},
				),
			)
			cancelledCtx, cancel := context.WithCancel(t.Context())
			cancel()
			require.NoError(
				t,
				runtime.componentError(
					cancelledCtx,
					startAllComponentResult{err: context.Canceled},
				),
			)
		},
	)
}
