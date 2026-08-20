package main

import (
	"context"
	"errors"
	"fmt"

	jobspkg "github.com/gemyago/sumweave/apps/sumweave/internal/jobs"
	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	"github.com/spf13/cobra"
)

const (
	jobsCommandName       = "jobs"
	jobsWorkerCommandName = "worker"
	enqueueDueCommandName = "enqueue-due"
)

func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: jobsCommandName, Short: "Run durable jobs worker and scheduler commands"}
	cmd.AddCommand(newJobsWorkerCmd(), newJobsEnqueueDueCmd())
	return cmd
}

type jobsWorkerRunner interface {
	Run(context.Context) error
	RunOnce(context.Context) error
}

type jobsWorkerCommandRunner interface {
	jobsWorkerRunner
	Close(context.Context) error
}

type jobsWorkerResolver func(*cobra.Command) (jobsWorkerCommandRunner, error)

type jobsSchedulerRunner interface {
	EnqueueDue(context.Context) (int, error)
}

type jobsSchedulerCommandRunner interface {
	jobsSchedulerRunner
	Close(context.Context) error
}

type jobsSchedulerResolver func(*cobra.Command) (jobsSchedulerCommandRunner, error)

func newJobsWorkerCmd() *cobra.Command {
	return newJobsWorkerCmdWithResolver(resolveJobsWorker)
}

func newJobsWorkerCmdWithResolver(resolver jobsWorkerResolver) *cobra.Command {
	once := false
	cmd := &cobra.Command{
		Use:   jobsWorkerCommandName,
		Short: "Run durable jobs worker mode",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			worker, err := resolver(cmd)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, worker.Close(cmd.Context()))
			}()
			if once {
				err = worker.RunOnce(cmd.Context())
				return err
			}
			err = worker.Run(cmd.Context())
			return err
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, "Run a single consumer pass and exit")
	return cmd
}

func newJobsEnqueueDueCmd() *cobra.Command {
	return newJobsEnqueueDueCmdWithResolver(resolveJobsScheduler)
}

func newJobsEnqueueDueCmdWithResolver(resolver jobsSchedulerResolver) *cobra.Command {
	return &cobra.Command{
		Use:   enqueueDueCommandName,
		Short: "Publish due finance-owned semantic bank and FX commands",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			scheduler, err := resolver(cmd)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, scheduler.Close(cmd.Context()))
			}()
			_, err = scheduler.EnqueueDue(cmd.Context())
			return err
		},
	}
}

//nolint:ireturn
func resolveJobsWorker(cmd *cobra.Command) (jobsWorkerCommandRunner, error) { // coverage-ignore
	options, err := jobsOptionsFromRoot(cmd.Root())
	if err != nil {
		return nil, err
	}
	ctx, err := jobsCommandContext(cmd)
	if err != nil {
		return nil, err
	}
	root, err := wireup.BuildWorker(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("build worker root: %w", err)
	}
	return &jobsWorkerRuntime{worker: root.Worker, close: root.Close}, nil
}

//nolint:ireturn
func resolveJobsScheduler(cmd *cobra.Command) (jobsSchedulerCommandRunner, error) { // coverage-ignore
	options, err := jobsOptionsFromRoot(cmd.Root())
	if err != nil {
		return nil, err
	}
	ctx, err := jobsCommandContext(cmd)
	if err != nil {
		return nil, err
	}
	root, err := wireup.BuildScheduler(ctx, wireup.SchedulerOptions(options))
	if err != nil {
		return nil, fmt.Errorf("build scheduler root: %w", err)
	}
	return &jobsSchedulerRuntime{scheduler: root, close: root.Close}, nil
}

func jobsCommandContext(cmd *cobra.Command) (context.Context, error) { // coverage-ignore
	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}
	if ctx == nil {
		return nil, errors.New("jobs command context is required")
	}
	return ctx, nil
}

func jobsOptionsFromRoot(root *cobra.Command) (wireup.WorkerOptions, error) { // coverage-ignore
	options, err := commandRootOptionsFromRoot(root)
	if err != nil {
		return wireup.WorkerOptions{}, err
	}
	return wireup.WorkerOptions{
		Environment: options.Environment, DefaultLogLevel: options.DefaultLogLevel,
		JSONLogs: options.JSONLogs, LogsFile: options.LogsFile,
	}, nil
}

type jobsWorkerRuntime struct {
	worker *jobspkg.Worker
	close  func(context.Context) error
}

func (runtime *jobsWorkerRuntime) Run(ctx context.Context) error { // coverage-ignore
	return runtime.worker.Run(ctx)
}
func (runtime *jobsWorkerRuntime) RunOnce(ctx context.Context) error { // coverage-ignore
	return runtime.worker.RunOnce(ctx)
}
func (runtime *jobsWorkerRuntime) Close(ctx context.Context) error { // coverage-ignore
	return runtime.close(ctx)
}

type jobsSchedulerRuntime struct {
	scheduler jobsSchedulerRunner
	close     func(context.Context) error
}

func (runtime *jobsSchedulerRuntime) EnqueueDue(ctx context.Context) (int, error) { // coverage-ignore
	return runtime.scheduler.EnqueueDue(ctx)
}
func (runtime *jobsSchedulerRuntime) Close(ctx context.Context) error { // coverage-ignore
	return runtime.close(ctx)
}
