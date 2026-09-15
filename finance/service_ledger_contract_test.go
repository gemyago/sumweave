package finance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTransactionListLimit(t *testing.T) {
	t.Run("defaults and bounds transaction pages", func(t *testing.T) {
		for _, testCase := range []struct {
			limit     int64
			want      int64
			wantError bool
		}{
			{limit: 0, want: DefaultTransactionListLimit},
			{limit: 1, want: 1},
			{limit: MaxTransactionListLimit, want: MaxTransactionListLimit},
			{limit: -1, wantError: true},
			{limit: MaxTransactionListLimit + 1, wantError: true},
		} {
			limit, err := NormalizeTransactionListLimit(testCase.limit)
			if testCase.wantError {
				require.ErrorIs(t, err, ErrInvalidTransactionListLimit)
				continue
			}
			require.NoError(t, err)
			require.Equal(t, testCase.want, limit)
		}
	})
}
