package appdispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportBoundary(t *testing.T) {
	for _, root := range []string{
		filepath.Clean(filepath.Join("..", "..", "..", "..", "finance")),
		filepath.Clean(filepath.Join("..", "..", "..", "..", "runtime")),
	} {
		require.NoError(t, assertNoWatermillImports(root))
	}
}

func assertNoWatermillImports(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(contents), "ThreeDotsLabs/watermill") {
			return fmt.Errorf("watermill import leaked into %s", path)
		}
		return nil
	})
}
