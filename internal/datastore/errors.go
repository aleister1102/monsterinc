package datastore

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

// Error represents a general error in the datastore library.
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

// NewError creates a new general Error.
func NewError(message string) error {
	return &Error{Message: message}
}

// WrapError wraps an existing error with a message.
func WrapError(err error, message string) error {
	return &Error{Message: message, Err: err}
} 