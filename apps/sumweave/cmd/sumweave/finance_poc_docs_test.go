package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancialPOCDocumentationAndIgnoreRules(t *testing.T) {
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

	t.Run("enable banking PKO doc covers setup and AI-agent constraints", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "doc", "financial-poc", "enable-banking-pko.md"))

		assert.Contains(t, doc, "sandbox")
		assert.Contains(t, doc, "production")
		assert.Contains(t, doc, "private key")
		assert.Contains(t, doc, "outside the repo")
		assert.Contains(t, doc, "JWT `kid`")
		assert.Contains(t, doc, "ENABLE_BANKING_APP_ID")
		assert.Contains(t, doc, "ENABLE_BANKING_PRIVATE_KEY_PATH")
		assert.Contains(t, doc, "ENABLE_BANKING_BASE_URL")
		assert.Contains(t, doc, "aspsps --country PL")
		assert.Contains(t, doc, "PKO")
		assert.Contains(t, doc, "redirect")
		assert.Contains(t, doc, "HTTPS localhost callback")
		assert.Contains(t, doc, "trusted certificate")
		assert.Contains(t, doc, "callback-cert-file")
		assert.Contains(t, doc, "callback-key-file")
		assert.Contains(t, doc, "must be provided together")
		assert.Contains(t, doc, "self-signed")
		assert.Contains(t, doc, "browser may warn")
		assert.Contains(t, doc, "tunnel")
		assert.Contains(t, doc, "manual")
		assert.Contains(t, doc, "linked account")
		assert.Contains(t, doc, "restricted")
		assert.Contains(t, doc, "AI agent")
		assert.Contains(t, doc, "cannot complete PKO strong customer authentication")
		assert.Contains(t, doc, "start-auth")
		assert.Contains(t, doc, "finish-session --auth-file")
		assert.Contains(t, doc, "--code")
		assert.Contains(t, doc, "--state")
		assert.Contains(t, doc, "../../data/financial-poc/")
	})

	t.Run("monobank doc covers token setup, limits, and live-call warnings", func(t *testing.T) {
		doc := readFile(resolvePath("..", "..", "doc", "financial-poc", "monobank.md"))

		assert.Contains(t, doc, "personal token")
		assert.Contains(t, doc, "outside the repo")
		assert.Contains(t, doc, "MONOBANK_TOKEN")
		assert.Contains(t, doc, "MONOBANK_BASE_URL")
		assert.Contains(t, doc, "monobank accounts --json")
		assert.Contains(t, doc, "monobank transactions")
		assert.Contains(t, doc, "--account 0")
		assert.Contains(t, doc, "60-second")
		assert.Contains(t, doc, "31 days plus 1 hour")
		assert.Contains(t, doc, "default account `0`")
		assert.Contains(t, doc, "AI agent")
		assert.Contains(t, doc, "repeated live calls can hit monobank rate limits")
		assert.Contains(t, doc, "../../data/financial-poc/")
	})

	t.Run("gitignore covers financial POC artifacts", func(t *testing.T) {
		gitignore := readFile(resolvePath("..", "..", "..", "..", ".gitignore"))

		assert.Contains(t, gitignore, "data/financial-poc/")
		assert.Contains(t, gitignore, "*.enable-banking-session.json")
		assert.Contains(t, gitignore, "*.monobank-transactions.json")
		assert.Contains(t, gitignore, "*.enable-banking-transactions.json")
	})
}
