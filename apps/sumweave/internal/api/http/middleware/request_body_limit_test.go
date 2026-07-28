package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitMiddleware(t *testing.T) {
	const maxBytes = 10
	makeNext := func(t *testing.T) http.Handler {
		t.Helper()
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, err := io.ReadAll(request.Body)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusRequestEntityTooLarge)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		})
	}

	t.Run("accepts a body at the configured limit", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxBytes)))

		NewRequestBodyLimitMiddleware(maxBytes)(makeNext(t)).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("rejects a declared oversized body before it reaches the handler", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxBytes+1)))

		NewRequestBodyLimitMiddleware(maxBytes)(makeNext(t)).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "request body exceeds configured limit")
	})
}
