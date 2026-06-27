package persistence

import (
	"fmt"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestOpenDatabase(t *testing.T) {
	t.Run("rejects missing dsn", func(t *testing.T) {
		_, err := OpenDatabase("   ")
		require.Error(t, err)
	})

	t.Run("opens sqlite database", func(t *testing.T) {
		database, err := OpenDatabase(
			fmt.Sprintf("file:%s?mode=memory&cache=shared", "database-"+faker.New().UUID().V4()),
		)
		require.NoError(t, err)
		require.NotNil(t, database)
		require.NotNil(t, database.db)
	})

	t.Run("surfaces database open failures", func(t *testing.T) {
		_, err := OpenDatabase(fmt.Sprintf("%s/nope/test.sqlite", t.TempDir()))
		require.Error(t, err)
	})
}
