package contextutils

import (
	"context"
	"strings"

	"slices"

	"github.com/rs/zerolog"
)

// ContextCheckResult represents the result of a context cancellation check
type ContextCheckResult struct {
	Cancelled bool
	Error     error
}

// CheckCancellationWithLog checks for context cancellation and logs if cancelled
func CheckCancellationWithLog(ctx context.Context, logger zerolog.Logger, operation string) ContextCheckResult {
	result := CheckCancellation(ctx)
	if result.Cancelled {
		logger.Info().Str("operation", operation).Msg("Context cancelled")
	}
	return result
}

// CheckCancellation checks if the context is cancelled and returns appropriate result
func CheckCancellation(ctx context.Context) ContextCheckResult {
	select {
	case <-ctx.Done():
		return ContextCheckResult{
			Cancelled: true,
			Error:     ctx.Err(),
		}
	default:
		return ContextCheckResult{
			Cancelled: false,
			Error:     nil,
		}
	}
}

// ContainsCancellationError checks if error messages contain cancellation-related errors
func ContainsCancellationError(messages []string) bool {
	return slices.ContainsFunc(messages, containsCancellationKeywords)
}

// containsCancellationKeywords checks for cancellation keywords in a message
func containsCancellationKeywords(message string) bool {
	keywords := []string{
		"context canceled",
		"context deadline exceeded",
		"operation interrupted",
		"cancelled",
		"canceled",
	}

	lowerMessage := strings.ToLower(message)
	for _, keyword := range keywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}
