package context

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCheckCancellationWithLog(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name        string
		ctx         context.Context
		operation   string
		expectError bool
	}{
		{
			name:        "normal context",
			ctx:         context.Background(),
			operation:   "test_operation",
			expectError: false,
		},
		{
			name:        "cancelled context",
			ctx:         createCancelledContext(),
			operation:   "test_operation",
			expectError: true,
		},
		{
			name:        "deadline exceeded context",
			ctx:         createDeadlineExceededContext(),
			operation:   "test_operation",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCancellationWithLog(tt.ctx, logger, tt.operation)

			if tt.expectError {
				assert.True(t, result.Cancelled)
				assert.Error(t, result.Error)
			} else {
				assert.False(t, result.Cancelled)
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestCheckCancellation(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectError bool
	}{
		{
			name:        "normal context",
			ctx:         context.Background(),
			expectError: false,
		},
		{
			name:        "cancelled context",
			ctx:         createCancelledContext(),
			expectError: true,
		},
		{
			name:        "deadline exceeded context",
			ctx:         createDeadlineExceededContext(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCancellation(tt.ctx)

			if tt.expectError {
				assert.True(t, result.Cancelled)
				assert.Error(t, result.Error)
			} else {
				assert.False(t, result.Cancelled)
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestContainsCancellationError(t *testing.T) {
	tests := []struct {
		name        string
		messages    []string
		expectFound bool
	}{
		{
			name:        "empty messages",
			messages:    []string{},
			expectFound: false,
		},
		{
			name:        "no cancellation keywords",
			messages:    []string{"normal error", "timeout error", "connection failed"},
			expectFound: false,
		},
		{
			name:        "contains context canceled",
			messages:    []string{"normal error", "context canceled", "another error"},
			expectFound: true,
		},
		{
			name:        "contains context cancelled",
			messages:    []string{"error: context cancelled"},
			expectFound: true,
		},
		{
			name:        "contains deadline exceeded",
			messages:    []string{"context deadline exceeded"},
			expectFound: true,
		},
		{
			name:        "contains operation was canceled",
			messages:    []string{"operation was canceled by user"},
			expectFound: true,
		},
		{
			name:        "contains operation was cancelled",
			messages:    []string{"the operation was cancelled"},
			expectFound: true,
		},
		{
			name:        "case insensitive matching",
			messages:    []string{"CONTEXT CANCELED", "Context Deadline Exceeded"},
			expectFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsCancellationError(tt.messages)
			assert.Equal(t, tt.expectFound, result)
		})
	}
}

func TestContainsCancellationKeywords(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		expectFound bool
	}{
		{
			name:        "empty message",
			message:     "",
			expectFound: false,
		},
		{
			name:        "normal message",
			message:     "this is a normal error message",
			expectFound: false,
		},
		{
			name:        "context canceled",
			message:     "context canceled",
			expectFound: true,
		},
		{
			name:        "context cancelled",
			message:     "context cancelled",
			expectFound: true,
		},
		{
			name:        "deadline exceeded",
			message:     "context deadline exceeded",
			expectFound: true,
		},
		{
			name:        "operation was canceled",
			message:     "the operation was canceled",
			expectFound: true,
		},
		{
			name:        "operation was cancelled",
			message:     "operation was cancelled by user",
			expectFound: true,
		},
		{
			name:        "case insensitive",
			message:     "CONTEXT CANCELED",
			expectFound: true,
		},
		{
			name:        "partial match in longer message",
			message:     "error occurred: context deadline exceeded - please retry",
			expectFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsCancellationKeywords(tt.message)
			assert.Equal(t, tt.expectFound, result)
		})
	}
}

// Helper functions to create test contexts
func createCancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func createDeadlineExceededContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()
	return ctx
}
