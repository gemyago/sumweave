package middleware

import "net/http"

// NewRequestBodyLimitMiddleware protects JSON endpoints from oversized bodies.
func NewRequestBodyLimitMiddleware(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Body == nil {
				next.ServeHTTP(writer, request)
				return
			}
			if request.ContentLength > maxBytes {
				http.Error(writer, "request body exceeds configured limit", http.StatusRequestEntityTooLarge)
				return
			}
			request.Body = http.MaxBytesReader(writer, request.Body, maxBytes)
			next.ServeHTTP(writer, request)
		})
	}
}
