package errorwrapper

import (
	"errors"
	"fmt"
)

// Common error types used across the application
var (
	// ErrInvalidInput indicates invalid user input
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound indicates a resource was not found
	ErrNotFound = errors.New("not found")
	// ErrPermissionDenied indicates access permission issues
	ErrPermissionDenied = errors.New("permission denied")
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
	// ErrNetworkFailure indicates network connectivity issues
	ErrNetworkFailure = errors.New("network failure")
	// ErrInvalidConfiguration indicates configuration issues
	ErrInvalidConfiguration = errors.New("invalid configuration")
	// ErrServiceUnavailable indicates a service is not available
	ErrServiceUnavailable = errors.New("service unavailable")
)

// WrapError wraps an error with additional context information
func WrapError(err error, message string) error {
	if err == nil {
		return fmt.Errorf("%s: <nil>", message)
	}
	return fmt.Errorf("%s: %w", message, err)
}

// NewError creates a new error with a formatted message
func NewError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// ValidationError represents validation errors with field-specific information
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value any, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NetworkError represents network-related errors
type NetworkError struct {
	URL     string
	Reason  string
	Wrapped error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error for URL '%s': %s", e.URL, e.Reason)
}

func (e *NetworkError) Unwrap() error {
	return e.Wrapped
}

// NewNetworkError creates a new network error
func NewNetworkError(url, reason string, wrapped error) *NetworkError {
	return &NetworkError{
		URL:     url,
		Reason:  reason,
		Wrapped: wrapped,
	}
}

// HTTPError represents HTTP-related errors
type HTTPError struct {
	StatusCode int
	Message    string
	URL        string
}

func (e *HTTPError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("HTTP %d error for URL '%s': %s", e.StatusCode, e.URL, e.Message)
	}
	return fmt.Sprintf("HTTP %d error: %s", e.StatusCode, e.Message)
}

// NewHTTPErrorWithURL creates a new HTTP error with URL context
func NewHTTPErrorWithURL(statusCode int, message, url string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		URL:        url,
	}
}
