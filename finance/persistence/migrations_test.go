package persistence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	database := openTestDatabase(t)

	require.NoError(t, NewMigrator(database).Migrate(t.Context()))

	canceledContext, cancel := context.WithCancel(t.Context())
	cancel()
	err := NewMigrator(database).Migrate(canceledContext)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "auto-migrate finance schema")
}
