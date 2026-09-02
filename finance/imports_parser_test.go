package finance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFixedCSV(t *testing.T) {
	headers, rows, rejected, err := parseFixedCSV(
		"Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description\n" +
			"29.05.26,Wallet,,,\"12,34\",,USD,Lunch\n",
	)

	require.NoError(t, err)
	assert.Equal(t, fixedCSVHeaders(), headers)
	require.Len(t, rows, 1)
	assert.Empty(t, rejected)
	assert.Equal(t, "Wallet", rows[0].Account)
	assert.Equal(t, int64(-1234), rows[0].AmountMinor)
}
