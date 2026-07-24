package middleware

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAfterMiddleware replays one safe request after a rate-limit response.
type RetryAfterMiddleware struct {
	next          http.RoundTripper
	now           func() time.Time
	wait          func(context.Context, time.Duration) error
	fallbackDelay time.Duration
}

// NewRetryAfterMiddleware creates a transport that honors Retry-After values
// or a fallback delay on one HTTP 429 response.
func NewRetryAfterMiddleware(next http.RoundTripper, fallbackDelay time.Duration) http.RoundTripper {
	return newRetryAfterMiddleware(next, fallbackDelay, time.Now, waitForRetryAfter)
}

func newRetryAfterMiddleware(
	next http.RoundTripper,
	fallbackDelay time.Duration,
	now func() time.Time,
	wait func(context.Context, time.Duration) error,
) http.RoundTripper {
	return &RetryAfterMiddleware{next: next, now: now, wait: wait, fallbackDelay: fallbackDelay}
}

// RoundTrip implements [http.RoundTripper].
func (m *RetryAfterMiddleware) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := m.next.RoundTrip(request)
	if err != nil || response.StatusCode != http.StatusTooManyRequests {
		return response, err
	}

	delay := retryAfterDelay(response.Header.Get("Retry-After"), m.now())
	if delay <= 0 {
		delay = m.fallbackDelay
	}
	if delay <= 0 || !safeRetryMethod(request.Method) {
		return response, nil
	}

	replay := request.Clone(request.Context())
	if request.Body != nil && request.Body != http.NoBody {
		body, replayable := retryBody(request)
		if !replayable {
			return response, nil
		}
		replay.Body = body
	}

	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if err = m.wait(request.Context(), delay); err != nil {
		return nil, err
	}
	return m.next.RoundTrip(replay)
}

func retryBody(request *http.Request) (io.ReadCloser, bool) {
	if request.GetBody == nil {
		return nil, false
	}
	body, err := request.GetBody()
	return body, err == nil
}

func safeRetryMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func retryAfterDelay(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 || seconds > int64((time.Duration(1<<63-1))/time.Second) {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := retryAt.Sub(now)
	if delay <= 0 {
		return 0
	}
	return delay
}

func waitForRetryAfter(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
