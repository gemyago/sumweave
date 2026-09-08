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

	t.Run(
		"root AGENTS documents PostgreSQL bootstrap and standard local backend workflow",
		func(t *testing.T) {
			doc := readFile(resolvePath("..", "..", "..", "..", "AGENTS.md"))

			assert.Contains(t, doc, "make postgres-bootstrap")
			assert.Contains(t, doc, "start-all")
			assert.Contains(t, doc, "pm2 start ecosystem.config.js")
			assert.Contains(t, doc, "`api`, `worker`, and `ui`")
			assert.Contains(t, doc, "pm2 start|stop|restart|delete backend")
		},
	)

	t.Run("backend AGENTS keeps local PostgreSQL bootstrap guidance and split commands", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "AGENTS.md"))

		assert.Contains(t, doc, "sumweave start-all")
		assert.Contains(t, doc, "make postgres-bootstrap")
		assert.Contains(t, doc, "sumweave start")
		assert.Contains(t, doc, "sumweave jobs worker")
		assert.Contains(t, doc, "sumweave jobs enqueue-due")
		assert.Contains(t, doc, "`backend` namespace as `api` and `worker`")
		assert.Contains(t, doc, "pm2 start|stop|restart|delete backend")
	})

	t.Run("backend architecture doc describes PM2 split mode and start-all diagnostics", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "doc", "architecture.md"))

		assert.Contains(t, doc, "sumweave start-all")
		assert.Contains(t, doc, "diagnostic entrypoint")
		assert.Contains(t, doc, "local PM2")
		assert.Contains(t, doc, "sumweave jobs worker")
		assert.Contains(t, doc, "sumweave jobs enqueue-due")
	})

	t.Run("repo architecture doc keeps db-migrate before start-all guidance", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "..", "..", "docs", "ARCHITECTURE.md"))

		assert.Contains(t, doc, "sumweave db-migrate")
		assert.Contains(t, doc, "sumweave start-all")
	})
}
