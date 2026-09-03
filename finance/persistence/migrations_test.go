//go:build postgres_test

package persistence

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	database := openTestDatabase(t)

	require.NoError(t, NewMigrator(database).Migrate(t.Context()))
}
