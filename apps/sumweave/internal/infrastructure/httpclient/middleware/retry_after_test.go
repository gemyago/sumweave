package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type retryAfterTrackingBody struct {
	*bytes.Reader

	closed bool
}

func (b *retryAfterTrackingBody) Close() error {
	b.closed = true
	return nil
}

func TestRetryAfterMiddleware(t *testing.T) {
	fake := faker.New()
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	fallbackDelay := 500 * time.Millisecond
	makeRequest := func(t *testing.T, method string, body io.Reader) *http.Request {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), method, "https://"+fake.Internet().Domain(), body)
		require.NoError(t, err)
		return request
	}
	makeResponse := func(status int, retryAfter string, body *retryAfterTrackingBody) *http.Response {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Retry-After": []string{retryAfter}},
			Body:       body,
		}
	}
	makeBody := func(value string) *retryAfterTrackingBody {
		return &retryAfterTrackingBody{Reader: bytes.NewReader([]byte(value))}
	}
	makeMiddleware := func(
		transport http.RoundTripper,
		wait func(context.Context, time.Duration) error,
	) http.RoundTripper {
		return newRetryAfterMiddleware(transport, fallbackDelay, func() time.Time { return now }, wait)
	}

	t.Run("replays once for positive delta seconds and future HTTP dates", func(t *testing.T) {
		testCases := []struct {
			name       string
			retryAfter string
			delay      time.Duration
		}{
			{name: "delta seconds", retryAfter: "17", delay: 17 * time.Second},
			{name: "HTTP date", retryAfter: now.Add(2 * time.Minute).Format(http.TimeFormat), delay: 2 * time.Minute},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				firstBody := makeBody(fake.Lorem().Text(10))
				secondBody := makeBody(fake.Lorem().Text(10))
				transport := &MockRoundTripper{}
				transport.On("RoundTrip", mock.Anything).Once().Return(
					makeResponse(http.StatusTooManyRequests, testCase.retryAfter, firstBody), nil,
				)
				transport.On("RoundTrip", mock.Anything).Once().Return(makeResponse(http.StatusOK, "", secondBody), nil)
				waits := []time.Duration{}
				middleware := makeMiddleware(transport, func(_ context.Context, delay time.Duration) error {
					waits = append(waits, delay)
					return nil
				})

				response, err := middleware.RoundTrip(makeRequest(t, http.MethodGet, nil))

				require.NoError(t, err)
				assert.Same(t, secondBody, response.Body)
				assert.True(t, firstBody.closed)
				assert.Equal(t, []time.Duration{testCase.delay}, waits)
				transport.AssertExpectations(t)
			})
		}
	})

	t.Run("uses fallback for invalid Retry-After values", func(t *testing.T) {
		testCases := []string{"0", "not-a-delay", now.Add(-time.Second).Format(http.TimeFormat), "9223372036854775807"}
		for _, retryAfter := range testCases {
			t.Run(retryAfter, func(t *testing.T) {
				firstBody := makeBody(fake.Lorem().Text(10))
				secondBody := makeBody(fake.Lorem().Text(10))
				transport := &MockRoundTripper{}
				transport.On("RoundTrip", mock.Anything).Once().Return(
					makeResponse(http.StatusTooManyRequests, retryAfter, firstBody), nil,
				)
				transport.On("RoundTrip", mock.Anything).Once().Return(makeResponse(http.StatusOK, "", secondBody), nil)
				waits := []time.Duration{}
				middleware := makeMiddleware(transport, func(_ context.Context, delay time.Duration) error {
					waits = append(waits, delay)
					return nil
				})

				actual, err := middleware.RoundTrip(makeRequest(t, http.MethodGet, nil))

				require.NoError(t, err)
				assert.Same(t, secondBody, actual.Body)
				assert.True(t, firstBody.closed)
				assert.Equal(t, []time.Duration{fallbackDelay}, waits)
				transport.AssertExpectations(t)
			})
		}
	})

	t.Run("stops after the replay and returns its response", func(t *testing.T) {
		firstBody := makeBody(fake.Lorem().Text(10))
		secondBody := makeBody(fake.Lorem().Text(10))
		second := makeResponse(http.StatusTooManyRequests, "3", secondBody)
		transport := &MockRoundTripper{}
		transport.On("RoundTrip", mock.Anything).Once().Return(
			makeResponse(http.StatusTooManyRequests, "3", firstBody), nil,
		)
		transport.On("RoundTrip", mock.Anything).Once().Return(second, nil)
		middleware := makeMiddleware(transport, func(context.Context, time.Duration) error { return nil })

		actual, err := middleware.RoundTrip(makeRequest(t, http.MethodGet, nil))

		require.NoError(t, err)
		assert.Same(t, second, actual)
		assert.True(t, firstBody.closed)
		assert.False(t, secondBody.closed)
		transport.AssertExpectations(t)
	})

	t.Run("cancellation during the wait prevents a replay", func(t *testing.T) {
		firstBody := makeBody(fake.Lorem().Text(10))
		transport := &MockRoundTripper{}
		transport.On("RoundTrip", mock.Anything).Once().Return(
			makeResponse(http.StatusTooManyRequests, "7", firstBody), nil,
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		request := makeRequest(t, http.MethodGet, nil).WithContext(ctx)
		middleware := makeMiddleware(transport, waitForRetryAfter)

		response, err := middleware.RoundTrip(request)

		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, response)
		assert.True(t, firstBody.closed)
		transport.AssertExpectations(t)
	})

	t.Run("does not replay unsafe or non-replayable requests", func(t *testing.T) {
		testCases := []struct {
			name    string
			method  string
			body    io.Reader
			getBody func() (io.ReadCloser, error)
		}{
			{
				name: "unsafe method", method: http.MethodPost,
				body: bytes.NewBufferString(fake.Lorem().Text(10)),
			},
			{
				name: "body without GetBody", method: http.MethodGet,
				body: bytes.NewBufferString(fake.Lorem().Text(10)),
			},
			{
				name: "GetBody failure", method: http.MethodGet,
				body: bytes.NewBufferString(fake.Lorem().Text(10)),
				getBody: func() (io.ReadCloser, error) {
					return nil, errors.New(fake.Lorem().Word())
				},
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				body := makeBody(fake.Lorem().Text(10))
				response := makeResponse(http.StatusTooManyRequests, "5", body)
				transport := &MockRoundTripper{}
				transport.On("RoundTrip", mock.Anything).Once().Return(response, nil)
				middleware := makeMiddleware(transport, func(context.Context, time.Duration) error { return nil })
				request := makeRequest(t, testCase.method, testCase.body)
				request.GetBody = testCase.getBody

				actual, err := middleware.RoundTrip(request)

				require.NoError(t, err)
				assert.Same(t, response, actual)
				assert.False(t, body.closed)
				transport.AssertExpectations(t)
			})
		}
	})
}
