package app

import (
	"errors"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserDirectory(t *testing.T) {
	fake := faker.New()

	t.Run("requires auth user lookup", func(t *testing.T) {
		_, err := NewUserDirectory(nil)
		require.ErrorContains(t, err, "auth user lookup is required")
	})

	t.Run("looks up usernames and tolerates missing users", func(t *testing.T) {
		lookup := NewMockAuthUserLookup(t)
		directory, err := NewUserDirectory(lookup)
		require.NoError(t, err)
		userID := fake.UUID().V4()
		username := "user-" + fake.Internet().User()
		missingUserID := fake.UUID().V4()
		lookup.EXPECT().CurrentUser(mock.Anything, userID).Return(&auth.UserInfo{ID: userID, Username: username}, nil)
		lookup.EXPECT().CurrentUser(mock.Anything, missingUserID).Return(nil, auth.ErrUserNotFound)

		actualUsername, found, err := directory.LookupUsername(t.Context(), userID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, username, actualUsername)

		actualUsername, found, err = directory.LookupUsername(t.Context(), missingUserID)
		require.NoError(t, err)
		require.False(t, found)
		require.Empty(t, actualUsername)
	})

	t.Run("returns unexpected lookup errors", func(t *testing.T) {
		lookup := NewMockAuthUserLookup(t)
		directory, err := NewUserDirectory(lookup)
		require.NoError(t, err)
		lookup.EXPECT().CurrentUser(mock.Anything, mock.Anything).Return(nil, errors.New(fake.Lorem().Sentence(3)))

		_, _, err = directory.LookupUsername(t.Context(), fake.UUID().V4())
		require.ErrorContains(t, err, "lookup auth user")
	})
}
