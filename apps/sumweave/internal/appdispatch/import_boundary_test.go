package appdispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
