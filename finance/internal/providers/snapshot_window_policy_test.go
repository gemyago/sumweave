package providers

import (
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestedWindowSnapshotPolicy(t *testing.T) {
	t.Run("returns the requested window unchanged", func(t *testing.T) {
		fake := faker.New()
		requestedWindow := makeRandomProviderSyncWindow(fake)
		policy := NewRequestedWindowSnapshotPolicy()

		snapshotWindow, err := policy.Determine(requestedWindow)
		require.NoError(t, err)
		assert.Equal(t, requestedWindow, snapshotWindow)
	})
}
