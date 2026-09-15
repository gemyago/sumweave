package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewAppErrorHandler(t *testing.T) {
	fake := faker.New()
	handler := NewAppErrorHandler(telemetry.RootTestLogger())
	for _, testCase := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid request", err: app.NewErrInvalidInput(fake.Lorem().Word(), fake.Lorem().Sentence(3)), wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "unauthorized", err: app.NewErrUnauthorized(fake.Lorem().Sentence(3)), wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "permission", err: app.NewErrForbidden(), wantStatus: http.StatusForbidden, wantCode: "insufficient_permission"},
		{name: "tenant permission", err: app.NewErrTenantAccessDenied(), wantStatus: http.StatusForbidden, wantCode: "tenant_access_denied"},
		{name: "not found", err: app.NewErrNotFound(fake.Lorem().Word(), fake.UUID().V4()), wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "idempotency conflict", err: app.NewErrIdempotencyConflict(), wantStatus: http.StatusConflict, wantCode: "idempotency_conflict"},
		{name: "internal wrapped error", err: errors.New(fake.Lorem().Sentence(3)), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			correlationID := fake.UUID().V4()
			request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(t.Context())
			request.Header.Set(telemetry.CorrelationIDHeader, correlationID)
			response := httptest.NewRecorder()
			correlationMiddleware := NewCorrelationMiddleware(ident.NewDefaultGenerator())
			correlationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler(w, r, testCase.err)
			})).ServeHTTP(response, request)
			assert.Equal(t, testCase.wantStatus, response.Code)
			assert.Equal(t, correlationID, response.Header().Get(telemetry.CorrelationIDHeader))
			assert.JSONEq(t, expectedAPIError(testCase.wantCode, correlationID), response.Body.String())
			assert.NotContains(t, response.Body.String(), testCase.err.Error())
		})
	}
}

func expectedAPIError(code, correlationID string) string {
	return `{"code":"` + code + `","message":"` + apiMessage(code) +
		`","correlationId":"` + correlationID + `"}`
}

func TestNewParserErrorHandler(t *testing.T) {
	fake := faker.New()
	handler := NewParserErrorHandler(telemetry.RootTestLogger())
	correlationID := fake.UUID().V4()
	parseErr := errors.New(fake.Lorem().Sentence(3))
	request := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(t.Context())
	request.Header.Set(telemetry.CorrelationIDHeader, correlationID)
	response := httptest.NewRecorder()

	correlationMiddleware := NewCorrelationMiddleware(ident.NewDefaultGenerator())
	correlationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, parseErr)
	})).ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, correlationID, response.Header().Get(telemetry.CorrelationIDHeader))
	assert.JSONEq(t, expectedAPIError(apiCodeInvalidRequest, correlationID), response.Body.String())
	assert.NotContains(t, response.Body.String(), parseErr.Error())
}

func apiMessage(code string) string {
	switch code {
	case "invalid_request":
		return "The request is invalid."
	case "unauthorized":
		return "Authentication is required."
	case "insufficient_permission":
		return "The credential cannot access this resource."
	case "tenant_access_denied":
		return "The credential cannot access this tenant."
	case "not_found":
		return "The requested resource was not found."
	case "idempotency_conflict":
		return "The request conflicts with an earlier submission."
	default:
		return "An internal error occurred."
	}
}
