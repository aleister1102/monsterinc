package httpxrunner

import (
	"fmt"
	"strings"
)

// Pre-defined errors for the builder and runner.
var (
	ErrConfigNotSet  = fmt.Errorf("config not set")
	ErrRootURLNotSet = fmt.Errorf("root target URL not set")
)

// ErrBuilderErrors is a custom error type that holds multiple errors
// that occurred during the builder validation process.
type ErrBuilderErrors []error

func (e ErrBuilderErrors) Error() string {
	var errorStrings []string
	for _, err := range e {
		errorStrings = append(errorStrings, err.Error())
	}
	return fmt.Sprintf("builder validation failed: [%s]", strings.Join(errorStrings, ", "))
}

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
