package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/appdispatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry(t *testing.T) {
	type input struct{ Value string }
	type result struct{ Value string }
	type progress struct{ Value string }

	t.Run("rejects missing registration inputs and duplicate job types", func(t *testing.T) {
		registry := NewRegistry()
		require.Error(t, registry.Register(nil))
		require.Error(
			t,
			RegisterTypedHandler[input, result, progress](
				nil,
				TypedHandlerSpec[input, result, progress]{},
			),
		)
		require.Error(t, RegisterTypedHandler(
			registry,
			TypedHandlerSpec[input, result, progress]{JobType: JobType("finance.test")},
		))
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobType("finance.test"),
			Run:     func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
		require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobType("finance.test"),
			Run:     func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType:      JobType("finance.other"),
			DispatchKind: appdispatch.ExecutionKind("dispatch.test"),
			Run:          func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
		require.Error(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType:      JobType("finance.third"),
			DispatchKind: appdispatch.ExecutionKind("dispatch.test"),
			Run:          func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
	})

	t.Run("surfaces missing handlers and execute marshaling failures", func(t *testing.T) {
		registry := NewRegistry()
		_, err := registry.Handler(JobType("missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		var nilRegistry *Registry
		_, err = nilRegistry.Handler(JobType("missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		_, err = registry.HandlerByExecutionKind(appdispatch.ExecutionKind("missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)
		_, err = nilRegistry.HandlerByExecutionKind(appdispatch.ExecutionKind("missing"))
		require.ErrorIs(t, err, ErrHandlerNotRegistered)

		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, chan int, progress]{
			JobType: JobType("finance.bad-result"),
			Run: func(context.Context, input, func(progress) error) (chan int, error) {
				return make(chan int), nil
			},
		}))
		handler, err := registry.Handler(JobType("finance.bad-result"))
		require.NoError(t, err)
		_, err = handler.execute(
			t.Context(),
			Job{InputJSON: mustRegistryJSON(t, input{Value: "ok"})},
			func(json.RawMessage) error { return nil },
		)
		require.Error(t, err)

		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobType("finance.progress-error"),
			Run: func(context.Context, input, func(progress) error) (result, error) {
				return result{}, errors.New("boom")
			},
		}))
		handler, err = registry.Handler(JobType("finance.progress-error"))
		require.NoError(t, err)
		_, err = handler.execute(
			t.Context(),
			Job{InputJSON: mustRegistryJSON(t, input{Value: "ok"})},
			func(json.RawMessage) error { return nil },
		)
		require.EqualError(t, err, "boom")

		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, chan int]{
			JobType: JobType("finance.bad-progress"),
			Run: func(_ context.Context, value input, setProgress func(chan int) error) (result, error) {
				require.Equal(t, input{Value: "ok"}, value)
				return result{}, setProgress(make(chan int))
			},
		}))
		handler, err = registry.Handler(JobType("finance.bad-progress"))
		require.NoError(t, err)
		_, err = handler.execute(
			t.Context(),
			Job{InputJSON: mustRegistryJSON(t, input{Value: "ok"})},
			func(json.RawMessage) error { return nil },
		)
		require.Error(t, err)
	})

	t.Run("resolves handlers by dispatch kind and default mapping", func(t *testing.T) {
		registry := NewRegistry()
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobTypeHistoricalRawCandleBackfill,
			Run:     func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))
		require.NoError(t, RegisterTypedHandler(registry, TypedHandlerSpec[input, result, progress]{
			JobType: JobType("finance.custom"),
			Run:     func(context.Context, input, func(progress) error) (result, error) { return result{}, nil },
		}))

		historicalHandler, err := registry.HandlerByExecutionKind(DispatchKindHistoricalRawCandleBackfill)
		require.NoError(t, err)
		assert.Equal(t, JobTypeHistoricalRawCandleBackfill, historicalHandler.jobType())

		customHandler, err := registry.HandlerByExecutionKind(appdispatch.ExecutionKind("finance.custom"))
		require.NoError(t, err)
		assert.Equal(t, JobType("finance.custom"), customHandler.jobType())
	})
}

func mustRegistryJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := EncodeJobPayload(value)
	require.NoError(t, err)
	return payload
}
