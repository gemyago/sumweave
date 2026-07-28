package main

import (
	"bytes"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
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
			dig.New(),
			func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) { return migrator, nil },
		)
		require.NoError(t, migrateCmd.ExecuteContext(t.Context()))
		migrationErr := errors.New(fake.Lorem().Sentence(3))
		migrator = newMockdatabaseMigrationRunner(t)
		migrator.EXPECT().Migrate(mock.Anything).Return(migrationErr)
		migrateCmd = newDatabaseMigrateCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) { return migrator, nil },
		)
		require.ErrorIs(t, migrateCmd.ExecuteContext(t.Context()), migrationErr)
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		migrateCmd = newDatabaseMigrateCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (databaseMigrationRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, migrateCmd.ExecuteContext(t.Context()), resolverErr)

		worker := newMockjobsWorkerRunner(t)
		worker.EXPECT().Run(mock.Anything).Return(nil).Once()
		worker.EXPECT().RunOnce(mock.Anything).Return(nil).Once()
		workerCmd := newJobsWorkerCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsWorkerRunner, error) { return worker, nil },
		)
		require.NoError(t, workerCmd.ExecuteContext(t.Context()))
		require.NoError(t, workerCmd.Flags().Set("once", "true"))
		require.NoError(t, workerCmd.ExecuteContext(t.Context()))
		workerCmd = newJobsWorkerCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsWorkerRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, workerCmd.ExecuteContext(t.Context()), resolverErr)
		scheduler := newMockjobsSchedulerRunner(t)
		scheduler.EXPECT().EnqueueDue(mock.Anything).Return(1, nil)
		schedulerCmd := newJobsEnqueueDueCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsSchedulerRunner, error) { return scheduler, nil },
		)
		require.NoError(t, schedulerCmd.ExecuteContext(t.Context()))
		schedulerCmd = newJobsEnqueueDueCmdWithResolver(
			dig.New(),
			func(*cobra.Command, *dig.Container) (jobsSchedulerRunner, error) { return nil, resolverErr },
		)
		require.ErrorIs(t, schedulerCmd.ExecuteContext(t.Context()), resolverErr)
		require.NoError(t, primeFinanceJobs(nil))
		require.Error(t, primeFinanceJobs(dig.New()))
		_, err := resolveJobsWorkerAfterSetup(dig.New())
		require.Error(t, err)
		_, err = resolveJobsSchedulerAfterSetup(dig.New())
		require.Error(t, err)
	})

	t.Run("user administration creates lists and changes financial operators", func(t *testing.T) {
		store, hasher := makeUserDeps(t)
		username, password := fake.Internet().User(), fake.Internet().Password()
		var output bytes.Buffer
		require.NoError(
			t,
			runUserAdd(
				t.Context(),
				userAddCmdDeps{Store: store, Hasher: hasher},
				userAddParams{Username: username, Password: password},
				&output,
			),
		)
		require.Contains(t, output.String(), username)
		require.Error(
			t,
			runUserAdd(
				t.Context(),
				userAddCmdDeps{Store: store, Hasher: hasher},
				userAddParams{Username: username, Password: fake.Internet().Password()},
				&bytes.Buffer{},
			),
		)
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
}
