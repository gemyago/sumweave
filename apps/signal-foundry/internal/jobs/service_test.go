package jobs

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/app"
	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/system/ident"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wakeRecorder struct{ calls atomic.Int64 }

func (w *wakeRecorder) SignalWake(_ string) { w.calls.Add(1) }

func TestService(t *testing.T) {
	fake := faker.New()
	makeService := func(t *testing.T, now time.Time) (*Service, *Store, ident.Generator) {
		t.Helper()
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "svc_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		idGen := ident.NewMockGenerator()
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: idGen,
			Clock: func() time.Time {
				return now
			},
			Limits: HistoricalBackfillLimits{MaxIntervals: 10, MaxPageSize: 200},
		})
		require.NoError(t, err)
		return svc, store, idGen
	}
	validParams := func(now time.Time) CreateHistoricalRawCandleBackfillParams {
		return CreateHistoricalRawCandleBackfillParams{
			Requester: Requester{
				UserID: "user-" + fake.UUID().V4(),
				Source: RequesterSourceOperator,
			},
			IdempotencyKey: "idem-" + fake.UUID().V4(),
			CorrelationID:  "corr-" + fake.UUID().V4(),
			Venue:          "hyperliquid-perps",
			Symbol:         "btc",
			AssetClass:     "future",
			Timeframe:      "1h",
			Start:          now.Add(-3 * time.Hour),
			End:            now.Add(-2 * time.Hour),
			PageSize:       100,
		}
	}

	t.Run("creates queued jobs immediately with generated job and ingestion run ids", func(t *testing.T) {
		now := time.Now().UTC()
		svc, _, _ := makeService(t, now)
		created, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), validParams(now))
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, JobStatusQueued, created.Status)
		assert.NotEmpty(t, created.ID)
		assert.NotEmpty(t, created.Input.IngestionRunID)
		assert.NotEqual(t, created.ID, created.Input.IngestionRunID)
		assert.Equal(t, "BTC", created.Input.Symbol)
	})

	t.Run("get returns persisted jobs and create signals wake listeners", func(t *testing.T) {
		now := time.Now().UTC()
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "svc_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		wake := &wakeRecorder{}
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Clock: func() time.Time {
				return now
			},
			Limits: HistoricalBackfillLimits{MaxIntervals: 10, MaxPageSize: 200},
			Wake:   wake,
		})
		require.NoError(t, err)
		created, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), validParams(now))
		require.NoError(t, err)
		loaded, err := svc.Get(t.Context(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, loaded.ID)
		assert.EqualValues(t, 1, wake.calls.Load())
	})

	t.Run("reuses same idempotency key only for same canonical input hash", func(t *testing.T) {
		now := time.Now().UTC()
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "svc_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		wake := &wakeRecorder{}
		svc, err := NewService(ServiceDeps{
			Store:       store,
			IDGenerator: ident.NewMockGenerator(),
			Clock: func() time.Time {
				return now
			},
			Limits: HistoricalBackfillLimits{MaxIntervals: 10, MaxPageSize: 200},
			Wake:   wake,
		})
		require.NoError(t, err)
		params := validParams(now)
		first, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), params)
		require.NoError(t, err)
		second, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), params)
		require.NoError(t, err)
		assert.Equal(t, first.ID, second.ID)
		assert.EqualValues(t, 1, wake.calls.Load())
		listed, err := store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, listed.Items, 1)
		conflictParams := params
		conflictParams.End = params.End.Add(time.Hour)
		_, err = svc.CreateHistoricalRawCandleBackfill(t.Context(), conflictParams)
		require.Error(t, err)
		assert.True(t, IsIdempotencyConflict(err))
		listedAfter, err := store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, listedAfter.Items, 1)
	})

	t.Run("allows same scope without idempotency key to create new jobs", func(t *testing.T) {
		now := time.Now().UTC()
		svc, store, _ := makeService(t, now)
		params := validParams(now)
		params.IdempotencyKey = ""
		first, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), params)
		require.NoError(t, err)
		second, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), params)
		require.NoError(t, err)
		assert.NotEqual(t, first.ID, second.ID)
		listed, err := store.List(t.Context(), ListParams{Limit: 10})
		require.NoError(t, err)
		require.Len(t, listed.Items, 2)
	})

	t.Run("validates hyperliquid futures symbol timeframe range page size and future end", func(t *testing.T) {
		now := time.Now().UTC()
		svc, _, _ := makeService(t, now)
		cases := []CreateHistoricalRawCandleBackfillParams{
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.Venue = "other"; return p }(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.AssetClass = "spot"; return p }(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.Symbol = " "; return p }(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.Timeframe = "2h"; return p }(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.End = p.Start; return p }(),
			func() CreateHistoricalRawCandleBackfillParams {
				p := validParams(now)
				p.End = now.Add(time.Hour)
				return p
			}(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.PageSize = -1; return p }(),
			func() CreateHistoricalRawCandleBackfillParams { p := validParams(now); p.PageSize = 201; return p }(),
			func() CreateHistoricalRawCandleBackfillParams {
				p := validParams(now)
				p.Start = now.Add(-20 * time.Hour)
				p.End = now
				return p
			}(),
		}
		for _, tc := range cases {
			_, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), tc)
			var invalid *app.InvalidInputError
			require.ErrorAs(t, err, &invalid)
		}
	})

	t.Run("get returns not found for missing rows", func(t *testing.T) {
		svc, _, _ := makeService(t, time.Now().UTC())
		_, err := svc.Get(t.Context(), fake.UUID().V4())
		var notFound *app.NotFoundError
		require.ErrorAs(t, err, &notFound)
	})

	t.Run("requires requester source", func(t *testing.T) {
		svc, _, _ := makeService(t, time.Now().UTC())
		params := validParams(time.Now().UTC())
		params.Requester.Source = ""
		_, err := svc.CreateHistoricalRawCandleBackfill(t.Context(), params)
		var invalid *app.InvalidInputError
		require.ErrorAs(t, err, &invalid)
	})

	t.Run("detects idempotency conflict sentinel", func(t *testing.T) {
		assert.True(t, IsIdempotencyConflict(&idempotencyConflictError{key: "k"}))
		assert.False(t, IsIdempotencyConflict(errors.New("nope")))
	})

	t.Run("returns constructor errors and forwards list queries", func(t *testing.T) {
		_, err := NewService(ServiceDeps{})
		require.Error(t, err)
		store, err := NewStore(
			filepath.Join(t.TempDir(), fake.UUID().V4()+".sqlite"),
			StoreOpts{TablePrefix: "svc_"},
		)
		require.NoError(t, err)
		require.NoError(t, store.AutoMigrate())
		_, err = NewService(ServiceDeps{Store: store})
		require.Error(t, err)
		svc, _, _ := makeService(t, time.Now().UTC())
		result, err := svc.List(t.Context(), ListParams{Limit: 5})
		require.NoError(t, err)
		assert.Empty(t, result.Items)
	})
}
