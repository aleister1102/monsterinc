package httpxrunner_test

import (
	"errors"
	"fmt"
	"testing"

	telescope "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/stretchr/testify/assert"
)

func TestErrBuilderErrors_Error(t *testing.T) {
	testCases := []struct {
		name     string
		errs     telescope.ErrBuilderErrors
		expected string
	}{
		{
			name:     "single error",
			errs:     telescope.ErrBuilderErrors{errors.New("first error")},
			expected: "builder validation failed: [first error]",
		},
		{
			name: "multiple errors",
			errs: telescope.ErrBuilderErrors{
				errors.New("first error"),
				errors.New("second error"),
			},
			expected: "builder validation failed: [first error, second error]",
		},
		{
			name:     "no errors",
			errs:     telescope.ErrBuilderErrors{},
			expected: "builder validation failed: []",
		},
		{
			name: "pre-defined errors",
			errs: telescope.ErrBuilderErrors{
				telescope.ErrConfigNotSet,
				telescope.ErrRootURLNotSet,
			},
			expected: fmt.Sprintf(
				"builder validation failed: [%s, %s]",
				telescope.ErrConfigNotSet,
				telescope.ErrRootURLNotSet,
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.errs.Error())
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	testCases := []struct {
		name     string
		err      *telescope.ValidationError
		expected string
	}{
		{
			name: "Simple validation error",
			err: &telescope.ValidationError{
				Field:   "email",
				Value:   "not-an-email",
				Message: "invalid format",
			},
			expected: "validation error on field 'email' (value: not-an-email): invalid format",
		},
		{
			name: "Error with empty message",
			err: &telescope.ValidationError{
				Field:   "password",
				Value:   "123",
				Message: "",
			},
			expected: "validation error on field 'password' (value: 123): ",
		},
		{
			name: "Error with integer value",
			err: &telescope.ValidationError{
				Field:   "age",
				Value:   -5,
				Message: "must be positive",
			},
			expected: "validation error on field 'age' (value: -5): must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.err.Error())
		})
	}
}

func TestNewValidationError(t *testing.T) {
	err := telescope.NewValidationError("username", "user!", "contains invalid characters")

	// Check that the returned error is of the correct type
	validationErr, ok := err.(*telescope.ValidationError)
	assert.True(t, ok, "NewValidationError should return a *ValidationError")

	// Check the fields
	assert.Equal(t, "username", validationErr.Field)
	assert.Equal(t, "user!", validationErr.Value)
	assert.Equal(t, "contains invalid characters", validationErr.Message)

	// Check the error message format via the Error() method
	expectedMsg := fmt.Sprintf("validation error on field '%s' (value: %v): %s", "username", "user!", "contains invalid characters")
	assert.Equal(t, expectedMsg, err.Error())
}
