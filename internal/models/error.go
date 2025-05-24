package models

import "fmt"

// URLValidationError represents an error during URL validation.
// It is moved here from the urlhandler package to centralize models.
type URLValidationError struct {
	URL     string
	Message string
}

// Error returns the error message for URLValidationError.
func (e *URLValidationError) Error() string {
	return fmt.Sprintf("invalid URL %s: %s", e.URL, e.Message)
}

// TODO: Add other common error structs here as they are identified.
