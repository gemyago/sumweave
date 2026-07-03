package finance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootServiceCallerAudit(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), ".."))

	t.Run("repo product code no longer imports or constructs finance.Service", func(t *testing.T) {
		_, statErr := os.Stat(filepath.Join(repoRoot, "finance", "service.go"))
		require.ErrorIs(t, statErr, os.ErrNotExist)

		for _, relativePath := range []string{"apps", "finance", "runtime"} {
			root := filepath.Join(repoRoot, relativePath)
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				require.NoError(t, err)
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				contents, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				text := string(contents)
				assert.NotContains(t, text, "finance.NewService(", path)
				assert.NotContains(t, text, "finance.Service", path)
				if filepath.Dir(path) == filepath.Join(repoRoot, "finance") {
					assert.NotContains(t, text, "type Service struct", path)
					assert.NotContains(t, text, "func NewService(", path)
				}
				return nil
			})
			require.NoError(t, err)
		}
	})
}
