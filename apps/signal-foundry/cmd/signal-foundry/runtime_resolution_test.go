package main

import (
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestRuntimeResolution(t *testing.T) {
	makeRoot := func(t *testing.T, command *cobra.Command) *cobra.Command {
		t.Helper()
		root := newRootCmd()
		root.AddCommand(command)
		require.NoError(t, root.PersistentFlags().Set("env", "test"))
		return root
	}
	prepareSchemas := func(t *testing.T) {
		t.Helper()
		container := dig.New()
		command := newDatabaseMigrateCmd(container)
		root := makeRoot(t, command)
		migrator, err := resolveDatabaseMigrator(root, container)
		require.NoError(t, err)
		require.NoError(t, migrator.Migrate(t.Context()))
	}

	t.Run("database migration resolves the explicit migrator", func(t *testing.T) {
		container := dig.New()
		command := newDatabaseMigrateCmd(container)
		root := makeRoot(t, command)
		resolved, err := resolveDatabaseMigrator(command, container)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		require.Same(t, root, command.Root())
	})

	t.Run("jobs split mode resolves worker and scheduler after finance job registration", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.sqlite"))
		prepareSchemas(t)
		container := dig.New()
		jobsCommand := newJobsCmd(container)
		root := makeRoot(t, jobsCommand)
		workerCommand, _, err := jobsCommand.Find([]string{jobsWorkerCommandName})
		require.NoError(t, err)
		worker, err := resolveJobsWorker(workerCommand, container)
		require.NoError(t, err)
		require.NotNil(t, worker)
		require.Same(t, root, workerCommand.Root())

		container = dig.New()
		jobsCommand = newJobsCmd(container)
		root = makeRoot(t, jobsCommand)
		schedulerCommand, _, err := jobsCommand.Find([]string{enqueueDueCommandName})
		require.NoError(t, err)
		scheduler, err := resolveJobsScheduler(schedulerCommand, container)
		require.NoError(t, err)
		require.NotNil(t, scheduler)
		require.Same(t, root, schedulerCommand.Root())
	})

	t.Run("start all resolves the local component composition", func(t *testing.T) {
		t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.sqlite"))
		prepareSchemas(t)
		container := dig.New()
		command := newStartAllCmd(container)
		makeRoot(t, command)
		runtime, err := resolveStartAllRuntime(command, container, startServerParams{noop: true})
		require.NoError(t, err)
		require.NotNil(t, runtime)
	})

	t.Run(
		"root start command applies persistent defaults before surfacing HTTP composition errors",
		func(t *testing.T) {
			t.Setenv("APP_APPLICATION_DATABASE_DSN", filepath.Join(t.TempDir(), "application.sqlite"))
			prepareSchemas(t)
			root := setupCommands()
			root.SetArgs([]string{"--env", "test", startCommandName, "--noop"})
			require.Error(t, root.ExecuteContext(t.Context()))
		},
	)

	t.Run("resolver setup failures remain visible to operators", func(t *testing.T) {
		container := dig.New()
		require.NoError(t, container.Provide(func() *internal.DatabaseMigrator { return nil }))
		command := newDatabaseMigrateCmd(container)
		makeRoot(t, command)
		_, err := resolveDatabaseMigrator(command, container)
		require.Error(t, err)

		container = dig.New()
		require.NoError(t, container.Provide(func() *internal.DatabaseMigrator { return nil }))
		jobsCommand := newJobsCmd(container)
		makeRoot(t, jobsCommand)
		workerCommand, _, findErr := jobsCommand.Find([]string{jobsWorkerCommandName})
		require.NoError(t, findErr)
		_, err = resolveJobsWorker(workerCommand, container)
		require.Error(t, err)
	})
}
