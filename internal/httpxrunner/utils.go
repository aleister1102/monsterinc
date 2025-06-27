package httpxrunner

import (
	"context"
	"fmt"
)

// IsContextCancelled checks if the context has been cancelled. If so, it returns
// a cancellation error. Otherwise, it returns nil.
func IsContextCancelled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
		return nil
	}
}
