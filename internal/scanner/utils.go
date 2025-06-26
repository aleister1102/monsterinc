package scanner

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// CancellationResult represents the result of a cancellation check.
type CancellationResult struct {
	Cancelled bool
	Error     error
}

func CheckCancellationWithLog(ctx context.Context, logger zerolog.Logger, componentName string) CancellationResult {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		logger.Info().Err(err).Str("component", componentName).Msg("Context cancelled")
		return CancellationResult{
			Cancelled: true,
			Error:     fmt.Errorf("%s cancelled: %w", componentName, err),
		}
	default:
		return CancellationResult{Cancelled: false}
	}
}

// ContainsCancellationError checks if a list of error messages contains a cancellation error.
func ContainsCancellationError(errorMessages []string) bool {
	for _, msg := range errorMessages {
		if strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded") {
			return true
		}
	}
	return false
}

// CheckCancellation checks if context is cancelled and returns a CancellationResult
func CheckCancellation(ctx context.Context) CancellationResult {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		return CancellationResult{
			Cancelled: true,
			Error:     fmt.Errorf("context cancelled: %w", err),
		}
	default:
		return CancellationResult{Cancelled: false}
	}
}
