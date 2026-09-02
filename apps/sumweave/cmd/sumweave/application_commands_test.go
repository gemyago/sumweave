package main

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApplicationCommands(t *testing.T) {
	fake := faker.New()
	t.Run("database and jobs commands route resolvers and errors", func(t *testing.T) {
		migrator := newMockdatabaseMigrationRunner(t)
		migrator.EXPECT().Migrate(mock.Anything).Return(nil)
		migrateCmd := newDatabaseMigrateCmdWithResolver(
			func(*cobra.Command) (databaseMigrationRunner, error) { return migrator, nil },
		)
		require.NoError(t, migrateCmd.ExecuteContext(t.Context()))
		migrationErr := errors.New(fake.Lorem().Sentence(3))
		migrator = newMockdatabaseMigrationRunner(t)
		migrator.EXPECT().Migrate(mock.Anything).Return(migrationErr)
		migrateCmd = newDatabaseMigrateCmdWithResolver(
			func(*cobra.Command) (databaseMigrationRunner, error) { return migrator, nil },
		)
		require.ErrorIs(t, migrateCmd.ExecuteContext(t.Context()), migrationErr)
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		migrateCmd = newDatabaseMigrateCmdWithResolver(
			func(*cobra.Command) (databaseMigrationRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, migrateCmd.ExecuteContext(t.Context()), resolverErr)

		worker := newMockjobsWorkerCommandRunner(t)
		worker.EXPECT().Run(mock.Anything).Return(nil).Once()
		worker.EXPECT().RunOnce(mock.Anything).Return(nil).Once()
		worker.EXPECT().Close(mock.Anything).Return(nil).Twice()
		workerCmd := newJobsWorkerCmdWithResolver(
			func(*cobra.Command) (jobsWorkerCommandRunner, error) { return worker, nil },
		)
		require.NoError(t, workerCmd.ExecuteContext(t.Context()))
		require.NoError(t, workerCmd.Flags().Set("once", "true"))
		require.NoError(t, workerCmd.ExecuteContext(t.Context()))
		workerCmd = newJobsWorkerCmdWithResolver(
			func(*cobra.Command) (jobsWorkerCommandRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, workerCmd.ExecuteContext(t.Context()), resolverErr)
		scheduler := newMockjobsSchedulerCommandRunner(t)
		scheduler.EXPECT().EnqueueDue(mock.Anything).Return(1, nil)
		scheduler.EXPECT().Close(mock.Anything).Return(nil)
		schedulerCmd := newJobsEnqueueDueCmdWithResolver(
			func(*cobra.Command) (jobsSchedulerCommandRunner, error) { return scheduler, nil },
		)
		require.NoError(t, schedulerCmd.ExecuteContext(t.Context()))
		schedulerCmd = newJobsEnqueueDueCmdWithResolver(
			func(*cobra.Command) (jobsSchedulerCommandRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, schedulerCmd.ExecuteContext(t.Context()), resolverErr)
	})

	t.Run("builds the command tree without resolving persistence", func(t *testing.T) {
		require.NotNil(t, setupCommands())
		root := setupCommands()
		require.NoError(t, root.PersistentPreRunE(root, nil))
		require.NotNil(t, newStartServerCmd())
	})

	t.Run("start command delegates without opening persistence", func(t *testing.T) {
		runner := newMockstartServerRunner(t)
		runner.EXPECT().StartHTTPServer(mock.Anything, mock.Anything).Return(nil).Once()
		command := newStartServerCmdWithResolver(
			func(*cobra.Command) (startServerRunner, error) { return runner, nil },
		)
		require.NoError(t, command.ExecuteContext(t.Context()))
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		command = newStartServerCmdWithResolver(
			func(*cobra.Command) (startServerRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, command.ExecuteContext(t.Context()), resolverErr)
	})

	t.Run("user command behavior uses injected store boundaries", func(t *testing.T) {
		params := userAddParams{Username: fake.UUID().V4(), Password: fake.Lorem().Text(20)}
		created := &auth.User{ID: fake.UUID().V4(), Username: params.Username, CreatedAt: time.Now()}
		store := newMockuserCommandStore(t)
		store.EXPECT().Create(mock.Anything, mock.MatchedBy(func(input auth.CreateUserParams) bool {
			return input.Username == params.Username && input.PasswordHash != ""
		})).Return(created, nil).Once()
		out := &bytes.Buffer{}
		require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, params, out))
		require.Contains(t, out.String(), "User created")

		store = newMockuserCommandStore(t)
		store.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, auth.ErrUsernameExists).Once()
		out.Reset()
		require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, userAddParams{Username: params.Username, Password: params.Password, IfNotExists: true}, out))
		require.Contains(t, out.String(), "already exists")

		store = newMockuserCommandStore(t)
		listErr := errors.New(fake.Lorem().Sentence(3))
		store.EXPECT().List(mock.Anything).Return(nil, listErr).Once()
		require.ErrorIs(t, runUserList(t.Context(), userListCmdDeps{Store: store}, &bytes.Buffer{}), listErr)
		store = newMockuserCommandStore(t)
		store.EXPECT().List(mock.Anything).Return([]auth.User{*created}, nil).Once()
		out.Reset()
		require.NoError(t, runUserList(t.Context(), userListCmdDeps{Store: store}, out))
		require.Contains(t, out.String(), params.Username)

		store = newMockuserCommandStore(t)
		store.EXPECT().GetByUsername(mock.Anything, params.Username).Return(created, nil).Once()
		store.EXPECT().UpdatePassword(mock.Anything, created.ID, mock.Anything).Return(nil).Once()
		out.Reset()
		require.NoError(t, runUserChangePassword(t.Context(), userChangePasswordCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, userChangePasswordParams{Username: params.Username, Password: params.Password}, out))
		require.Contains(t, out.String(), "Password updated")

		createErr := errors.New(fake.Lorem().Sentence(3))
		store = newMockuserCommandStore(t)
		store.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, createErr).Once()
		require.ErrorIs(t, runUserAdd(t.Context(), userAddCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, params, out), createErr)

		lookupErr := errors.New(fake.Lorem().Sentence(3))
		store = newMockuserCommandStore(t)
		store.EXPECT().GetByUsername(mock.Anything, params.Username).Return(nil, lookupErr).Once()
		require.ErrorIs(t, runUserChangePassword(t.Context(), userChangePasswordCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, userChangePasswordParams{Username: params.Username, Password: params.Password}, out), lookupErr)

		updateErr := errors.New(fake.Lorem().Sentence(3))
		store = newMockuserCommandStore(t)
		store.EXPECT().GetByUsername(mock.Anything, params.Username).Return(created, nil).Once()
		store.EXPECT().UpdatePassword(mock.Anything, created.ID, mock.Anything).Return(updateErr).Once()
		require.ErrorIs(t, runUserChangePassword(t.Context(), userChangePasswordCmdDeps{
			Store: store, Hasher: auth.NewArgon2idHasher(),
		}, userChangePasswordParams{Username: params.Username, Password: params.Password}, out), updateErr)
	})

	t.Run("user cobra commands validate flags before resolving persistence", func(t *testing.T) {
		resolver := func(*cobra.Command) (userCommandRuntime, error) { return userCommandRuntime{}, assert.AnError }
		root := &cobra.Command{Use: "root"}
		root.AddCommand(newUserCmdWithResolver(resolver))
		root.SetArgs([]string{"user", "add", "--username", fake.UUID().V4()})
		require.Error(t, root.ExecuteContext(t.Context()))
		root.SetArgs([]string{"user", "list"})
		require.ErrorIs(t, root.ExecuteContext(t.Context()), assert.AnError)
		root.SetArgs([]string{"user", "change-password", "--username", fake.UUID().V4()})
		require.Error(t, root.ExecuteContext(t.Context()))
	})
}
