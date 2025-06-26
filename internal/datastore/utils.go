package datastore

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// CancellationResult holds information about context cancellation
type CancellationResult struct {
	Cancelled bool
	Error     error
}

// CheckCancellationWithLog checks if context is cancelled and logs the event
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