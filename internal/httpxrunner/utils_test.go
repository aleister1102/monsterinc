package telescope_test

import (
	"context"
	"testing"

	"github.com/aleister1102/go-telescope"
	"github.com/stretchr/testify/assert"
)

func TestIsContextCancelled(t *testing.T) {
	t.Run("Context Not Cancelled", func(t *testing.T) {
		ctx := context.Background()
		err := telescope.IsContextCancelled(ctx)
		assert.NoError(t, err, "IsContextCancelled should return nil for a context that is not cancelled")
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel the context

		err := telescope.IsContextCancelled(ctx)
		assert.Error(t, err, "IsContextCancelled should return an error for a cancelled context")
		assert.ErrorIs(t, err, context.Canceled, "The error should wrap context.Canceled")
		assert.Contains(t, err.Error(), "context cancelled", "The error message should indicate that the context was cancelled")
	})
}
