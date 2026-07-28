package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
)

// AuthUserLookup retrieves safe auth user information for app-layer consumers.
type AuthUserLookup interface {
	CurrentUser(context.Context, string) (*auth.UserInfo, error)
}

// UserDirectory exposes safe user display data to application consumers.
type UserDirectory struct {
	authUsers AuthUserLookup
}

func NewUserDirectory(authUsers AuthUserLookup) (*UserDirectory, error) {
	if authUsers == nil {
		return nil, errors.New("auth user lookup is required")
	}
	return &UserDirectory{authUsers: authUsers}, nil
}

func (d *UserDirectory) LookupUsername(ctx context.Context, userID string) (string, bool, error) {
	user, err := d.authUsers.CurrentUser(ctx, userID)
	if errors.Is(err, auth.ErrUserNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup auth user: %w", err)
	}
	if user == nil {
		return "", false, nil
	}
	return user.Username, true, nil
}
