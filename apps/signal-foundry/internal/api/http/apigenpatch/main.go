package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const generatedValidatorsPath = "v1routes/internal/validators.go"

const generatedImportSnippet = "\t\"fmt\"\n\t\"regexp\"\n"

const patchedImportSnippet = "\t\"fmt\"\n\t\"reflect\"\n\t\"regexp\"\n"

const generatedEnsureNonDefaultSnippet = `func EnsureNonDefault[TTargetVal comparable](val TTargetVal) error {
	var empty TTargetVal
	if val == empty {
		return fmt.Errorf("provided value %v is default for given type and considered empty: %w", val, ErrValueRequired)
	}
	return nil
}
`

const patchedEnsureNonDefaultSnippet = `func EnsureNonDefault[TTargetVal any](val TTargetVal) error {
	if reflect.ValueOf(val).IsZero() {
		return fmt.Errorf("provided value %v is default for given type and considered empty: %w", val, ErrValueRequired)
	}
	return nil
}
`

func main() {
	os.Exit(run(os.Args, os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintf(stderr, "usage: %s\n", args[0])
		return 2
	}

	if err := patchGeneratedValidators(); err != nil {
		_, _ = fmt.Fprintf(stderr, "patch generated validators: %v\n", err)
		return 1
	}

	return 0
}

func patchGeneratedValidators() error {
	fileInfo, err := os.Stat(generatedValidatorsPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	content, err := os.ReadFile(generatedValidatorsPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	patched, changed, err := patchGeneratedValidatorsContent(string(content))
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	fileHandle, err := os.OpenFile(generatedValidatorsPath, os.O_WRONLY|os.O_TRUNC, fileInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	defer fileHandle.Close()

	if _, writeErr := fileHandle.WriteString(patched); writeErr != nil {
		return fmt.Errorf("write file: %w", writeErr)
	}

	return nil
}

func patchGeneratedValidatorsContent(content string) (string, bool, error) {
	if strings.Contains(content, patchedEnsureNonDefaultSnippet) {
		return content, false, nil
	}

	if !strings.Contains(content, generatedEnsureNonDefaultSnippet) {
		return "", false, errors.New("generated EnsureNonDefault helper not found")
	}

	patched := strings.Replace(content, generatedEnsureNonDefaultSnippet, patchedEnsureNonDefaultSnippet, 1)
	if strings.Contains(patched, "\t\"reflect\"\n") {
		return patched, true, nil
	}

	if !strings.Contains(patched, generatedImportSnippet) {
		return "", false, errors.New("generated import block not found")
	}

	patched = strings.Replace(patched, generatedImportSnippet, patchedImportSnippet, 1)

	return patched, true, nil
}
