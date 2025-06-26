package scheduler

import (
	"fmt"
)

// Error represents a general error in the scheduler library.
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