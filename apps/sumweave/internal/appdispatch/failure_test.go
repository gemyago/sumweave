package appdispatch

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusinessFailure(t *testing.T) {
	t.Run("acknowledges only explicit business failures", func(t *testing.T) {
		cause := errors.New("domain failure")
		err := NewBusinessFailure(cause, "domain_failure", "domain failed", "safe details")
		failure, ok := BusinessFailureFrom(err)
		require.True(t, ok)
		assert.Equal(t, "domain_failure", failure.Code)
		require.ErrorIs(t, err, cause)
		require.NoError(t, NewBusinessFailure(nil, "", "", ""))
		_, ok = BusinessFailureFrom(cause)
		assert.False(t, ok)
	})
}
