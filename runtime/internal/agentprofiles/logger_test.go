package agentprofiles

import (
	"log/slog"
	"testing"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.DiscardHandler).With("test", t.Name())
}
