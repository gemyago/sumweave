package finance

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidTimestampRange = errors.New("invalid timestamp range")

func ValidateRequiredTimestampRange(start time.Time, end time.Time) error {
	if start.IsZero() {
		return fmt.Errorf("%w: start timestamp is required", ErrInvalidTimestampRange)
	}
	if end.IsZero() {
		return fmt.Errorf("%w: end timestamp is required", ErrInvalidTimestampRange)
	}
	if start.After(end) {
		return fmt.Errorf("%w: start timestamp must not be after end timestamp", ErrInvalidTimestampRange)
	}
	return nil
}
