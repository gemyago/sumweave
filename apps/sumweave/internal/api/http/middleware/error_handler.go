package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gemyago/sumweave/apps/sumweave/internal/app"
	"github.com/gemyago/sumweave/apps/sumweave/internal/auth"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
)

const (
	apiCodeUnauthorized           = "unauthorized"
	apiCodeInsufficientPermission = "insufficient_permission"
	apiCodeInvalidRequest         = "invalid_request"
	apiCodeIdempotencyConflict    = "idempotency_conflict"
	apiCodeConflict               = "conflict"
	apiCodeNotFound               = "not_found"
	apiCodeTenantAccessDenied     = "tenant_access_denied"
	apiCodeInternalError          = "internal_error"

	apiMessageUnauthorized           = "Authentication is required."
	apiMessageInsufficientPermission = "The credential cannot access this resource."
	apiMessageInvalidRequest         = "The request is invalid."
	apiMessageIdempotencyConflict    = "The request conflicts with an earlier submission."
	apiMessageConflict               = "The request conflicts with existing state."
	apiMessageNotFound               = "The requested resource was not found."
	apiMessageTenantAccessDenied     = "The credential cannot access this tenant."
	apiMessageInternalError          = "An internal error occurred."
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func APIErrorUnauthorized() APIError {
	return newAPIError(http.StatusUnauthorized, apiCodeUnauthorized, apiMessageUnauthorized)
}

func APIErrorInsufficientPermission() APIError {
	return newAPIError(http.StatusForbidden, apiCodeInsufficientPermission, apiMessageInsufficientPermission)
}

func apiErrorInternal() APIError {
	return newAPIError(http.StatusInternalServerError, apiCodeInternalError, apiMessageInternalError)
}

func newAPIError(status int, code, message string) APIError {
	return APIError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func NewAppErrorHandler(rootLogger *slog.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	logger := rootLogger.WithGroup("error-handler")
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var errNotFound *app.NotFoundError
		var errInvalidInput *app.InvalidInputError
		var errConflict *app.ConflictError
		var errUnauthorized *app.UnauthorizedError
		var errForbidden *app.ForbiddenError
		var errIdempotencyConflict *app.IdempotencyConflictError
		logLevel := slog.LevelWarn
		apiError := apiErrorInternal()
		switch {
		case errors.As(err, &errInvalidInput):
			apiError = newAPIError(http.StatusBadRequest, apiCodeInvalidRequest, apiMessageInvalidRequest)
		case errors.As(err, &errIdempotencyConflict):
			apiError = newAPIError(http.StatusConflict, apiCodeIdempotencyConflict, apiMessageIdempotencyConflict)
		case errors.As(err, &errConflict):
			apiError = newAPIError(http.StatusConflict, apiCodeConflict, apiMessageConflict)
		case errors.As(err, &errNotFound):
			apiError = newAPIError(http.StatusNotFound, apiCodeNotFound, apiMessageNotFound)
		case errors.As(err, &errForbidden):
			if errForbidden.TenantAccessDenied {
				apiError = newAPIError(http.StatusForbidden, apiCodeTenantAccessDenied, apiMessageTenantAccessDenied)
			} else {
				apiError = APIErrorInsufficientPermission()
			}
		case errors.As(err, &errUnauthorized),
			errors.Is(err, auth.ErrInvalidCredentials),
			errors.Is(err, auth.ErrInvalidRefreshToken),
			errors.Is(err, auth.ErrUserNotFound):
			apiError = APIErrorUnauthorized()
		default:
			logLevel = slog.LevelError
		}
		logger.Log(r.Context(), logLevel, "failed to process request", telemetry.ErrAttr(err))
		WriteError(w, r, apiError)
	}
}

func NewParserErrorHandler(rootLogger *slog.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	logger := rootLogger.WithGroup("error-handler")
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.WarnContext(r.Context(), "failed to parse request", telemetry.ErrAttr(err))
		WriteError(w, r, newAPIError(http.StatusBadRequest, apiCodeInvalidRequest, apiMessageInvalidRequest))
	}
}

// WriteError writes the shared safe API error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, apiError APIError) {
	correlationID := w.Header().Get(telemetry.CorrelationIDHeader)
	if correlationID == "" {
		correlationID = r.Header.Get(telemetry.CorrelationIDHeader)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(apiError.Status)
	_ = json.NewEncoder(w).Encode(struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		CorrelationID string `json:"correlationId"`
	}{
		Code:          apiError.Code,
		Message:       apiError.Message,
		CorrelationID: correlationID,
	})
}
