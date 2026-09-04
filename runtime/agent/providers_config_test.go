//go:build !release

package agent

import (
	"testing"

	"github.com/gemyago/sumweave/runtime/internal"
	"github.com/stretchr/testify/require"
)

func TestNewFileProvidersConfigService(t *testing.T) {
	rootTestLogger := internal.RootTestLogger()
	t.Run("creates service with valid base dir", func(t *testing.T) {
		svc, err := NewFileProvidersConfigService(t.TempDir(), rootTestLogger)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})
}

func TestNewDatabaseProvidersConfigService(t *testing.T) {
	logger := internal.RootTestLogger()
	svc, err := NewDatabaseProvidersConfigService(testDatabaseDSN(t), logger, testDatabaseTablePrefix)
	require.NoError(t, err)
	require.NotNil(t, svc)
}
