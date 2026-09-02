//go:build postgres_test

package sessions

import (
	"os"
	"testing"

	"github.com/gemyago/sumweave/runtime/internal/gormsumweave"
	"github.com/stretchr/testify/require"
)

const postgresTestTablePrefix = "sumweave_runtime_"

func postgresTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SUMWEAVE_POSTGRES_TEST_DSN")
	require.NotEmpty(t, dsn, "SUMWEAVE_POSTGRES_TEST_DSN is required for postgres_test")
	return dsn
}

func postgresTestTablesOpts() gormsumweave.GormSumweaveTablesOpts {
	return gormsumweave.GormSumweaveTablesOpts{TablePrefix: postgresTestTablePrefix}
}
