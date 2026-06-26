package jobs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	fake := faker.New()
	t.Run("constructor validates dependencies", func(t *testing.T) {
		_, err := NewScheduler(SchedulerDeps{})
		require.Error(t, err)
		store, err := NewStore(filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"), StoreOpts{TablePrefix: "sched_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		_, err = NewScheduler(SchedulerDeps{Store: store})
		require.Error(t, err)
	})

	t.Run("enqueue due surfaces schedule read errors", func(t *testing.T) {
		store, err := NewStore(filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"), StoreOpts{TablePrefix: "sched_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		registry := NewRegistry()
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[map[string]string, map[string]string, struct{}]{
				JobType: JobType("finance.fx_rates_sync"),
				Run: func(_ context.Context, input map[string]string, _ func(struct{}) error) (map[string]string, error) {
					return input, nil
				},
			},
		))
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Registry:    registry,
		})
		require.NoError(t, err)
		now := time.Now().UTC()
		require.NoError(t, store.UpsertSchedule(t.Context(), Schedule{
			ID:        "sched-1",
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: "system", Source: RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: now.Add(-time.Minute),
			InputJSON: mustRegistryJSON(t, map[string]string{"scope": "test"}),
		}))
		scheduler, err := NewScheduler(SchedulerDeps{
			Store:   store,
			Service: svc,
			Clock:   func() time.Time { return now },
		})
		require.NoError(t, err)
		require.NoError(t, store.db.Exec("DROP TABLE "+store.scheduleTableName()).Error)
		_, err = scheduler.EnqueueDue(t.Context())
		require.Error(t, err)
	})

	t.Run("enqueue due returns non-idempotency enqueue errors", func(t *testing.T) {
		store, err := NewStore(filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"), StoreOpts{TablePrefix: "sched_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		registry := NewRegistry()
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Registry:    registry,
		})
		require.NoError(t, err)
		now := time.Now().UTC()
		require.NoError(t, store.UpsertSchedule(t.Context(), Schedule{
			ID:        "sched-2",
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: "system", Source: RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: now.Add(-time.Minute),
			InputJSON: mustRegistryJSON(t, map[string]string{"scope": "test"}),
		}))
		scheduler, err := NewScheduler(SchedulerDeps{
			Store:   store,
			Service: svc,
			Clock:   func() time.Time { return now },
		})
		require.NoError(t, err)
		_, err = scheduler.EnqueueDue(t.Context())
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
	})

	t.Run("enqueue due keeps the due window eligible when schedule advancement cannot commit", func(t *testing.T) {
		store, err := NewStore(filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"), StoreOpts{TablePrefix: "sched_"})
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		registry := NewRegistry()
		require.NoError(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[map[string]string, map[string]string, struct{}]{
				JobType: JobType("finance.fx_rates_sync"),
				Run: func(_ context.Context, input map[string]string, _ func(struct{}) error) (map[string]string, error) {
					return input, nil
				},
			},
		))
		now := time.Now().UTC()
		schedule := Schedule{
			ID:        "sched-atomic-" + fake.UUID().V4(),
			JobType:   JobType("finance.fx_rates_sync"),
			Requester: Requester{UserID: "system", Source: RequesterSourceOperator},
			Interval:  time.Hour,
			NextRunAt: now.Add(-time.Minute),
			InputJSON: mustRegistryJSON(t, map[string]string{"scope": "scheduled"}),
		}
		require.NoError(t, store.UpsertSchedule(t.Context(), schedule))

		tickCtx, cancelTick := context.WithCancel(t.Context())

		failingPublisher := &publisherStub{publishInTx: func(context.Context, *sql.Tx, appdispatch.Envelope) error {
			cancelTick()
			return nil
		}}
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   failingPublisher,
			Clock:       func() time.Time { return now },
			Registry:    registry,
		})
		require.NoError(t, err)
		scheduler, err := NewScheduler(SchedulerDeps{
			Store:   store,
			Service: svc,
			Clock:   func() time.Time { return now },
		})
		require.NoError(t, err)

		count, err := scheduler.EnqueueDue(tickCtx)
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))

		listed, err := store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, listed.Items)

		dueSchedules, err := store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		require.Len(t, dueSchedules, 1)
		assert.Equal(t, schedule.NextRunAt, dueSchedules[0].NextRunAt)
		assert.Nil(t, dueSchedules[0].LastEnqueuedAt)

		workingSvc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Publisher:   &publisherStub{},
			Clock:       func() time.Time { return now },
			Registry:    registry,
		})
		require.NoError(t, err)
		scheduler, err = NewScheduler(SchedulerDeps{
			Store:   store,
			Service: workingSvc,
			Clock:   func() time.Time { return now },
		})
		require.NoError(t, err)

		count, err = scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		listed, err = store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, listed.Items, 1)
		assert.Equal(t, JobStatusQueued, listed.Items[0].Status)

		dueSchedules, err = store.ListDueSchedules(t.Context(), now)
		require.NoError(t, err)
		assert.Empty(t, dueSchedules)
	})
}
