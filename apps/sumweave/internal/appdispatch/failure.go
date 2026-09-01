package appdispatch

import (
	"errors"
)

// BusinessFailureError is an explicitly handled, terminal processing failure. Its
// fields are safe for an optional job visibility projection.
type BusinessFailureError struct {
	cause   error
	Code    string
	Summary string
	Details string
}

func NewBusinessFailure(cause error, code, summary, details string) error {
	if cause == nil {
		return nil
	}
	return &BusinessFailureError{cause: cause, Code: code, Summary: summary, Details: details}
}

func (e *BusinessFailureError) Error() string { return e.cause.Error() }

func (e *BusinessFailureError) Unwrap() error { return e.cause }

func BusinessFailureFrom(err error) (*BusinessFailureError, bool) {
	var failure *BusinessFailureError
	if !errors.As(err, &failure) {
		return nil, false
	}
	return failure, true
}
