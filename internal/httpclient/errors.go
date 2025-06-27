package httpclient

import (
	"fmt"
)

// ValidationError represents an error during validation.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s' (value: %v): %s", e.Field, e.Value, e.Message)
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field string, value interface{}, message string) error {
	return &ValidationError{Field: field, Value: value, Message: message}
}

// Error represents a general error in the httpclient library.
type Error struct {
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new general Error.
func NewError(message string) error {
	return &Error{Message: message}
}

// WrapError wraps an existing error with a message.
func WrapError(err error, message string) error {
	return &Error{Message: message, Err: err}
}

// NetworkError represents a network-level error.
type NetworkError struct {
	URL     string
	Message string
	Err     error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error for URL '%s': %s: %v", e.URL, e.Message, e.Err)
}

// Unwrap returns the underlying error.
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// NewNetworkError creates a new NetworkError.
func NewNetworkError(url, message string, err error) error {
	return &NetworkError{URL: url, Message: message, Err: err}
}

// HTTPError represents an HTTP-level error (non-2xx status code).
type HTTPError struct {
	StatusCode int
	Body       string
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http error for URL '%s': status %d, body: %s", e.URL, e.StatusCode, e.Body)
}

// NewHTTPErrorWithURL creates a new HTTPError.
func NewHTTPErrorWithURL(statusCode int, body string, url string) error {
	return &HTTPError{StatusCode: statusCode, Body: body, URL: url}
}
