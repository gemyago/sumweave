package main

import (
	"bytes"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApplicationCommands(t *testing.T) {
	fake := faker.New()
	makeUserDeps := func(t *testing.T) (*auth.UserStore, *auth.Argon2idHasher) {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "users.sqlite")
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := auth.NewUserStore(
			auth.UserStoreDeps{
				SQLDB:       db,
				DatabaseDSN: dsn,
				TablePrefix: "users_",
				IDGen:       ident.NewDefaultGenerator(),
				Logger:      slog.Default(),
			},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return store, auth.NewArgon2idHasherWithParams(
			auth.Argon2idHasherParams{Memory: 8, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32},
		)
	}
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

	t.Run("user administration creates lists and changes financial operators", func(t *testing.T) {
		store, hasher := makeUserDeps(t)
		username, password := fake.Internet().User(), fake.Internet().Password()
		makeAddParams := func(candidatePassword string, ifNotExists bool) userAddParams {
			return userAddParams{
				Username:    username,
				Password:    candidatePassword,
				IfNotExists: ifNotExists,
			}
		}
		var output bytes.Buffer
		require.NoError(
			t,
			runUserAdd(
				t.Context(),
				userAddCmdDeps{Store: store, Hasher: hasher},
				makeAddParams(password, false),
				&output,
			),
		)
		require.Contains(t, output.String(), username)
		require.Error(
			t,
			runUserAdd(
				t.Context(),
				userAddCmdDeps{Store: store, Hasher: hasher},
				makeAddParams(fake.Internet().Password(), false),
				&bytes.Buffer{},
			),
		)
		var ensureOutput bytes.Buffer
		require.NoError(
			t,
			runUserAdd(
				t.Context(),
				userAddCmdDeps{Store: store, Hasher: hasher},
				makeAddParams(fake.Internet().Password(), true),
				&ensureOutput,
			),
		)
		require.Contains(t, ensureOutput.String(), "already exists")
		existingUser, err := store.GetByUsername(t.Context(), username)
		require.NoError(t, err)
		passwordUnchanged, err := hasher.Verify(password, existingUser.PasswordHash)
		require.NoError(t, err)
		require.True(t, passwordUnchanged)
		var listed bytes.Buffer
		require.NoError(t, runUserList(t.Context(), userListCmdDeps{Store: store}, &listed))
		require.Contains(t, listed.String(), username)
		newPassword := fake.Internet().Password()
		require.NoError(
			t,
			runUserChangePassword(
				t.Context(),
				userChangePasswordCmdDeps{Store: store, Hasher: hasher},
				userChangePasswordParams{Username: username, Password: newPassword},
				&bytes.Buffer{},
			),
		)
		user, err := store.GetByUsername(t.Context(), username)
		require.NoError(t, err)
		ok, err := hasher.Verify(newPassword, user.PasswordHash)
		require.NoError(t, err)
		require.True(t, ok)
		require.Error(
			t,
			runUserChangePassword(
				t.Context(),
				userChangePasswordCmdDeps{Store: store, Hasher: hasher},
				userChangePasswordParams{Username: fake.Internet().User(), Password: newPassword},
				&bytes.Buffer{},
			),
		)
		cmd := setupCommands()
		require.NotNil(t, cmd)
	})

	t.Run("user commands resolve narrow administration capabilities", func(t *testing.T) {
		store, hasher := makeUserDeps(t)
		username, password := fake.Internet().User(), fake.Internet().Password()
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		resolver := func(*cobra.Command) (userCommandRuntime, error) {
			if username == "" {
				return userCommandRuntime{}, resolverErr
			}
			return userCommandRuntime{store: store, hasher: hasher}, nil
		}
		newCommand := func(t *testing.T, args ...string) *cobra.Command {
			t.Helper()
			root := newRootCmd()
			root.AddCommand(newUserCmdWithResolver(resolver))
			root.SetArgs(args)
			return root
		}

		addCommand := newCommand(t, "user", "add", "--username", username, "--password", password)
		require.NoError(t, addCommand.ExecuteContext(t.Context()))
		listCommand := newCommand(t, "user", "list")
		require.NoError(t, listCommand.ExecuteContext(t.Context()))
		changePasswordCommand := newCommand(
			t,
			"user",
			"change-password",
			"--username",
			username,
			"--password",
			fake.Internet().Password(),
		)
		require.NoError(t, changePasswordCommand.ExecuteContext(t.Context()))

		failingCommand := newRootCmd()
		failingCommand.AddCommand(newUserCmdWithResolver(
			func(*cobra.Command) (userCommandRuntime, error) { return userCommandRuntime{}, resolverErr },
		))
		failingCommand.SetArgs([]string{"user", "list"})
		require.ErrorIs(t, failingCommand.ExecuteContext(t.Context()), resolverErr)
	})
}
