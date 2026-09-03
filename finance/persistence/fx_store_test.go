package persistence

import (
	"sync"
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentFXRateStore(t *testing.T) {
	makeRate := func(provider string, rate float64, at time.Time) domain.FXRate {
		return domain.FXRate{
			Provider: provider, BaseCurrency: "EUR", QuoteCurrency: "USD", EffectiveAt: at,
			LastSuccessfulRefreshAt: at, Rate: rate,
		}
	}

	t.Run("retains an existing rate while concurrent fixture writes race", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewCurrentFXRateStore(database)
		fake := faker.New()
		provider := "fixture-" + fake.UUID().V4()
		existing := makeRate(provider, float64(fake.IntBetween(101, 200))/100, time.Now().Add(-time.Hour))
		require.NoError(t, store.SaveCurrentFXRates(t.Context(), []domain.FXRate{existing}))
		before, err := store.ListCurrentFXRates(t.Context(), ListCurrentFXRatesParams{Provider: provider})
		require.NoError(t, err)

		var group sync.WaitGroup
		errs := make(chan error, 2)
		for _, rate := range []float64{
			float64(fake.IntBetween(201, 300)) / 100,
			float64(fake.IntBetween(301, 400)) / 100,
		} {
			group.Add(1)
			go func(rate float64) {
				defer group.Done()
				errs <- store.SaveCurrentFXRatesIfAbsent(t.Context(), []domain.FXRate{
					makeRate(provider, rate, time.Now()),
				})
			}(rate)
		}
		group.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		after, err := store.ListCurrentFXRates(t.Context(), ListCurrentFXRatesParams{Provider: provider})
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("accepts legacy timestamps only when they resolve to a rate date", func(t *testing.T) {
		database := openTestDatabase(t)
		store := NewCurrentFXRateStore(database)
		fake := faker.New()
		provider := "fixture-" + fake.UUID().V4()
		rateDate := time.Now()
		require.NoError(t, store.SaveCurrentFXRatesIfAbsent(t.Context(), []domain.FXRate{{
			Provider: provider, BaseCurrency: "EUR", QuoteCurrency: "USD", RateDate: rateDate,
			Rate: float64(fake.IntBetween(101, 200)) / 100,
		}}))
		rates, err := store.ListCurrentFXRates(t.Context(), ListCurrentFXRatesParams{Provider: provider})
		require.NoError(t, err)
		require.Len(t, rates, 1)
		assert.True(t, rateDate.Equal(rates[0].EffectiveAt))
		assert.True(t, rateDate.Equal(rates[0].LastSuccessfulRefreshAt))

		require.Error(t, store.SaveCurrentFXRatesIfAbsent(t.Context(), []domain.FXRate{{
			Provider: provider, BaseCurrency: "GBP", QuoteCurrency: "USD",
			Rate: float64(fake.IntBetween(201, 300)) / 100,
		}}))
	})
}
