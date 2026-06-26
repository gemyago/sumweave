package main

import (
	"context"
	"fmt"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	jobspkg "github.com/gemyago/signal-foundry/apps/signal-foundry/internal/jobs"
	financepkg "github.com/gemyago/signal-foundry/finance"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

const (
	jobsCommandName       = "jobs"
	jobsWorkerCommandName = "worker"
	enqueueDueCommandName = "enqueue-due"
)

func newJobsCmd(container *dig.Container) *cobra.Command {
	cmd := &cobra.Command{Use: jobsCommandName, Short: "Run durable jobs worker and scheduler commands"}
	cmd.AddCommand(newJobsWorkerCmd(container), newJobsEnqueueDueCmd(container))
	return cmd
}

type jobsWorkerRunner interface {
	Run(context.Context) error
	RunOnce(context.Context) error
}

type jobsWorkerResolver func(*cobra.Command, *dig.Container) (jobsWorkerRunner, error)

type jobsSchedulerRunner interface {
	EnqueueDue(context.Context) (int, error)
}

type jobsSchedulerResolver func(*cobra.Command, *dig.Container) (jobsSchedulerRunner, error)

func newJobsWorkerCmd(container *dig.Container) *cobra.Command {
	return newJobsWorkerCmdWithResolver(container, resolveJobsWorker)
}

func newJobsWorkerCmdWithResolver(container *dig.Container, resolver jobsWorkerResolver) *cobra.Command {
	once := false
	cmd := &cobra.Command{
		Use:   jobsWorkerCommandName,
		Short: "Run durable jobs worker mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			worker, err := resolver(cmd, container)
			if err != nil {
				return err
			}
			if once {
				return worker.RunOnce(cmd.Context())
			}
			return worker.Run(cmd.Context())
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, "Run a single consumer pass and exit")
	return cmd
}

func newJobsEnqueueDueCmd(container *dig.Container) *cobra.Command {
	return newJobsEnqueueDueCmdWithResolver(container, resolveJobsScheduler)
}

func newJobsEnqueueDueCmdWithResolver(container *dig.Container, resolver jobsSchedulerResolver) *cobra.Command {
	return &cobra.Command{
		Use:   enqueueDueCommandName,
		Short: "Enqueue due durable jobs from the schedule registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scheduler, err := resolver(cmd, container)
			if err != nil {
				return err
			}
			_, err = scheduler.EnqueueDue(cmd.Context())
			return err
		},
	}
}

func resolveJobsWorker(cmd *cobra.Command, container *dig.Container) (jobsWorkerRunner, error) { //nolint:ireturn
	if _, err := newEngineFromRootWithOpts(
		cmd.Root(),
		container,
		internal.WithEngineJobsWorkerAutoStart(false),
	); err != nil {
		return nil, err
	}
	if err := primeFinanceJobs(container); err != nil {
		return nil, err
	}
	var worker *jobspkg.Worker
	if err := container.Invoke(func(resolved *jobspkg.Worker) { worker = resolved }); err != nil {
		return nil, fmt.Errorf("resolve jobs worker: %w", err)
	}
	return worker, nil
}

func resolveJobsScheduler(cmd *cobra.Command, container *dig.Container) (jobsSchedulerRunner, error) { //nolint:ireturn
	if _, err := newEngineFromRootWithOpts(
		cmd.Root(),
		container,
		internal.WithEngineJobsWorkerAutoStart(false),
	); err != nil {
		return nil, err
	}
	if err := primeFinanceJobs(container); err != nil {
		return nil, err
	}
	var scheduler *jobspkg.Scheduler
	if err := container.Invoke(func(resolved *jobspkg.Scheduler) { scheduler = resolved }); err != nil {
		return nil, fmt.Errorf("resolve jobs scheduler: %w", err)
	}
	return scheduler, nil
}

func primeFinanceJobs(container *dig.Container) error {
	if container == nil {
		return nil
	}
	if err := container.Invoke(func(*financepkg.Service) {}); err != nil {
		return fmt.Errorf("prime finance jobs: %w", err)
	}
	return nil
}
