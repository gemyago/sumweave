package jobs

import (
	"errors"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/runtime/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeHelpers(t *testing.T) {
	t.Run("idempotency conflict helpers expose stable values", func(t *testing.T) {
		err := &idempotencyConflictError{key: "abc"}
		assert.Contains(t, err.Error(), "abc")
		assert.Equal(t, "idempotency_key_conflict", err.Code())
	})

	t.Run("normalizers apply defaults and bounds", func(t *testing.T) {
		assert.Equal(
			t,
			defaultHistoricalMaxIntervals,
			normalizeHistoricalBackfillLimits(HistoricalBackfillLimits{}).MaxIntervals,
		)
		assert.Equal(t, defaultWorkerPollInterval, normalizeWorkerConfig(WorkerConfig{}).PollInterval)
		assert.Equal(t, defaultListLimit, normalizeListParams(ListParams{}).Limit)
		assert.Equal(t, maxListLimit, normalizeListParams(ListParams{Limit: 1000}).Limit)
	})

	t.Run("truncate and safe execution errors cover edge branches", func(t *testing.T) {
		assert.Empty(t, truncateBounded("", 0))
		assert.Equal(t, "x", truncateBounded("xx", 1))
		assert.Equal(t, "…", truncateBounded("abcdef", len("…")))
		assert.Equal(t, "abcd…", truncateBounded("abcdefghi", 7))
		assert.Nil(t, jobErrorFromExecution(nil))
		unsafe := jobErrorFromExecution(errors.New("gorm sql update secrets"))
		require.NotNil(t, unsafe)
		assert.Equal(t, "historical backfill execution failed", unsafe.Details)
		safe := jobErrorFromExecution(errors.New("venue timeout"))
		require.NotNil(t, safe)
		assert.Equal(t, "venue timeout", safe.Details)
	})

	t.Run("historical timeframe duration covers supported and unsupported values", func(t *testing.T) {
		for _, timeframe := range []domain.Timeframe{
			domain.Timeframe1m,
			domain.Timeframe5m,
			domain.Timeframe15m,
			domain.Timeframe1h,
			domain.Timeframe4h,
			domain.Timeframe1d,
		} {
			_, err := historicalBackfillTimeframeDuration(timeframe)
			require.NoError(t, err)
		}
		_, err := historicalBackfillTimeframeDuration(domain.Timeframe("2h"))
		require.Error(t, err)
	})

	t.Run("validate input normalizes timestamps and page defaults", func(t *testing.T) {
		now := time.Now().UTC()
		input, err := validateHistoricalBackfillInput(HistoricalRawCandleBackfillInput{
			Venue:      "hyperliquid-perps",
			Symbol:     "eth",
			AssetClass: "future",
			Timeframe:  "1m",
			Start:      now.Add(-2 * time.Minute),
			End:        now.Add(-time.Minute),
			PageSize:   0,
		}, HistoricalBackfillLimits{}, now)
		require.NoError(t, err)
		assert.Equal(t, "ETH", input.Symbol)
		assert.Equal(t, input.Start, input.TimeRange.Start)
	})
}
