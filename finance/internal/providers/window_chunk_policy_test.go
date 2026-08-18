package providers

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOldestFirstWindowChunkPolicy(t *testing.T) {
	makeWindow := func(fake faker.Faker, days int) domain.ProviderSyncWindow {
		location := time.FixedZone("case-"+fake.Lorem().Word(), fake.IntBetween(-11, 12)*60*60)
		start := time.Date(
			2026,
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 20),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			0,
			0,
			location,
		)
		return domain.ProviderSyncWindow{Start: start, End: start.AddDate(0, 0, days)}
	}

	t.Run("split", func(t *testing.T) {
		t.Run("returns a short target unchanged", func(t *testing.T) {
			fake := faker.New()
			window := makeWindow(fake, fake.IntBetween(1, 29))
			policy := NewOldestFirstWindowChunkPolicy()

			windows, err := policy.Split(window)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderSyncWindow{window}, windows)
		})

		t.Run("keeps an exact boundary in one window", func(t *testing.T) {
			fake := faker.New()
			window := makeWindow(fake, 30)
			policy := NewOldestFirstWindowChunkPolicy()

			windows, err := policy.Split(window)
			require.NoError(t, err)
			assert.Equal(t, []domain.ProviderSyncWindow{window}, windows)
		})

		t.Run("splits a long target into contiguous oldest-first windows", func(t *testing.T) {
			fake := faker.New()
			window := makeWindow(fake, fake.IntBetween(61, 120))
			policy := NewOldestFirstWindowChunkPolicy()

			windows, err := policy.Split(window)
			require.NoError(t, err)
			require.Greater(t, len(windows), 2)
			assert.Equal(t, window.Start, windows[0].Start)
			assert.Equal(t, window.End, windows[len(windows)-1].End)
			for index, requested := range windows {
				assert.False(t, requested.End.After(requested.Start.AddDate(0, 0, 30)))
				if index > 0 {
					assert.Equal(t, windows[index-1].End, requested.Start)
				}
			}
		})

		t.Run("preserves target location", func(t *testing.T) {
			fake := faker.New()
			window := makeWindow(fake, fake.IntBetween(31, 60))
			policy := NewOldestFirstWindowChunkPolicy()

			windows, err := policy.Split(window)
			require.NoError(t, err)
			for _, requested := range windows {
				assert.Same(t, window.Start.Location(), requested.Start.Location())
				assert.Same(t, window.Start.Location(), requested.End.Location())
			}
		})

		t.Run("rejects invalid targets", func(t *testing.T) {
			fake := faker.New()
			validWindow := makeWindow(fake, fake.IntBetween(1, 29))
			policy := NewOldestFirstWindowChunkPolicy()

			for name, window := range map[string]domain.ProviderSyncWindow{
				"zero start": {End: validWindow.End},
				"zero end":   {Start: validWindow.Start},
				"equal bounds": {
					Start: validWindow.Start,
					End:   validWindow.Start,
				},
			} {
				t.Run(name, func(t *testing.T) {
					_, err := policy.Split(window)
					require.ErrorIs(t, err, ErrInvalidProviderSyncTargetWindow)
				})
			}
		})
	})
}
