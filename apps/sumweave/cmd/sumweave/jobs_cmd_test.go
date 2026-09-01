package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJobsCommandErrorHandling(t *testing.T) {
	fake := faker.New()
	t.Run("worker returns cleanup-only errors", func(t *testing.T) {
		worker := newMockjobsWorkerCommandRunner(t)
		cleanupErr := errors.New(fake.Lorem().Sentence(3))
		worker.EXPECT().Run(mock.Anything).Return(nil)
		worker.EXPECT().Close(mock.Anything).Return(cleanupErr)

		cmd := newJobsWorkerCmdWithResolver(func(*cobra.Command) (jobsWorkerCommandRunner, error) {
			return worker, nil
		})

		require.ErrorIs(t, cmd.ExecuteContext(t.Context()), cleanupErr)
	})

	t.Run("worker joins operation and cleanup errors", func(t *testing.T) {
		worker := newMockjobsWorkerCommandRunner(t)
		operationErr := errors.New(fake.Lorem().Sentence(3))
		cleanupErr := errors.New(fake.Lorem().Sentence(3))
		worker.EXPECT().Run(mock.Anything).Return(operationErr)
		worker.EXPECT().Close(mock.Anything).Return(cleanupErr)

		cmd := newJobsWorkerCmdWithResolver(func(*cobra.Command) (jobsWorkerCommandRunner, error) {
			return worker, nil
		})

		err := cmd.ExecuteContext(t.Context())
		require.ErrorIs(t, err, operationErr)
		require.ErrorIs(t, err, cleanupErr)
	})

	t.Run("enqueue-due returns cleanup-only errors", func(t *testing.T) {
		scheduler := newMockjobsSchedulerCommandRunner(t)
		cleanupErr := errors.New(fake.Lorem().Sentence(3))
		scheduler.EXPECT().EnqueueDue(mock.Anything).Return(1, nil)
		scheduler.EXPECT().Close(mock.Anything).Return(cleanupErr)

		cmd := newJobsEnqueueDueCmdWithResolver(func(*cobra.Command) (jobsSchedulerCommandRunner, error) {
			return scheduler, nil
		})

		require.ErrorIs(t, cmd.ExecuteContext(t.Context()), cleanupErr)
	})

	t.Run("enqueue-due joins operation and cleanup errors", func(t *testing.T) {
		scheduler := newMockjobsSchedulerCommandRunner(t)
		operationErr := errors.New(fake.Lorem().Sentence(3))
		cleanupErr := errors.New(fake.Lorem().Sentence(3))
		scheduler.EXPECT().EnqueueDue(mock.Anything).Return(0, operationErr)
		scheduler.EXPECT().Close(mock.Anything).Return(cleanupErr)

		cmd := newJobsEnqueueDueCmdWithResolver(func(*cobra.Command) (jobsSchedulerCommandRunner, error) {
			return scheduler, nil
		})

		err := cmd.ExecuteContext(t.Context())
		require.ErrorIs(t, err, operationErr)
		require.ErrorIs(t, err, cleanupErr)
	})

	t.Run("resolver errors do not close worker", func(t *testing.T) {
		worker := newMockjobsWorkerCommandRunner(t)
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		cmd := newJobsWorkerCmdWithResolver(func(*cobra.Command) (jobsWorkerCommandRunner, error) {
			return worker, resolverErr
		})

		require.ErrorIs(t, cmd.ExecuteContext(t.Context()), resolverErr)
	})

	t.Run("resolver errors do not close scheduler", func(t *testing.T) {
		scheduler := newMockjobsSchedulerCommandRunner(t)
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		cmd := newJobsEnqueueDueCmdWithResolver(func(*cobra.Command) (jobsSchedulerCommandRunner, error) {
			return scheduler, resolverErr
		})

		require.ErrorIs(t, cmd.ExecuteContext(t.Context()), resolverErr)
	})
	t.Run("canceled worker exit is clean and closes the root", func(t *testing.T) {
		worker := newMockjobsWorkerCommandRunner(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		worker.EXPECT().Run(mock.Anything).Return(context.Canceled)
		worker.EXPECT().Close(mock.Anything).Return(nil)

		cmd := newJobsWorkerCmdWithResolver(func(*cobra.Command) (jobsWorkerCommandRunner, error) {
			return worker, nil
		})

		require.NoError(t, cmd.ExecuteContext(ctx))
	})
}
