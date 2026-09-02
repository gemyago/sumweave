//go:build postgres_test

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRuntimeResolution(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("APP_DATADIR", t.TempDir())
	makeRoot := func(t *testing.T, command *cobra.Command) *cobra.Command {
		t.Helper()
		root := newRootCmd()
		root.SetContext(t.Context())
		root.AddCommand(command)
		require.NoError(t, root.PersistentFlags().Set("env", "test"))
		return root
	}
	t.Run("jobs split mode resolves worker and scheduler after finance job registration", func(t *testing.T) {
		jobsCommand := newJobsCmd()
		root := makeRoot(t, jobsCommand)
		workerCommand, _, err := jobsCommand.Find([]string{jobsWorkerCommandName})
		require.NoError(t, err)
		worker, err := resolveJobsWorker(workerCommand)
		require.NoError(t, err)
		require.NotNil(t, worker)
		require.NoError(t, worker.Close(t.Context()))
		require.Same(t, root, workerCommand.Root())

		jobsCommand = newJobsCmd()
		root = makeRoot(t, jobsCommand)
		schedulerCommand, _, err := jobsCommand.Find([]string{enqueueDueCommandName})
		require.NoError(t, err)
		scheduler, err := resolveJobsScheduler(schedulerCommand)
		require.NoError(t, err)
		require.NotNil(t, scheduler)
		require.NoError(t, scheduler.Close(t.Context()))
		require.Same(t, root, schedulerCommand.Root())
	})

	t.Run("start all noop resolves, validates, and closes local component wireup", func(t *testing.T) {
		command := newStartAllCmd()
		makeRoot(t, command)
		runtime, err := resolveStartAllRuntime(command, startServerParams{noop: true})
		require.NoError(t, err)
		require.NotNil(t, runtime)
		require.NoError(t, runtime.Run(t.Context()))
	})

	t.Run("split jobs resolution accepts test defaults and rejects missing command context", func(t *testing.T) {
		jobsCommand := newJobsCmd()
		makeRoot(t, jobsCommand)
		workerCommand, _, findErr := jobsCommand.Find([]string{jobsWorkerCommandName})
		require.NoError(t, findErr)
		worker, err := resolveJobsWorker(workerCommand)
		require.NoError(t, err)
		require.NoError(t, worker.Close(t.Context()))
		_, err = resolveJobsScheduler(&cobra.Command{})
		require.Error(t, err)
	})
}
