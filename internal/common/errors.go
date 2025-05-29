package common

import (
	"errors"
	"fmt"
	"strings"
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
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorf wraps an error with formatted context information
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// NewError creates a new error with a formatted message
func NewError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// ValidationError represents validation errors with field-specific information
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigurationError represents configuration-related errors
type ConfigurationError struct {
	Section string
	Field   string
	Reason  string
}

func (e *ConfigurationError) Error() string {
	if e.Section != "" && e.Field != "" {
		return fmt.Sprintf("configuration error in section '%s', field '%s': %s", e.Section, e.Field, e.Reason)
	} else if e.Section != "" {
		return fmt.Sprintf("configuration error in section '%s': %s", e.Section, e.Reason)
	}
	return fmt.Sprintf("configuration error: %s", e.Reason)
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(section, field, reason string) *ConfigurationError {
	return &ConfigurationError{
		Section: section,
		Field:   field,
		Reason:  reason,
	}
}

// NetworkError represents network-related errors
type NetworkError struct {
	URL     string
	Reason  string
	Wrapped error
}

func (e *NetworkError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("network error for '%s': %s: %v", e.URL, e.Reason, e.Wrapped)
	}
	return fmt.Sprintf("network error for '%s': %s", e.URL, e.Reason)
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
		return fmt.Sprintf("HTTP %d error for '%s': %s", e.StatusCode, e.URL, e.Message)
	}
	return fmt.Sprintf("HTTP %d error: %s", e.StatusCode, e.Message)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewHTTPErrorWithURL creates a new HTTP error with URL context
func NewHTTPErrorWithURL(statusCode int, message, url string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		URL:        url,
	}
}

// IsErrorType checks if an error is of a specific type using errors.Is
func IsErrorType(err error, target error) bool {
	return errors.Is(err, target)
}

// HasErrorType checks if any error in the chain matches the target
func HasErrorType(err error, target error) bool {
	return errors.Is(err, target)
}

// GetRootCause returns the root cause of an error by unwrapping all wrapped errors
func GetRootCause(err error) error {
	for {
		wrapped := errors.Unwrap(err)
		if wrapped == nil {
			return err
		}
		err = wrapped
	}
}

// CombineErrors combines multiple errors into a single error with formatted message
func CombineErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	var messages []string
	for _, err := range errors {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	if len(messages) == 0 {
		return nil
	}

	return fmt.Errorf("multiple errors occurred: [%s]", strings.Join(messages, "; "))
}

// ErrorCollector helps collect multiple errors during processing
type ErrorCollector struct {
	errors []error
}

// Add adds an error to the collector
func (ec *ErrorCollector) Add(err error) {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// AddWithContext adds an error with additional context
func (ec *ErrorCollector) AddWithContext(err error, context string) {
	if err != nil {
		ec.errors = append(ec.errors, WrapError(err, context))
	}
}

// HasErrors returns true if any errors were collected
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

// Error returns a combined error from all collected errors
func (ec *ErrorCollector) Error() error {
	return CombineErrors(ec.errors)
}

// Errors returns all collected errors
func (ec *ErrorCollector) Errors() []error {
	return ec.errors
}

// Clear removes all collected errors
func (ec *ErrorCollector) Clear() {
	ec.errors = nil
}
