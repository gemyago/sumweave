package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartAllDocumentation(t *testing.T) {
	resolvePath := func(parts ...string) string {
		t.Helper()
		_, currentFile, _, ok := runtime.Caller(0)
		require.True(t, ok)
		pathParts := append([]string{filepath.Dir(currentFile)}, parts...)
		return filepath.Clean(filepath.Join(pathParts...))
	}

	readFile := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(content)
	}

	t.Run("root AGENTS documents db-migrate plus start-all as standard local backend workflow", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "..", "..", "AGENTS.md"))

		assert.Contains(t, doc, "db-migrate")
		assert.Contains(t, doc, "start-all")
		assert.Contains(t, doc, "pm2 start ecosystem.config.js")
	})

	t.Run("backend AGENTS keeps local start-all guidance and split commands", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "AGENTS.md"))

		assert.Contains(t, doc, "signal-foundry start-all")
		assert.Contains(t, doc, "signal-foundry db-migrate")
		assert.Contains(t, doc, "signal-foundry start")
		assert.Contains(t, doc, "signal-foundry jobs worker")
		assert.Contains(t, doc, "signal-foundry jobs enqueue-due")
	})

	t.Run("backend architecture doc describes start-all as the normal local mode", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "doc", "architecture.md"))

		assert.Contains(t, doc, "signal-foundry start-all")
		assert.Contains(t, doc, "standard local backend workflow")
		assert.Contains(t, doc, "signal-foundry jobs worker")
		assert.Contains(t, doc, "signal-foundry jobs enqueue-due")
	})

	t.Run("repo architecture doc keeps db-migrate before start-all guidance", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "..", "..", "docs", "ARCHITECTURE.md"))

		assert.Contains(t, doc, "signal-foundry db-migrate")
		assert.Contains(t, doc, "signal-foundry start-all")
	})
}
