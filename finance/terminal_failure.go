package finance

import "errors"

// TerminalFailureError is a finance-owned, handled outcome that must not be retried
// by the durable command transport. Its fields are safe for job visibility.
type TerminalFailureError struct {
	cause   error
	Code    string
	Summary string
	Details string
}

func NewTerminalFailure(cause error, code, summary, details string) error {
	if cause == nil {
		return nil
	}
	return &TerminalFailureError{
		cause: cause, Code: code, Summary: summary, Details: details,
	}
}

func (e *TerminalFailureError) Error() string { return e.cause.Error() }

func (e *TerminalFailureError) Unwrap() error { return e.cause }

func TerminalFailureFrom(err error) (*TerminalFailureError, bool) {
	var failure *TerminalFailureError
	if !errors.As(err, &failure) {
		return nil, false
	}
	return failure, true
}
