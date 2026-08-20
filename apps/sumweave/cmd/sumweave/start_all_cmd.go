package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	sumweave "github.com/gemyago/sumweave/apps/sumweave"
	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	"github.com/spf13/cobra"
)

const startAllCommandName = "start-all"

type startHTTPServerOpt = sumweave.EngineStartServerOpt

type startAllHTTPServer interface {
	StartHTTPServer(context.Context, ...startHTTPServerOpt) error
	Close(context.Context) error
}

type startAllRunner interface {
	Run(context.Context) error
}

type startAllResolver func(*cobra.Command, startServerParams) (startAllRunner, error)

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
	noop                  bool
	close                 func(context.Context) error
}

func newStartAllCmd() *cobra.Command {
	return newStartAllCmdWithResolver(resolveStartAllRuntime)
}

func newStartAllCmdWithResolver(resolver startAllResolver) *cobra.Command {
	params := startServerParams{}
	cmd := &cobra.Command{
		Use:   startAllCommandName,
		Short: "Start the local backend with HTTP server, jobs worker, and scheduler loop",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := resolver(cmd, params)
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
	return cmd
}

//nolint:ireturn // Cobra wiring resolves a concrete runtime behind a command-local interface.
func resolveStartAllRuntime(
	cmd *cobra.Command,
	params startServerParams,
) (startAllRunner, error) { // coverage-ignore // Root construction is covered by wireup and command smoke tests.
	options, err := commandRootOptionsFromRoot(cmd.Root())
	if err != nil {
		return nil, err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}
	if ctx == nil {
		return nil, errors.New("start-all command context is required")
	}
	httpRoot, err := wireup.BuildHTTP(ctx, wireup.HTTPOptions{
		Environment: options.Environment, DefaultLogLevel: options.DefaultLogLevel,
		JSONLogs: options.JSONLogs, LogsFile: options.LogsFile,
	})
	if err != nil {
		return nil, fmt.Errorf("build start-all HTTP root: %w", err)
	}
	workerRoot, err := wireup.BuildWorker(ctx, wireup.WorkerOptions{
		Environment: options.Environment, DefaultLogLevel: options.DefaultLogLevel,
		JSONLogs: options.JSONLogs, LogsFile: options.LogsFile,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("build start-all worker root: %w", err), httpRoot.Close(ctx))
	}
	schedulerRoot, err := wireup.BuildScheduler(ctx, wireup.SchedulerOptions{
		Environment: options.Environment, DefaultLogLevel: options.DefaultLogLevel,
		JSONLogs: options.JSONLogs, LogsFile: options.LogsFile,
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("build start-all scheduler root: %w", err),
			workerRoot.Close(ctx),
			httpRoot.Close(ctx),
		)
	}

	return &startAllRuntime{
		logger:    httpRoot.Logger(),
		engine:    startAllHTTPRoot{root: httpRoot, noop: params.noop},
		worker:    workerRoot.Worker,
		scheduler: schedulerRoot, schedulerLoopInterval: schedulerRoot.SchedulerLoopInterval,
		noop: params.noop,
		close: func(shutdownCtx context.Context) error {
			return errors.Join(
				workerRoot.Close(shutdownCtx),
				schedulerRoot.Close(shutdownCtx),
				httpRoot.Close(shutdownCtx),
			)
		},
		startServerOpts: []startHTTPServerOpt{
			sumweave.WithStartHTTPServerNoop(params.noop),
		},
	}, nil
}

type startAllHTTPRoot struct {
	root *wireup.HTTPRoot
	noop bool
}

func (server startAllHTTPRoot) StartHTTPServer(ctx context.Context, _ ...startHTTPServerOpt) error {
	return server.root.StartHTTPServer(ctx, server.noop)
}

func (server startAllHTTPRoot) Close(ctx context.Context) error {
	return server.root.Close(ctx)
}

func (r *startAllRuntime) Run(ctx context.Context) (err error) {
	if validateErr := r.validate(); validateErr != nil {
		return validateErr
	}
	if r.noop {
		return r.closeResources(context.WithoutCancel(ctx))
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

	defer func() {
		err = errors.Join(err, r.closeResources(context.WithoutCancel(ctx)))
	}()
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

func (r *startAllRuntime) closeResources(ctx context.Context) error {
	if r.close != nil {
		return r.close(ctx)
	}
	return r.engine.Close(ctx)
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
	if r.noop {
		return nil
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
