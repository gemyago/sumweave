package main

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestStartAllCommand(t *testing.T) {
	chdirModuleRoot(t)
	fake := faker.New()

	t.Run("start-all command is discoverable and exposes standard HTTP flags", func(t *testing.T) {
		rootCmd := setupCommands()
		startAllCmd := findRootCommandByName(t, rootCmd, startAllCommandName)

		require.NotNil(t, startAllCmd.Flags().Lookup("noop"))
		require.Nil(t, startAllCmd.Flags().Lookup("ui-location"))
	})

	t.Run("resolver and runner errors are surfaced", func(t *testing.T) {
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		cmd := newStartAllCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container, startServerParams) (startAllRunner, error) {
				return nil, resolverErr
			},
		)
		cmd.SetContext(t.Context())
		require.ErrorIs(t, cmd.RunE(cmd, nil), resolverErr)

		runnerErr := errors.New(fake.Lorem().Sentence(3))
		cmd = newStartAllCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container, startServerParams) (startAllRunner, error) {
				return startAllRunnerStub{runErr: runnerErr}, nil
			},
		)
		cmd.SetContext(t.Context())
		require.ErrorIs(t, cmd.RunE(cmd, nil), runnerErr)
	})

	t.Run(
		"resolver composes engine worker and scheduler with startup auto-start disabled",
		func(t *testing.T) {
			dsn := filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite")
			t.Setenv("APP_DATALAYER_DATABASE_DSN", dsn)
			t.Setenv("APP_JOBS_WORKER_ENABLED", "true")
			t.Setenv("APP_JOBS_SCHEDULER_LOOPINTERVAL", "17ms")
			migrateAppDatabaseForTests(t, dsn)

			container := dig.New()
			rootCmd := newRootCmd()
			rootCmd.SetContext(t.Context())
			require.NoError(t, rootCmd.PersistentFlags().Set("env", "test"))
			require.NoError(t, rootCmd.PersistentFlags().Set("logs-file", testLogFile(t)))
			rootCmd.AddCommand(newStartAllCmd(container))
			command, _, err := rootCmd.Find([]string{startAllCommandName})
			require.NoError(t, err)

			runner, err := resolveStartAllRuntime(command, container, startServerParams{noop: true})
			require.NoError(t, err)

			resolved, ok := runner.(*startAllRuntime)
			require.True(t, ok)
			require.NotNil(t, resolved.engine)
			require.IsType(t, &jobspkg.Worker{}, resolved.worker)
			require.IsType(t, &jobspkg.Scheduler{}, resolved.scheduler)
			assert.Equal(t, 17*time.Millisecond, resolved.schedulerLoopInterval)
			assertNoJobsWorkerHook(t, container)
		},
	)
}

