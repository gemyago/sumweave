package providers

import (
	"testing"
	"time"

	"github.com/gemyago/sumweave/finance/domain"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointTargetWindowPolicy(t *testing.T) {
	t.Run("determines explicit and automatic targets", func(t *testing.T) {
		fake := faker.New()
		location := time.FixedZone("case-"+fake.Lorem().Word(), fake.IntBetween(-11, 12)*60*60)
		now := time.Date(
			2026,
			time.Month(fake.IntBetween(1, 12)),
			fake.IntBetween(1, 20),
			fake.IntBetween(0, 23),
			fake.IntBetween(0, 59),
			0,
			0,
			location,
		)
		policy := NewCheckpointTargetWindowPolicy()
		makeState := func() *domain.ProviderSyncState {
			checkpointAge := fake.IntBetween(31, 60)
			return &domain.ProviderSyncState{
				Window: domain.ProviderSyncWindow{
					Start: now.AddDate(0, 0, -checkpointAge-fake.IntBetween(1, 10)),
					End:   now.AddDate(0, 0, -checkpointAge),
				},
				SucceededAt: &now,
			}
		}

		t.Run("uses journal state for automatic bounds", func(t *testing.T) {
			state := makeState()

			window, err := policy.Determine(TargetWindowRequest{
				Now:   now,
				State: state,
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncWindow{Start: state.Window.End, End: now}, window)
		})

		t.Run("preserves a supplied start and resolves the end to now", func(t *testing.T) {
			start := now.AddDate(0, 0, -fake.IntBetween(1, 29))

			window, err := policy.Determine(TargetWindowRequest{
				Now:         now,
				WindowStart: &start,
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncWindow{Start: start, End: now}, window)
		})

		t.Run("uses a supplied end with journal-derived start", func(t *testing.T) {
			state := makeState()
			end := now.AddDate(0, 0, -fake.IntBetween(1, 10))
			expectedStart := state.Window.End
			if recentStart := end.AddDate(0, 0, -recentRefreshDays); expectedStart.After(recentStart) {
				expectedStart = recentStart
			}

			window, err := policy.Determine(TargetWindowRequest{
				Now:       now,
				State:     state,
				WindowEnd: &end,
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncWindow{Start: expectedStart, End: end}, window)
		})

		t.Run("preserves complete explicit bounds", func(t *testing.T) {
			start := now.AddDate(0, 0, -fake.IntBetween(31, 60))
			end := start.AddDate(0, 0, fake.IntBetween(1, 30))

			window, err := policy.Determine(TargetWindowRequest{
				Now:         now,
				WindowStart: &start,
				WindowEnd:   &end,
			})
			require.NoError(t, err)
			assert.Equal(t, domain.ProviderSyncWindow{Start: start, End: end}, window)
		})

		t.Run("rejects invalid resolved windows", func(t *testing.T) {
			for name, window := range map[string]domain.ProviderSyncWindow{
				"equal bounds": {
					Start: now,
					End:   now,
				},
				"reversed bounds": {
					Start: now,
					End:   now.Add(-time.Second),
				},
			} {
				t.Run(name, func(t *testing.T) {
					_, err := policy.Determine(TargetWindowRequest{
						Now:         now,
						WindowStart: &window.Start,
						WindowEnd:   &window.End,
					})
					require.ErrorIs(t, err, ErrInvalidProviderSyncTargetWindow)
				})
			}
		})
	})

	t.Run("determine", func(t *testing.T) {
		t.Run("plans a 3 year backfill for fresh runs", func(t *testing.T) {
			fake := faker.New()
			now := fake.Time().Recent().UTC().Truncate(time.Second)
			policy := NewCheckpointTargetWindowPolicy()

			window, err := policy.Determine(TargetWindowRequest{Now: now})
			require.NoError(t, err)

			assert.Equal(t, domain.ProviderSyncWindow{
				Start: now.AddDate(-3, 0, 0),
				End:   now,
			}, window)
		})

		t.Run("refreshes the last 30 days when the latest succeeded checkpoint is recent", func(t *testing.T) {
			fake := faker.New()
			connection := makeRandomProviderConnectionRef(
				fake,
				domain.ProviderIDMonobank,
				domain.ProviderConnectorIDMonobank,
			)
			now := fake.Time().Recent().UTC().Truncate(time.Second)
			makeState := func(
				window domain.ProviderSyncWindow,
				succeededAt *time.Time,
			) *domain.ProviderSyncState {
				attemptedAt := window.End.Add(time.Hour)
				return &domain.ProviderSyncState{
					Connection:  connection,
					AttemptedAt: &attemptedAt,
					SucceededAt: succeededAt,
					Window:      window,
					RunID:       "run-" + fake.UUID().V4(),
					JobID:       "job-" + fake.UUID().V4(),
				}
			}
			policy := NewCheckpointTargetWindowPolicy()
			succeededAt := now.Add(-6 * time.Hour)
			state := makeState(domain.ProviderSyncWindow{
				Start: now.AddDate(0, 0, -7),
				End:   now.AddDate(0, 0, -5),
			}, &succeededAt)

			window, err := policy.Determine(TargetWindowRequest{Now: now, State: state})
			require.NoError(t, err)

			assert.Equal(t, domain.ProviderSyncWindow{
				Start: now.AddDate(0, 0, -30),
				End:   now,
			}, window)
		})

		t.Run("catches up from the last succeeded window end when it is older than 30 days", func(t *testing.T) {
			fake := faker.New()
			connection := makeRandomProviderConnectionRef(
				fake,
				domain.ProviderIDMonobank,
				domain.ProviderConnectorIDMonobank,
			)
			now := fake.Time().Recent().UTC().Truncate(time.Second)
			makeState := func(
				window domain.ProviderSyncWindow,
				succeededAt *time.Time,
			) *domain.ProviderSyncState {
				attemptedAt := window.End.Add(time.Hour)
				return &domain.ProviderSyncState{
					Connection:  connection,
					AttemptedAt: &attemptedAt,
					SucceededAt: succeededAt,
					Window:      window,
					RunID:       "run-" + fake.UUID().V4(),
					JobID:       "job-" + fake.UUID().V4(),
				}
			}
			policy := NewCheckpointTargetWindowPolicy()
			succeededAt := now.AddDate(0, 0, -44)
			checkpoint := now.AddDate(0, 0, -45)
			state := makeState(domain.ProviderSyncWindow{
				Start: checkpoint.AddDate(0, 0, -5),
				End:   checkpoint,
			}, &succeededAt)

			window, err := policy.Determine(TargetWindowRequest{Now: now, State: state})
			require.NoError(t, err)

			assert.Equal(t, domain.ProviderSyncWindow{
				Start: checkpoint,
				End:   now,
			}, window)
		})

		t.Run("retries from the latest failed window start when it is older than 30 days", func(t *testing.T) {
			fake := faker.New()
			connection := makeRandomProviderConnectionRef(
				fake,
				domain.ProviderIDMonobank,
				domain.ProviderConnectorIDMonobank,
			)
			now := fake.Time().Recent().UTC().Truncate(time.Second)
			makeState := func(
				window domain.ProviderSyncWindow,
				succeededAt *time.Time,
			) *domain.ProviderSyncState {
				attemptedAt := window.End.Add(time.Hour)
				return &domain.ProviderSyncState{
					Connection:  connection,
					AttemptedAt: &attemptedAt,
					SucceededAt: succeededAt,
					Window:      window,
					RunID:       "run-" + fake.UUID().V4(),
					JobID:       "job-" + fake.UUID().V4(),
				}
			}
			policy := NewCheckpointTargetWindowPolicy()
			checkpoint := now.AddDate(0, 0, -45)
			state := makeState(domain.ProviderSyncWindow{
				Start: checkpoint,
				End:   checkpoint.AddDate(0, 0, 2),
			}, nil)

			window, err := policy.Determine(TargetWindowRequest{Now: now, State: state})
			require.NoError(t, err)

			assert.Equal(t, domain.ProviderSyncWindow{
				Start: checkpoint,
				End:   now,
			}, window)
		})

		t.Run(
			"falls back to the rolling 30 day window for failed checkpoints inside the recent window",
			func(t *testing.T) {
				fake := faker.New()
				connection := makeRandomProviderConnectionRef(
					fake,
					domain.ProviderIDMonobank,
					domain.ProviderConnectorIDMonobank,
				)
				now := fake.Time().Recent().UTC().Truncate(time.Second)
				makeState := func(
					window domain.ProviderSyncWindow,
					succeededAt *time.Time,
				) *domain.ProviderSyncState {
					attemptedAt := window.End.Add(time.Hour)
					return &domain.ProviderSyncState{
						Connection:  connection,
						AttemptedAt: &attemptedAt,
						SucceededAt: succeededAt,
						Window:      window,
						RunID:       "run-" + fake.UUID().V4(),
						JobID:       "job-" + fake.UUID().V4(),
					}
				}
				policy := NewCheckpointTargetWindowPolicy()
				state := makeState(domain.ProviderSyncWindow{
					Start: now.AddDate(0, 0, -12),
					End:   now.AddDate(0, 0, -10),
				}, nil)

				window, err := policy.Determine(TargetWindowRequest{Now: now, State: state})
				require.NoError(t, err)

				assert.Equal(t, domain.ProviderSyncWindow{
					Start: now.AddDate(0, 0, -30),
					End:   now,
				}, window)
			},
		)

		t.Run("fails for invalid latest state windows", func(t *testing.T) {
			fake := faker.New()
			connection := makeRandomProviderConnectionRef(
				fake,
				domain.ProviderIDMonobank,
				domain.ProviderConnectorIDMonobank,
			)
			now := fake.Time().Recent().UTC().Truncate(time.Second)
			makeState := func(
				window domain.ProviderSyncWindow,
				succeededAt *time.Time,
			) *domain.ProviderSyncState {
				attemptedAt := window.End.Add(time.Hour)
				return &domain.ProviderSyncState{
					Connection:  connection,
					AttemptedAt: &attemptedAt,
					SucceededAt: succeededAt,
					Window:      window,
					RunID:       "run-" + fake.UUID().V4(),
					JobID:       "job-" + fake.UUID().V4(),
				}
			}
			policy := NewCheckpointTargetWindowPolicy()
			succeededAt := now.Add(-time.Hour)

			testCases := map[string]domain.ProviderSyncWindow{
				"zero start": {
					End: now,
				},
				"zero end": {
					Start: now.AddDate(0, 0, -1),
				},
				"inverted bounds": {
					Start: now,
					End:   now.Add(-time.Hour),
				},
			}

			for name, invalidWindow := range testCases {
				t.Run(name, func(t *testing.T) {
					_, err := policy.Determine(TargetWindowRequest{
						Now:   now,
						State: makeState(invalidWindow, &succeededAt),
					})
					require.ErrorIs(t, err, ErrInvalidProviderSyncStateWindow)
				})
			}
		})
	})
}
