package common

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapError(t *testing.T) {
	tests := []struct {
		name            string
		originalError   error
		message         string
		expectedMessage string
	}{
		{
			name:            "wrap simple error",
			originalError:   errors.New("original error"),
			message:         "wrapper message",
			expectedMessage: "wrapper message: original error",
		},
		{
			name:            "wrap nil error",
			originalError:   nil,
			message:         "wrapper message",
			expectedMessage: "wrapper message: <nil>",
		},
		{
			name:            "empty wrapper message",
			originalError:   errors.New("original error"),
			message:         "",
			expectedMessage: ": original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrappedError := WrapError(tt.originalError, tt.message)
			assert.Error(t, wrappedError)
			assert.Equal(t, tt.expectedMessage, wrappedError.Error())
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name            string
		format          string
		args            []interface{}
		expectedMessage string
	}{
		{
			name:            "simple message",
			format:          "simple error message",
			args:            nil,
			expectedMessage: "simple error message",
		},
		{
			name:            "formatted message",
			format:          "error with value: %d",
			args:            []interface{}{42},
			expectedMessage: "error with value: 42",
		},
		{
			name:            "multiple arguments",
			format:          "error: %s occurred at %s",
			args:            []interface{}{"connection failed", "localhost:8080"},
			expectedMessage: "error: connection failed occurred at localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.format, tt.args...)
			assert.Error(t, err)
			assert.Equal(t, tt.expectedMessage, err.Error())
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name            string
		field           string
		value           interface{}
		message         string
		expectedMessage string
	}{
		{
			name:            "string field validation",
			field:           "username",
			value:           "test@example.com",
			message:         "must be a valid username",
			expectedMessage: "validation error: field 'username' with value 'test@example.com': must be a valid username",
		},
		{
			name:            "numeric field validation",
			field:           "age",
			value:           -5,
			message:         "must be positive",
			expectedMessage: "validation error: field 'age' with value '-5': must be positive",
		},
		{
			name:            "nil value validation",
			field:           "required_field",
			value:           nil,
			message:         "cannot be nil",
			expectedMessage: "validation error: field 'required_field' with value '<nil>': cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr := NewValidationError(tt.field, tt.value, tt.message)

			assert.Error(t, validationErr)
			assert.Equal(t, tt.expectedMessage, validationErr.Error())
			assert.Equal(t, tt.field, validationErr.Field)
			assert.Equal(t, tt.value, validationErr.Value)
			assert.Equal(t, tt.message, validationErr.Message)
		})
	}
}

func TestNetworkError(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		reason          string
		wrappedError    error
		expectedMessage string
	}{
		{
			name:            "simple network error",
			url:             "https://example.com",
			reason:          "connection timeout",
			wrappedError:    nil,
			expectedMessage: "network error for URL 'https://example.com': connection timeout",
		},
		{
			name:            "network error with wrapped error",
			url:             "https://api.example.com/data",
			reason:          "DNS resolution failed",
			wrappedError:    errors.New("no such host"),
			expectedMessage: "network error for URL 'https://api.example.com/data': DNS resolution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkErr := NewNetworkError(tt.url, tt.reason, tt.wrappedError)

			assert.Error(t, networkErr)
			assert.Equal(t, tt.expectedMessage, networkErr.Error())
			assert.Equal(t, tt.url, networkErr.URL)
			assert.Equal(t, tt.reason, networkErr.Reason)
			assert.Equal(t, tt.wrappedError, networkErr.Wrapped)

			// Test Unwrap method
			unwrappedErr := networkErr.Unwrap()
			assert.Equal(t, tt.wrappedError, unwrappedErr)
		})
	}
}

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		message         string
		url             string
		expectedMessage string
	}{
		{
			name:            "not found error",
			statusCode:      http.StatusNotFound,
			message:         "resource not found",
			url:             "https://example.com/api/users/123",
			expectedMessage: "HTTP 404 error for URL 'https://example.com/api/users/123': resource not found",
		},
		{
			name:            "server error",
			statusCode:      http.StatusInternalServerError,
			message:         "internal server error",
			url:             "https://api.example.com/data",
			expectedMessage: "HTTP 500 error for URL 'https://api.example.com/data': internal server error",
		},
		{
			name:            "unauthorized error",
			statusCode:      http.StatusUnauthorized,
			message:         "authentication required",
			url:             "https://secure.example.com/profile",
			expectedMessage: "HTTP 401 error for URL 'https://secure.example.com/profile': authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := NewHTTPErrorWithURL(tt.statusCode, tt.message, tt.url)

			assert.Error(t, httpErr)
			assert.Equal(t, tt.expectedMessage, httpErr.Error())
			assert.Equal(t, tt.statusCode, httpErr.StatusCode)
			assert.Equal(t, tt.message, httpErr.Message)
			assert.Equal(t, tt.url, httpErr.URL)
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Test error chaining with custom errors
	originalErr := errors.New("database connection failed")
	networkErr := NewNetworkError("postgres://localhost:5432", "connection refused", originalErr)
	wrappedErr := WrapError(networkErr, "failed to initialize service")

	// Verify the error chain
	assert.Error(t, wrappedErr)
	assert.Contains(t, wrappedErr.Error(), "failed to initialize service")
	assert.Contains(t, wrappedErr.Error(), "network error")

	// Verify unwrapping works
	var netErr *NetworkError
	assert.True(t, errors.As(wrappedErr, &netErr))
	assert.Equal(t, "postgres://localhost:5432", netErr.URL)
	assert.Equal(t, originalErr, netErr.Unwrap())
}

func TestErrorTypeAssertions(t *testing.T) {
	validationErr := NewValidationError("email", "invalid-email", "must be valid email format")
	networkErr := NewNetworkError("https://example.com", "timeout", nil)
	httpErr := NewHTTPErrorWithURL(404, "not found", "https://example.com/api")

	// Test ValidationError type assertion
	var vErr *ValidationError
	assert.True(t, errors.As(validationErr, &vErr))
	assert.Equal(t, "email", vErr.Field)

	// Test NetworkError type assertion
	var nErr *NetworkError
	assert.True(t, errors.As(networkErr, &nErr))
	assert.Equal(t, "https://example.com", nErr.URL)

	// Test HTTPError type assertion
	var hErr *HTTPError
	assert.True(t, errors.As(httpErr, &hErr))
	assert.Equal(t, 404, hErr.StatusCode)

	// Test negative cases
	assert.False(t, errors.As(validationErr, &nErr))
	assert.False(t, errors.As(networkErr, &hErr))
	assert.False(t, errors.As(httpErr, &vErr))
}
