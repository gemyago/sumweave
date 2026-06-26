package internal

import (
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/v1routes/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceCSVMapValidators(t *testing.T) {
	t.Run("ensure non default supports map values", func(t *testing.T) {
		require.NoError(t, EnsureNonDefault(map[string]string{}))
		require.ErrorIs(t, EnsureNonDefault(map[string]string(nil)), ErrValueRequired)
	})

	t.Run("required map fields accept non nil maps", func(t *testing.T) {
		confirmCtx := &BindingContext{}
		NewFinanceCsvImportConfirmRequestValidator()(confirmCtx, &models.FinanceCsvImportConfirmRequest{
			Mapping: map[string]string{},
		})
		require.NoError(t, confirmCtx.AggregatedError())

		previewCtx := &BindingContext{}
		NewFinanceCsvImportPreviewResponseValidator()(previewCtx, &models.FinanceCsvImportPreviewResponse{
			ImportID:              "import-1",
			ImportType:            "transactions",
			Headers:               []string{"accountName"},
			Mapping:               map[string]string{},
			DuplicateRows:         []map[string]interface{}{{"rowNumber": 2}},
			RejectedRows:          []map[string]interface{}{{"rowNumber": 3}},
			WouldCreateAccounts:   []string{"wallet"},
			WouldCreateCategories: []string{"groceries"},
			WouldCreateTags:       []string{"team"},
		})
		require.NoError(t, previewCtx.AggregatedError())
	})

	t.Run("required map fields reject nil maps", func(t *testing.T) {
		ctx := &BindingContext{}
		NewFinanceCsvImportConfirmRequestValidator()(ctx, &models.FinanceCsvImportConfirmRequest{})

		err := ctx.AggregatedError()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mapping")
		assert.Contains(t, err.Error(), ErrValueRequired.Error())
	})
}
