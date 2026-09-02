//go:build postgres_test

package wireup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresCommandRoots(t *testing.T) {
	t.Chdir("../..")

	t.Run("builds user administration capabilities against the prepared application schema", func(t *testing.T) {
		root, err := BuildUsers(UsersOptions{Environment: "test"})
		require.NoError(t, err)
		require.NotNil(t, root.Store)
		require.NotNil(t, root.Hasher)
		require.NoError(t, root.Close())
	})

	t.Run("builds finance fixture storage against the prepared application schema", func(t *testing.T) {
		root, err := BuildFinanceFixtures(FinanceFixturesOptions{Environment: "test"})
		require.NoError(t, err)
		require.NotNil(t, root.Database)
		require.NotNil(t, root.JobsStore)
		require.NotEmpty(t, root.JWTSigningKey)
		require.NotEmpty(t, root.MonobankBaseURL)
		require.NoError(t, root.Close())
	})
}
