//go:build postgres_test

package agent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const testDatabaseTablePrefix = "sumweave_runtime_"

func testDatabaseDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	require.NotEmpty(t, dsn, "SUMWEAVE_POSTGRES_TEST_DSN is required for postgres_test")
	return dsn
}