func TestStartAllRuntime(t *testing.T) {
	fake := faker.New()

	makeBlockingServer := func(cancelled chan<- struct{}, started chan<- struct{}) startAllHTTPServer {
		return startAllHTTPServerStub{
			start: func(ctx context.Context, _ ...startHTTPServerOpt) error {
				if started != nil {
					close(started)
				}
				<-ctx.Done()
				if cancelled != nil {
					close(cancelled)
				}
				return nil
			},
		}
	}
	makeBlockingWorker := func(cancelled chan<- struct{}, started chan<- struct{}) jobsWorkerRunner {
		return workerRunnerStubWithRunOnce{
			run: func(ctx context.Context) error {
				if started != nil {
					close(started)
				}
				<-ctx.Done()
				if cancelled != nil {
					close(cancelled)
				}
				return nil
			},
		}
	}

	t.Run("runs server worker and non-overlapping scheduler loop", func(t *testing.T) {
		serverStarted := make(chan struct{})
		workerStarted := make(chan struct{})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var tickCount atomic.Int32
		var concurrentTicks atomic.Int32
		var maxConcurrentTicks atomic.Int32
		startedAt := time.Now()
		firstTickAt := make(chan time.Time, 1)

		runtime := &startAllRuntime{
			logger: telemetry.RootTestLogger(),
			engine: makeBlockingServer(nil, serverStarted),
			worker: makeBlockingWorker(nil, workerStarted),
			scheduler: schedulerRunnerStubWithRun(func(_ context.Context) (int, error) {
				awaitSignal(t, serverStarted)
				awaitSignal(t, workerStarted)
				count := tickCount.Add(1)
				if count == 1 {
					firstTickAt <- time.Now()
				}
				concurrency := concurrentTicks.Add(1)
				updateMaxAtomic(&maxConcurrentTicks, concurrency)
				time.Sleep(25 * time.Millisecond)
				concurrentTicks.Add(-1)
				if count >= 2 {
					cancel()
				}
				return 1, nil
			}),
			schedulerLoopInterval: 10 * time.Millisecond,
		}

		require.NoError(t, runtime.Run(ctx))
		assert.Equal(t, int32(2), tickCount.Load())
		assert.Equal(t, int32(1), maxConcurrentTicks.Load())
		assert.Less(t, (<-firstTickAt).Sub(startedAt), 20*time.Millisecond)
	})

	t.Run("continues later ticks after a tick error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var tickCount atomic.Int32
		firstErr := errors.New(fake.Lorem().Sentence(4))

		runtime := &startAllRuntime{
			logger:                telemetry.RootTestLogger(),
			engine:                makeBlockingServer(nil, nil),
			worker:                makeBlockingWorker(nil, nil),
			schedulerLoopInterval: 5 * time.Millisecond,
			scheduler: schedulerRunnerStubWithRun(func(context.Context) (int, error) {
				count := tickCount.Add(1)
				if count == 1 {
					return 0, firstErr
				}
				cancel()
				return 1, nil
			}),
		}

		require.NoError(t, runtime.Run(ctx))
		assert.Equal(t, int32(2), tickCount.Load())
	})

	t.Run("cancels sibling components and fails when the HTTP server stops unexpectedly", func(t *testing.T) {
		workerCancelled := make(chan struct{})
		runtime := &startAllRuntime{
			logger: telemetry.RootTestLogger(),
			engine: startAllHTTPServerStub{
				start: func(context.Context, ...startHTTPServerOpt) error {
					return nil
				},
			},
			worker:                makeBlockingWorker(workerCancelled, nil),
			scheduler:             schedulerRunnerStubWithRun(func(context.Context) (int, error) { return 1, nil }),
			schedulerLoopInterval: time.Hour,
		}

		err := runtime.Run(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "http server stopped unexpectedly")
		awaitSignal(t, workerCancelled)
	})

	t.Run("cancels sibling components and fails when the worker errors", func(t *testing.T) {
		serverCancelled := make(chan struct{})
		workerErr := errors.New(fake.Lorem().Sentence(4))
		runtime := &startAllRuntime{
			logger: telemetry.RootTestLogger(),
			engine: makeBlockingServer(serverCancelled, nil),
			worker: workerRunnerStubWithRunOnce{
				run: func(context.Context) error {
					return workerErr
				},
			},
			scheduler:             schedulerRunnerStubWithRun(func(context.Context) (int, error) { return 1, nil }),
			schedulerLoopInterval: time.Hour,
		}

		err := runtime.Run(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, err, "run jobs worker")
		require.ErrorIs(t, err, workerErr)
		awaitSignal(t, serverCancelled)
	})

	t.Run("rejects missing scheduler loop interval configuration", func(t *testing.T) {
		runtime := &startAllRuntime{
			logger:    telemetry.RootTestLogger(),
			engine:    makeBlockingServer(nil, nil),
			worker:    makeBlockingWorker(nil, nil),
			scheduler: schedulerRunnerStubWithRun(func(context.Context) (int, error) { return 0, nil }),
		}

		err := runtime.Run(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "scheduler loop interval")
	})

	t.Run("rejects missing required components", func(t *testing.T) {
		tests := []struct {
			name    string
			runtime startAllRuntime
			wantErr string
		}{
			{
				name: "missing server",
				runtime: startAllRuntime{
					worker: makeBlockingWorker(nil, nil),
					scheduler: schedulerRunnerStubWithRun(func(context.Context) (int, error) {
						return 0, nil
					}),
					schedulerLoopInterval: time.Second,
				},
				wantErr: "HTTP server",
			},
			{
				name: "missing worker",
				runtime: startAllRuntime{
					engine: makeBlockingServer(nil, nil),
					scheduler: schedulerRunnerStubWithRun(func(context.Context) (int, error) {
						return 0, nil
					}),
					schedulerLoopInterval: time.Second,
				},
				wantErr: "jobs worker",
			},
			{
				name: "missing scheduler",
				runtime: startAllRuntime{
					engine:                makeBlockingServer(nil, nil),
					worker:                makeBlockingWorker(nil, nil),
					schedulerLoopInterval: time.Second,
				},
				wantErr: "scheduler is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.runtime.Run(t.Context())
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
			})
		}
	})

	t.Run("uses the default logger and exits cleanly on parent cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		runtime := &startAllRuntime{
			engine:                makeBlockingServer(nil, nil),
			worker:                makeBlockingWorker(nil, nil),
			schedulerLoopInterval: time.Hour,
			scheduler: schedulerRunnerStubWithRun(func(context.Context) (int, error) {
				cancel()
				return 1, nil
			}),
		}

		require.NoError(t, runtime.Run(ctx))
		require.NotNil(t, runtime.logger)
	})
}

func awaitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for signal")
	}
}

func updateMaxAtomic(target *atomic.Int32, value int32) {
	for {
		current := target.Load()
		if value <= current {
			return
		}
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

type startAllRunnerStub struct {
	runErr error
}

func (s startAllRunnerStub) Run(context.Context) error { return s.runErr }

type startAllHTTPServerStub struct {
	start func(context.Context, ...startHTTPServerOpt) error
}

func (s startAllHTTPServerStub) StartHTTPServer(ctx context.Context, opts ...startHTTPServerOpt) error {
	return s.start(ctx, opts...)
}

type workerRunnerStubWithRunOnce struct {
	run func(context.Context) error
}

func (w workerRunnerStubWithRunOnce) Run(ctx context.Context) error { return w.run(ctx) }
func (w workerRunnerStubWithRunOnce) RunOnce(context.Context) error { return nil }

type schedulerRunnerStubWithRun func(context.Context) (int, error)

func (s schedulerRunnerStubWithRun) EnqueueDue(ctx context.Context) (int, error) { return s(ctx) }
