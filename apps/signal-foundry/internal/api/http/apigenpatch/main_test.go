package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchGeneratedValidatorsContent(t *testing.T) {
	t.Run("patches generated helper for map validators and stays idempotent", func(t *testing.T) {
		fixture := strings.Join([]string{
			"package internal",
			"",
			"import (",
			"\t\"fmt\"",
			"\t\"regexp\"",
			")",
			"",
			"var ErrValueRequired = fmt.Errorf(\"required\")",
			"var _ = regexp.MustCompile(\".*\")",
			"",
			generatedEnsureNonDefaultSnippet,
			"func use() error {",
			"\treturn EnsureNonDefault[map[string]string](map[string]string{})",
			"}",
		}, "\n")

		patched, changed, err := patchGeneratedValidatorsContent(fixture)
		require.NoError(t, err)
		require.True(t, changed)
		require.Contains(t, patched, "\t\"reflect\"")
		require.Contains(t, patched, "func EnsureNonDefault[TTargetVal any](val TTargetVal) error")

		repatched, changed, err := patchGeneratedValidatorsContent(patched)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, patched, repatched)

		tempDir := t.TempDir()
		goModPath := filepath.Join(tempDir, "go.mod")
		require.NoError(
			t,
			os.WriteFile(goModPath, []byte("module example.com/generated\n\ngo 1.26.0\n"), 0o644),
		)
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "validators.go"), []byte(patched), 0o644))

		cmd := exec.Command("go", "test", "./...")
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("fails when generator layout drifts", func(t *testing.T) {
		_, _, err := patchGeneratedValidatorsContent("package internal\n")
		require.ErrorContains(t, err, "generated EnsureNonDefault helper not found")
	})

	t.Run("fails when generated import block drifts", func(t *testing.T) {
		fixture := strings.Join([]string{
			"package internal",
			"",
			"import (",
			"\t\"fmt\"",
			")",
			"",
			generatedEnsureNonDefaultSnippet,
		}, "\n")

		_, _, err := patchGeneratedValidatorsContent(fixture)
		require.ErrorContains(t, err, "generated import block not found")
	})
}

func TestRun(t *testing.T) {
	t.Run("returns usage error for wrong arg count", func(t *testing.T) {
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"apigenpatch", "extra"}, stderr)

		require.Equal(t, 2, exitCode)
		require.Contains(t, stderr.String(), "usage: apigenpatch")
	})

	t.Run("returns patch error when generated validators file is missing", func(t *testing.T) {
		stderr := &bytes.Buffer{}
		t.Chdir(t.TempDir())

		exitCode := run([]string{"apigenpatch"}, stderr)

		require.Equal(t, 1, exitCode)
		require.Contains(t, stderr.String(), "stat file")
	})

	t.Run("patches generated validators file", func(t *testing.T) {
		tempDir := t.TempDir()
		validatorsPath := filepath.Join(tempDir, generatedValidatorsPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(validatorsPath), 0o755))
		t.Chdir(tempDir)
		require.NoError(t, os.WriteFile(validatorsPath, []byte(strings.Join([]string{
			"package internal",
			"",
			"import (",
			"\t\"fmt\"",
			"\t\"regexp\"",
			")",
			"",
			generatedEnsureNonDefaultSnippet,
		}, "\n")), 0o600))

		stderr := &bytes.Buffer{}
		exitCode := run([]string{"apigenpatch"}, stderr)

		require.Equal(t, 0, exitCode)
		require.Empty(t, stderr.String())

		content, err := os.ReadFile(validatorsPath)
		require.NoError(t, err)
		require.Contains(t, string(content), "\t\"reflect\"")
		require.Contains(t, string(content), "func EnsureNonDefault[TTargetVal any](val TTargetVal) error")
	})
}

func TestPatchGeneratedValidators(t *testing.T) {
	t.Run("returns stat error for missing file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		err := patchGeneratedValidators()

		require.ErrorContains(t, err, "stat file")
	})

	t.Run("returns read error when validators path is a directory", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		validatorsPath := filepath.Join(tempDir, generatedValidatorsPath)
		require.NoError(t, os.MkdirAll(validatorsPath, 0o755))

		err := patchGeneratedValidators()

		require.ErrorContains(t, err, "read file")
	})

	t.Run("returns nil when validators are already patched", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		validatorsPath := filepath.Join(tempDir, generatedValidatorsPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(validatorsPath), 0o755))
		require.NoError(t, os.WriteFile(validatorsPath, []byte(strings.Join([]string{
			"package internal",
			"",
			"import (",
			"\t\"fmt\"",
			"\t\"reflect\"",
			"\t\"regexp\"",
			")",
			"",
			patchedEnsureNonDefaultSnippet,
		}, "\n")), 0o600))

		require.NoError(t, patchGeneratedValidators())
	})

	t.Run("returns write error when target is read only", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		validatorsPath := filepath.Join(tempDir, generatedValidatorsPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(validatorsPath), 0o755))
		require.NoError(t, os.WriteFile(validatorsPath, []byte(strings.Join([]string{
			"package internal",
			"",
			"import (",
			"\t\"fmt\"",
			"\t\"regexp\"",
			")",
			"",
			generatedEnsureNonDefaultSnippet,
		}, "\n")), 0o400))

		err := patchGeneratedValidators()

		require.ErrorContains(t, err, "write file")
	})

	t.Run("returns patch error for unexpected helper layout", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		validatorsPath := filepath.Join(tempDir, generatedValidatorsPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(validatorsPath), 0o755))
		require.NoError(t, os.WriteFile(validatorsPath, []byte("package internal\n"), 0o600))

		err := patchGeneratedValidators()

		require.ErrorContains(t, err, "generated EnsureNonDefault helper not found")
	})
}
