package finance_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleBoundary(t *testing.T) {
	t.Run("does not depend on runtime module", func(t *testing.T) {
		cmd := exec.Command("go", "list", "-deps", "./...")
		cmd.Dir = "."

		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
			if line == "" {
				continue
			}
			require.NotContains(t, line, "github.com/gemyago/sumweave/runtime")
		}
	})
}
