//go:build postgres_test

package main

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/sqlconn"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestApplicationCommandsPostgres(t *testing.T) {
	makeUserDeps := func(t *testing.T) (*auth.UserStore, *auth.Argon2idHasher) {
		t.Helper()
		dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
		require.NotEmpty(t, dsn)
		db, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		store, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB:       db,
			DatabaseDSN: dsn,
			TablePrefix: "sumweave_auth_",
			IDGen:       ident.NewDefaultGenerator(),
			Logger:      slog.New(slog.DiscardHandler),
		})
		require.NoError(t, err)
		return store, auth.NewArgon2idHasherWithParams(
			auth.Argon2idHasherParams{Memory: 8, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32},
		)
	}
	makeUsername := func(fake faker.Faker) string {
		return "user-" + fake.UUID().V4()
	}

	t.Run("user administration creates lists and changes scoped users", func(t *testing.T) {
		fake := faker.New()
		store, hasher := makeUserDeps(t)
		username, password := makeUsername(fake), fake.Internet().Password()
		makeAddParams := func(candidatePassword string, ifNotExists bool) userAddParams {
			return userAddParams{Username: username, Password: candidatePassword, IfNotExists: ifNotExists}
		}
		var output bytes.Buffer
		require.NoError(t, runUserAdd(
			t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, makeAddParams(password, false), &output,
		))
		require.Contains(t, output.String(), username)
		require.Error(t, runUserAdd(
			t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, makeAddParams(fake.Internet().Password(), false), &bytes.Buffer{},
		))
		var ensureOutput bytes.Buffer
		require.NoError(t, runUserAdd(
			t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, makeAddParams(fake.Internet().Password(), true), &ensureOutput,
		))
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
		require.NoError(t, runUserChangePassword(
			t.Context(), userChangePasswordCmdDeps{Store: store, Hasher: hasher},
			userChangePasswordParams{Username: username, Password: newPassword}, &bytes.Buffer{},
		))
		user, err := store.GetByUsername(t.Context(), username)
		require.NoError(t, err)
		ok, err := hasher.Verify(newPassword, user.PasswordHash)
		require.NoError(t, err)
		require.True(t, ok)
		require.Error(t, runUserChangePassword(
			t.Context(), userChangePasswordCmdDeps{Store: store, Hasher: hasher},
			userChangePasswordParams{Username: makeUsername(fake), Password: newPassword}, &bytes.Buffer{},
		))
	})

	t.Run("user commands resolve prepared administration capabilities", func(t *testing.T) {
		fake := faker.New()
		store, hasher := makeUserDeps(t)
		username, password := makeUsername(fake), fake.Internet().Password()
		resolverErr := errors.New(fake.Lorem().Sentence(3))
		resolver := func(*cobra.Command) (userCommandRuntime, error) {
			return userCommandRuntime{store: store, hasher: hasher}, nil
		}
		newCommand := func(t *testing.T, args ...string) *cobra.Command {
			t.Helper()
			root := newRootCmd()
			root.AddCommand(newUserCmdWithResolver(resolver))
			root.SetArgs(args)
			return root
		}

		require.NoError(t, newCommand(t, "user", "add", "--username", username, "--password", password).ExecuteContext(t.Context()))
		require.NoError(t, newCommand(t, "user", "list").ExecuteContext(t.Context()))
		require.NoError(t, newCommand(
			t, "user", "change-password", "--username", username, "--password", fake.Internet().Password(),
		).ExecuteContext(t.Context()))

		failingCommand := newRootCmd()
		failingCommand.AddCommand(newUserCmdWithResolver(
			func(*cobra.Command) (userCommandRuntime, error) { return userCommandRuntime{}, resolverErr },
		))
		failingCommand.SetArgs([]string{"user", "list"})
		require.ErrorIs(t, failingCommand.ExecuteContext(t.Context()), resolverErr)
	})
}
