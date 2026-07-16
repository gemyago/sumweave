package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/auth"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/sqlconn"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func TestUserCmd(t *testing.T) {
	fake := faker.New()

	type userCmdDeps struct {
		store     *auth.UserStore
		hasher    *auth.Argon2idHasher
		container *dig.Container
	}

	newFastHasher := func() *auth.Argon2idHasher {
		return auth.NewArgon2idHasherWithParams(auth.Argon2idHasherParams{
			Memory:      8,
			Time:        1,
			Parallelism: 1,
			SaltLen:     16,
			KeyLen:      32,
		})
	}

	makeUserCmdDeps := func(t *testing.T) userCmdDeps {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "users.sqlite")
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		store, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "test_auth_",
			IDGen: ident.NewDefaultGenerator(), Logger: slog.Default(),
		})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		return userCmdDeps{
			store:     store,
			hasher:    newFastHasher(),
			container: dig.New(),
		}
	}

	makeBrokenStore := func(t *testing.T) *auth.UserStore {
		t.Helper()
		dsn := filepath.Join(t.TempDir(), "users.sqlite")
		sqlDB, err := sqlconn.Open(dsn)
		require.NoError(t, err)
		store, err := auth.NewUserStore(auth.UserStoreDeps{
			SQLDB: sqlDB, DatabaseDSN: dsn, TablePrefix: "test_auth_",
			IDGen: ident.NewDefaultGenerator(), Logger: slog.Default(),
		})
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		return store
	}

	t.Run("newUserCmd", func(t *testing.T) {
		t.Run("has correct Use", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			cmd := newUserCmd(deps.container)
			assert.Equal(t, "user", cmd.Use)
		})

		t.Run("has add, list, change-password subcommands", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			cmd := newUserCmd(deps.container)
			names := make([]string, 0)
			for _, sub := range cmd.Commands() {
				names = append(names, sub.Name())
			}
			assert.Contains(t, names, "add")
			assert.Contains(t, names, "list")
			assert.Contains(t, names, "change-password")
		})
	})

	t.Run("runUserAdd", func(t *testing.T) {
		t.Run("creates user and prints ID", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher
			username := fake.Internet().User()
			password := fake.Internet().Password()

			var buf bytes.Buffer
			err := runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username,
				Password: password,
			}, &buf)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, username)

			// Verify user is actually stored
			user, getErr := store.GetByUsername(t.Context(), username)
			require.NoError(t, getErr)
			assert.Equal(t, username, user.Username)
		})

		t.Run("returns error for duplicate username", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher
			username := fake.Internet().User()
			password := fake.Internet().Password()

			var buf bytes.Buffer
			require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username,
				Password: password,
			}, &buf))

			var buf2 bytes.Buffer
			err := runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username,
				Password: fake.Internet().Password(),
			}, &buf2)
			require.ErrorIs(t, err, auth.ErrUsernameExists)
		})
	})

	t.Run("runUserList", func(t *testing.T) {
		t.Run("lists all created users", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher

			username1 := fake.Internet().User() + "1"
			username2 := fake.Internet().User() + "2"

			var buf bytes.Buffer
			require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username1,
				Password: fake.Internet().Password(),
			}, &buf))
			require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username2,
				Password: fake.Internet().Password(),
			}, &buf))

			var listBuf bytes.Buffer
			err := runUserList(t.Context(), userListCmdDeps{Store: store}, &listBuf)
			require.NoError(t, err)

			output := listBuf.String()
			assert.Contains(t, output, username1)
			assert.Contains(t, output, username2)
			assert.NotContains(t, output, "passwordHash", "password hash should not be printed")
		})

		t.Run("prints header even when no users", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store

			var listBuf bytes.Buffer
			err := runUserList(t.Context(), userListCmdDeps{Store: store}, &listBuf)
			require.NoError(t, err)

			output := listBuf.String()
			// Should at minimum print something (header or empty message)
			assert.NotEmpty(t, output)
		})

		t.Run("does not print password hashes or algorithm name", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher
			username := fake.Internet().User()
			password := fake.Internet().Password()

			var buf bytes.Buffer
			require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username,
				Password: password,
			}, &buf))

			var listBuf bytes.Buffer
			require.NoError(t, runUserList(t.Context(), userListCmdDeps{Store: store}, &listBuf))

			output := listBuf.String()
			user, err := store.GetByUsername(t.Context(), username)
			require.NoError(t, err)
			assert.NotContains(t, output, user.PasswordHash)
			assert.NotContains(t, output, "argon2id", "hash should not be in list output")
		})
	})

	t.Run("runUserChangePassword", func(t *testing.T) {
		t.Run("updates password for existing user", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher
			username := fake.Internet().User()
			oldPassword := fake.Internet().Password()
			newPassword := fake.Internet().Password()

			var buf bytes.Buffer
			require.NoError(t, runUserAdd(t.Context(), userAddCmdDeps{Store: store, Hasher: hasher}, userAddParams{
				Username: username,
				Password: oldPassword,
			}, &buf))

			var changeBuf bytes.Buffer
			cpDeps := userChangePasswordCmdDeps{Store: store, Hasher: hasher}
			err := runUserChangePassword(t.Context(), cpDeps, userChangePasswordParams{
				Username: username,
				Password: newPassword,
			}, &changeBuf)
			require.NoError(t, err)

			// Verify password changed
			user, getErr := store.GetByUsername(t.Context(), username)
			require.NoError(t, getErr)

			ok, verifyErr := hasher.Verify(newPassword, user.PasswordHash)
			require.NoError(t, verifyErr)
			assert.True(t, ok, "new password should verify")

			notOk, verifyOldErr := hasher.Verify(oldPassword, user.PasswordHash)
			require.NoError(t, verifyOldErr)
			assert.False(t, notOk, "old password should not verify")
		})

		t.Run("returns error for non-existent user", func(t *testing.T) {
			deps := makeUserCmdDeps(t)
			store := deps.store
			hasher := deps.hasher

			var buf bytes.Buffer
			cpDeps := userChangePasswordCmdDeps{Store: store, Hasher: hasher}
			err := runUserChangePassword(t.Context(), cpDeps, userChangePasswordParams{
				Username: fake.Internet().User(),
				Password: fake.Internet().Password(),
			}, &buf)
			require.ErrorIs(t, err, auth.ErrUserNotFound)
		})
	})

	t.Run("end-to-end via cobra command", func(t *testing.T) {
		t.Run("user add and user list work together", func(t *testing.T) {
			chdirModuleRoot(t)
			dataLayerDSN := filepath.Join(t.TempDir(), "data-layer.sqlite")
			agentRuntimeDSN := filepath.Join(t.TempDir(), "agent-runtime.sqlite")
			t.Setenv("APP_DATALAYER_DATABASE_DSN", dataLayerDSN)
			t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", agentRuntimeDSN)
			runDatabaseMigrateCommand(t)
			// -e test uses dataDir on disk under apps/signal-foundry; faker usernames can collide with
			// users left from earlier runs or other tests, so the name must be unique per run.
			username := fmt.Sprintf("%s_%d", fake.Internet().User(), time.Now().UnixNano())
			password := fake.Internet().Password()
			logf := testLogFile(t)

			// Each Execute builds an engine on the shared dig.Container; use a fresh command tree
			// per run so provider registration is not duplicated.
			rootAdd := setupCommands()
			rootAdd.SilenceUsage = true
			rootAdd.SilenceErrors = true
			var addBuf bytes.Buffer
			rootAdd.SetOut(&addBuf)
			rootAdd.SetArgs([]string{
				"user", "add", "-e", "test", "--username", username, "--password", password, "--logs-file", logf,
			})
			require.NoError(t, rootAdd.ExecuteContext(t.Context()))

			rootList := setupCommands()
			rootList.SilenceUsage = true
			rootList.SilenceErrors = true
			var listBuf bytes.Buffer
			rootList.SetOut(&listBuf)
			rootList.SetArgs([]string{"user", "list", "-e", "test", "--logs-file", logf})
			require.NoError(t, rootList.ExecuteContext(t.Context()))

			assert.Contains(t, listBuf.String(), username)
		})
	})

	t.Run("error paths", func(t *testing.T) {
		t.Run("runUserList propagates store.List error", func(t *testing.T) {
			store := makeBrokenStore(t)
			var buf bytes.Buffer
			err := runUserList(t.Context(), userListCmdDeps{Store: store}, &buf)
			require.Error(t, err)
			assert.ErrorContains(t, err, "list users")
		})
	})
}
