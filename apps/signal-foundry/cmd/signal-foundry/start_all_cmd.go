package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	signalfoundry "github.com/gemyago/signal-foundry/apps/signal-foundry"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

const startAllCommandName = "start-all"

type startHTTPServerOpt = signalfoundry.EngineStartServerOpt

type startAllHTTPServer interface {
	StartHTTPServer(context.Context, ...startHTTPServerOpt) error
}

type startAllRunner interface {
	Run(context.Context) error
}

type startAllResolver func(*cobra.Command, *dig.Container, startServerParams) (startAllRunner, error)

type startAllComponentResult struct {
	name string
	err  error
}

type startAllRuntime struct {
	logger                *slog.Logger
	engine                startAllHTTPServer
	worker                jobsWorkerRunner
	scheduler             jobsSchedulerRunner
	schedulerLoopInterval time.Duration
	startServerOpts       []startHTTPServerOpt
}

func newStartAllCmd(container *dig.Container) *cobra.Command {
	return newStartAllCmdWithResolver(container, resolveStartAllRuntime)
}

func newStartAllCmdWithResolver(container *dig.Container, resolver startAllResolver) *cobra.Command {
	params := startServerParams{}
	cmd := &cobra.Command{
		Use:   startAllCommandName,
		Short: "Start the local backend with HTTP server, jobs worker, and scheduler loop",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := resolver(cmd, container, params)
			if err != nil {
				return err
			}
			return runner.Run(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(
		&params.noop,
		"noop",
		params.noop,
		"Do not start. Just setup params and exit. Useful for testing if setup is all working.",
	)
	cmd.Flags().StringVar(
		&params.uiLocation,
		"ui-location",
		params.uiLocation,
		"Path to pre-built UI static assets directory. When set, serves UI from this directory. Empty means API-only mode.",
	)
	return cmd
}

//nolint:ireturn // Cobra wiring resolves a concrete runtime behind a command-local interface.
func resolveStartAllRuntime(
	cmd *cobra.Command,
	container *dig.Container,
	params startServerParams,
) (startAllRunner, error) {
	engine, setupErr := newEngineFromRootWithOpts(
		cmd.Root(),
		container,
		internal.WithEngineJobsWorkerAutoStart(false),
	)
	if setupErr != nil {
		return nil, setupErr
	}
	primeErr := primeFinanceJobs(container)
	if primeErr != nil {
		return nil, primeErr
	}

	type deps struct {
		dig.In

		RootLogger            *slog.Logger
		Worker                *jobspkg.Worker
		Scheduler             *jobspkg.Scheduler
		SchedulerLoopInterval time.Duration `name:"config.jobs.scheduler.loopInterval"`
	}

	var resolved deps
	invokeErr := container.Invoke(func(inner deps) {
		resolved = inner
	})
	if invokeErr != nil {
		return nil, fmt.Errorf("resolve start-all dependencies: %w", invokeErr)
	}

	return &startAllRuntime{
		logger:                resolved.RootLogger,
		engine:                engine,
		worker:                resolved.Worker,
		scheduler:             resolved.Scheduler,
		schedulerLoopInterval: resolved.SchedulerLoopInterval,
		startServerOpts: []startHTTPServerOpt{
			signalfoundry.WithStartHTTPServerUILocation(params.uiLocation),
			signalfoundry.WithStartHTTPServerNoop(params.noop),
		},
	}, nil
}

func (r *startAllRuntime) Run(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan startAllComponentResult, 3)
	var wg sync.WaitGroup
	r.startComponent(runCtx, &wg, results, "http server", func(componentCtx context.Context) error {
		return r.engine.StartHTTPServer(componentCtx, r.startServerOpts...)
	})
	r.startComponent(runCtx, &wg, results, "jobs worker", r.worker.Run)
	r.startComponent(runCtx, &wg, results, "scheduler loop", r.runSchedulerLoop)

	for {
		select {
		case <-ctx.Done():
			return stopStartAll(cancel, &wg, nil)
		case result := <-results:
			resultErr := r.componentError(ctx, result)
			if resultErr == nil {
				continue
			}
			return stopStartAll(cancel, &wg, resultErr)
		}
	}
}

func (r *startAllRuntime) runSchedulerLoop(ctx context.Context) error {
	r.runSchedulerTick(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(r.schedulerLoopInterval):
		}
		r.runSchedulerTick(ctx)
	}
}

func (r *startAllRuntime) runSchedulerTick(ctx context.Context) {
	if _, err := r.scheduler.EnqueueDue(ctx); err != nil && !errors.Is(err, context.Canceled) {
		r.logger.ErrorContext(ctx, "start-all scheduler tick failed", "error", err)
	}
}

func (r *startAllRuntime) validate() error {
	if r.engine == nil {
		return errors.New("start-all HTTP server is required")
	}
	if r.worker == nil {
		return errors.New("start-all jobs worker is required")
	}
	if r.scheduler == nil {
		return errors.New("start-all scheduler is required")
	}
	if r.schedulerLoopInterval <= 0 {
		return errors.New("start-all scheduler loop interval is required")
	}
	if r.logger == nil {
		r.logger = slog.Default()
	}
	return nil
}

func (r *startAllRuntime) startComponent(
	runningCtx context.Context,
	wg *sync.WaitGroup,
	results chan<- startAllComponentResult,
	name string,
	run func(context.Context) error,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		results <- startAllComponentResult{name: name, err: run(runningCtx)}
	}()
}

func (r *startAllRuntime) componentError(ctx context.Context, result startAllComponentResult) error {
	if ctx.Err() != nil && isStartAllCleanExit(result.err) {
		return nil
	}
	if result.err == nil {
		return fmt.Errorf("%s stopped unexpectedly", result.name)
	}
	return fmt.Errorf("run %s: %w", result.name, result.err)
}

func stopStartAll(cancel context.CancelFunc, wg *sync.WaitGroup, err error) error {
	cancel()
	wg.Wait()
	return err
}

func isStartAllCleanExit(err error) bool {
	return err == nil || errors.Is(err, context.Canceled)
}
