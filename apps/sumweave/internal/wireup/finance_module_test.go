package wireup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildFinanceModule(t *testing.T) {
	t.Run("passes native dependency validation through without persistence", func(t *testing.T) {
		_, err := buildFinanceModule(financeModuleBuildDeps{})
		require.Error(t, err)
	})
}
