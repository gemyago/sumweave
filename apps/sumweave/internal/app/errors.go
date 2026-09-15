package app

import "fmt"

// Domain errors are application-level failures that carry rich, structured context
// for logging, monitoring, and generating specific API responses.
// Unlike sentinel errors, these custom error types allow programmatic inspection
// using errors.As to access their fields.

// NotFoundError represents a failure to locate a specific resource.
// It carries both the resource type (e.g., "user", "pet") and the identifier used in the lookup.
type NotFoundError struct {
	Resource string // The type of resource (e.g., "user", "order")
	ID       string // The unique identifier used in the lookup
}

// NewErrNotFound creates a new NotFoundError.
// Use clear, descriptive parameter names to prevent positional confusion.
func NewErrNotFound(resourceType string, resourceID string) *NotFoundError {
	return &NotFoundError{
		Resource: resourceType,
		ID:       resourceID,
	}
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// InvalidInputError represents a validation failure for a specific field.
// It carries the field name and the specific reason for the failure.
type InvalidInputError struct {
	Field  string // The name of the field that failed validation (e.g., "email")
	Reason string // The specific cause of the failure (e.g., "cannot be empty")
}

// NewErrInvalidInput creates a new InvalidInputError.
// Use clear, descriptive parameter names to prevent positional confusion.
func NewErrInvalidInput(fieldName string, failureReason string) *InvalidInputError {
	return &InvalidInputError{
		Field:  fieldName,
		Reason: failureReason,
	}
}

// Error implements the error interface.
func (e *InvalidInputError) Error() string {
	return fmt.Sprintf("invalid input for field '%s': %s", e.Field, e.Reason)
}

// ConflictError represents a business rule violation due to a conflict with existing data.
// It carries the resource type and the reason for the conflict.
type ConflictError struct {
	Resource string // The type of resource (e.g., "user", "email")
	Reason   string // The specific cause of the conflict (e.g., "already exists")
}

// NewErrConflict creates a new ConflictError.
// Use clear, descriptive parameter names to prevent positional confusion.
func NewErrConflict(resourceType string, conflictReason string) *ConflictError {
	return &ConflictError{
		Resource: resourceType,
		Reason:   conflictReason,
	}
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict with %s: %s", e.Resource, e.Reason)
}

// UnauthorizedError indicates the caller is not authenticated or not allowed to access the resource.
type UnauthorizedError struct {
	Reason string
}

// NewErrUnauthorized creates a new UnauthorizedError.
func NewErrUnauthorized(reason string) *UnauthorizedError {
	return &UnauthorizedError{Reason: reason}
}

// Error implements the error interface.
func (e *UnauthorizedError) Error() string {
	return e.Reason
}

// ForbiddenError indicates an authenticated caller lacks required permission.
type ForbiddenError struct {
	TenantAccessDenied bool
}

// NewErrForbidden creates a generic permission denial.
func NewErrForbidden() *ForbiddenError { return &ForbiddenError{} }

// NewErrTenantAccessDenied creates a tenant-membership permission denial.
func NewErrTenantAccessDenied() *ForbiddenError { return &ForbiddenError{TenantAccessDenied: true} }

// Error implements the error interface.
func (e *ForbiddenError) Error() string {
	if e.TenantAccessDenied {
		return "tenant access denied"
	}
	return "insufficient permission"
}

// IdempotencyConflictError identifies a retry with changed semantics.
type IdempotencyConflictError struct{}

// NewErrIdempotencyConflict creates an idempotency conflict.
func NewErrIdempotencyConflict() *IdempotencyConflictError { return &IdempotencyConflictError{} }

// Error implements the error interface.
func (*IdempotencyConflictError) Error() string { return "idempotency conflict" }
