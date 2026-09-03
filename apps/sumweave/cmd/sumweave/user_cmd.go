package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/wireup"
	"github.com/spf13/cobra"
)

type userAddParams struct {
	Username    string
	Password    string
	IfNotExists bool
}

type userChangePasswordParams struct {
	Username string
	Password string
}

const userCommandName = "user"

type userAddCmdDeps struct {
	Store  *auth.UserStore
	Hasher *auth.Argon2idHasher
}

type userListCmdDeps struct {
	Store *auth.UserStore
}

type userChangePasswordCmdDeps struct {
	Store  *auth.UserStore
	Hasher *auth.Argon2idHasher
}

type userCommandRuntime struct {
	store  *auth.UserStore
	hasher *auth.Argon2idHasher
	close  func() error
}

func runUserAdd(
	ctx context.Context,
	deps userAddCmdDeps,
	params userAddParams,
	out io.Writer,
) error {
	hash, err := deps.Hasher.Hash(params.Password)
	if err != nil { // coverage-ignore // crypto/rand failure is not practically testable
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := deps.Store.Create(ctx, auth.CreateUserParams{
		Username:     params.Username,
		PasswordHash: hash,
	})
	if err != nil {
		if params.IfNotExists && errors.Is(err, auth.ErrUsernameExists) {
			_, writeErr := fmt.Fprintf(out, "User already exists: username=%s\n", params.Username)
			return writeErr
		}
		return fmt.Errorf("create user: %w", err)
	}

	_, err = fmt.Fprintf(out, "User created: id=%s username=%s\n", user.ID, user.Username)
	return err
}

func runUserList(ctx context.Context, deps userListCmdDeps, out io.Writer) error {
	users, err := deps.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tUsername\tCreatedAt")
	for _, u := range users {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", u.ID, u.Username, u.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return w.Flush()
}

func runUserChangePassword(
	ctx context.Context,
	deps userChangePasswordCmdDeps,
	params userChangePasswordParams,
	out io.Writer,
) error {
	user, err := deps.Store.GetByUsername(ctx, params.Username)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	hash, err := deps.Hasher.Hash(params.Password)
	if err != nil { // coverage-ignore // crypto/rand failure is not practically testable
		return fmt.Errorf("hash password: %w", err)
	}

	if err = deps.Store.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_, err = fmt.Fprintf(out, "Password updated for user: username=%s\n", user.Username)
	return err
}

func newUserAddCmd() *cobra.Command { // coverage-ignore
	var params userAddParams

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new user",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtime, err := resolveUserCommandRuntime(cmd)
			if err != nil {
				return err
			}
			defer func() { err = closeUserCommandRuntime(err, runtime) }()
			return runUserAdd(
				cmd.Context(),
				userAddCmdDeps{Store: runtime.store, Hasher: runtime.hasher},
				params,
				cmd.OutOrStdout(),
			)
		},
	}
	cmd.Flags().StringVar(&params.Username, "username", "", "Username for the new user")
	cmd.Flags().StringVar(&params.Password, "password", "", "Password for the new user")
	cmd.Flags().BoolVar(
		&params.IfNotExists,
		"if-not-exists",
		false,
		"Succeed without changing the password when the username already exists",
	)
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserListCmd() *cobra.Command { // coverage-ignore
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtime, err := resolveUserCommandRuntime(cmd)
			if err != nil {
				return err
			}
			defer func() { err = closeUserCommandRuntime(err, runtime) }()
			return runUserList(cmd.Context(), userListCmdDeps{Store: runtime.store}, cmd.OutOrStdout())
		},
	}
	return cmd
}

func newUserChangePasswordCmd() *cobra.Command { // coverage-ignore
	var params userChangePasswordParams

	cmd := &cobra.Command{
		Use:   "change-password",
		Short: "Change a user's password",
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtime, err := resolveUserCommandRuntime(cmd)
			if err != nil {
				return err
			}
			defer func() { err = closeUserCommandRuntime(err, runtime) }()
			return runUserChangePassword(
				cmd.Context(),
				userChangePasswordCmdDeps{Store: runtime.store, Hasher: runtime.hasher},
				params,
				cmd.OutOrStdout(),
			)
		},
	}
	cmd.Flags().StringVar(&params.Username, "username", "", "Username of the user")
	cmd.Flags().StringVar(&params.Password, "password", "", "New password")
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserCmd() *cobra.Command { // coverage-ignore
	cmd := &cobra.Command{
		Use:   userCommandName,
		Short: "Manage users",
	}
	cmd.AddCommand(
		newUserAddCmd(),
		newUserListCmd(),
		newUserChangePasswordCmd(),
	)
	return cmd
}

func resolveUserCommandRuntime(cmd *cobra.Command) (userCommandRuntime, error) { // coverage-ignore
	environment, err := commandEnvironmentFromRoot(cmd.Root())
	if err != nil {
		return userCommandRuntime{}, err
	}
	root, err := wireup.BuildUsers(wireup.UsersOptions{Environment: environment})
	if err != nil {
		return userCommandRuntime{}, fmt.Errorf("build users root: %w", err)
	}
	return userCommandRuntime{store: root.Store, hasher: root.Hasher, close: root.Close}, nil
}

func closeUserCommandRuntime(commandErr error, runtime userCommandRuntime) error { // coverage-ignore
	if runtime.close == nil {
		return commandErr
	}
	if closeErr := runtime.close(); closeErr != nil {
		return errors.Join(commandErr, fmt.Errorf("close users root: %w", closeErr))
	}
	return commandErr
}
