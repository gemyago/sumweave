//go:build live

package live_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectRecentTradeProbeWindow(t *testing.T) {
	t.Parallel()

	t.Run("keeps the newest target rows in timestamp order", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
		rows := make([]recentTradeProbeRow, 0, 12)
		for idx := 0; idx < 12; idx++ {
			rows = append(rows, recentTradeProbeRow{
				Time: base.Add(time.Duration(11-idx) * time.Second).UnixMilli(),
			})
		}

		window, err := selectRecentTradeProbeWindow(rows)
		require.NoError(t, err)
		require.Equal(t, 12, window.AvailableCount)
		require.Equal(t, liveSmokeTradeProbeTargetRows, window.SelectedCount)
		require.Equal(t, base, window.EarliestAvailable)
		require.Equal(t, base.Add(11*time.Second), window.LatestAvailable)
		require.Equal(t, base.Add(2*time.Second), window.TimeRange.Start)
		require.Equal(
			t,
			base.Add(11*time.Second).Add(liveSmokeTradeRangePadding),
			window.TimeRange.End,
		)
	})

	t.Run("keeps the full window when fewer rows are available", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
		rows := []recentTradeProbeRow{
			{Time: base.Add(3 * time.Second).UnixMilli()},
			{Time: base.UnixMilli()},
			{Time: base.Add(2 * time.Second).UnixMilli()},
		}

		window, err := selectRecentTradeProbeWindow(rows)
		require.NoError(t, err)
		require.Equal(t, 3, window.AvailableCount)
		require.Equal(t, 3, window.SelectedCount)
		require.Equal(t, base, window.EarliestAvailable)
		require.Equal(t, base.Add(3*time.Second), window.LatestAvailable)
		require.Equal(t, base, window.TimeRange.Start)
		require.Equal(t, base.Add(3*time.Second).Add(liveSmokeTradeRangePadding), window.TimeRange.End)
	})
}
