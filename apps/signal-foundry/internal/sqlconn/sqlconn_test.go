package sqlconn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("rejects empty dsn", func(t *testing.T) {
		db, err := Open("   ")
		require.Error(t, err)
		require.Nil(t, db)
		require.EqualError(t, err, "database dsn is required")
	})
}
