package finance

import (
	"fmt"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/finance/domain"
	"github.com/gemyago/signal-foundry/finance/persistence"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardTransactionKindRegression(t *testing.T) {
	t.Run("includes a booked expense from the transaction editor in its civil month", func(t *testing.T) {
		fake := faker.New()
		location, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)

		now := time.Date(2026, time.July, 10, 16, 0, 0, 0, location)
		service := NewService(
			persistence.NewStore(openTestDatabase(t)),
			WithNow(func() time.Time { return now }),
		)
		ownerUserID := "owner-" + fake.UUID().V4()
		tenant, err := service.CreateTenant(t.Context(), CreateTenantParams{
			ActorUserID:     ownerUserID,
			Name:            "tenant-" + fake.Company().Name(),
			DisplayCurrency: "USD",
		})
		require.NoError(t, err)
		account, err := service.CreateAccount(t.Context(), CreateAccountParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			Name:        "account-" + fake.Lorem().Word(),
			Currency:    "USD",
			Kind:        domain.AccountKindManual,
		})
		require.NoError(t, err)

		_, err = service.RecordTransaction(t.Context(), RecordTransactionParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
			AccountID:   account.ID,
			Source:      domain.TransactionSourceManual,
			Status:      domain.TransactionStatusBooked,
			Kind:        domain.TransactionKind("expense"),
			AmountMinor: -1234,
			Currency:    "USD",
			Description: fmt.Sprintf("expense-%s", fake.Lorem().Word()),
			EffectiveAt: time.Date(2026, time.July, 10, 15, 30, 0, 0, location),
		})
		require.NoError(t, err)

		dashboard, err := service.GetDashboard(t.Context(), DashboardParams{
			ActorUserID: ownerUserID,
			TenantID:    tenant.ID,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1234), dashboard.Settled.ExpenseMinor)
		assert.Equal(t, int64(-1234), dashboard.Settled.NetMinor)
		assert.Equal(t, 1, dashboard.Settled.TransactionCount)
		assert.Equal(t, int64(0), dashboard.Pending.ExpenseMinor)
		assert.Equal(t, int64(0), dashboard.Pending.NetMinor)
		assert.Equal(t, 0, dashboard.Pending.TransactionCount)
	})
}
