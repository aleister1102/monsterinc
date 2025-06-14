package common

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBatchProcessor(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultBatchProcessorConfig()

	bp := NewBatchProcessor(config, logger)

	assert.NotNil(t, bp)
	assert.Equal(t, config.BatchSize, bp.config.BatchSize)
	assert.Equal(t, config.MaxConcurrentBatch, bp.config.MaxConcurrentBatch)
}

func TestBatchProcessor_ShouldUseBatching(t *testing.T) {
	tests := []struct {
		name           string
		config         BatchProcessorConfig
		inputSize      int
		expectedResult bool
	}{
		{
			name:           "input size below threshold",
			config:         BatchProcessorConfig{ThresholdSize: 500},
			inputSize:      400,
			expectedResult: false,
		},
		{
			name:           "input size at threshold",
			config:         BatchProcessorConfig{ThresholdSize: 500},
			inputSize:      500,
			expectedResult: true,
		},
		{
			name:           "input size above threshold",
			config:         BatchProcessorConfig{ThresholdSize: 500},
			inputSize:      600,
			expectedResult: true,
		},
	}

	logger := zerolog.Nop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := NewBatchProcessor(tt.config, logger)
			result := bp.ShouldUseBatching(tt.inputSize)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestBatchProcessor_SplitIntoBatches(t *testing.T) {
	tests := []struct {
		name            string
		batchSize       int
		input           []string
		expectedBatches int
	}{
		{
			name:            "empty input",
			batchSize:       3,
			input:           []string{},
			expectedBatches: 0,
		},
		{
			name:            "single batch",
			batchSize:       5,
			input:           []string{"a", "b", "c"},
			expectedBatches: 1,
		},
		{
			name:            "multiple batches",
			batchSize:       2,
			input:           []string{"a", "b", "c", "d", "e"},
			expectedBatches: 3,
		},
		{
			name:            "exact batch size",
			batchSize:       3,
			input:           []string{"a", "b", "c", "d", "e", "f"},
			expectedBatches: 2,
		},
	}

	logger := zerolog.Nop()
	config := BatchProcessorConfig{BatchSize: 0} // Will be overridden in test

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.BatchSize = tt.batchSize
			bp := NewBatchProcessor(config, logger)

			batches := bp.SplitIntoBatches(tt.input)
			assert.Equal(t, tt.expectedBatches, len(batches))

			// Verify all items are included
			totalItems := 0
			for _, batch := range batches {
				totalItems += len(batch)
				assert.True(t, len(batch) <= tt.batchSize)
			}
			assert.Equal(t, len(tt.input), totalItems)
		})
	}
}

func TestBatchProcessor_ProcessBatches_Sequential(t *testing.T) {
	logger := zerolog.Nop()
	config := BatchProcessorConfig{
		BatchSize:          2,
		MaxConcurrentBatch: 1, // Sequential processing
		BatchTimeout:       time.Minute,
		ThresholdSize:      1,
	}

	bp := NewBatchProcessor(config, logger)

	input := []string{"a", "b", "c", "d", "e"}
	processedItems := make([]string, 0)

	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		for _, item := range batch {
			processedItems = append(processedItems, item)
		}
		return nil
	}

	ctx := context.Background()
	results, err := bp.ProcessBatches(ctx, input, processFunc)

	require.NoError(t, err)
	assert.Equal(t, 3, len(results)) // 5 items / 2 batch size = 3 batches
	assert.Equal(t, input, processedItems)

	// Verify all results are successful
	for _, result := range results {
		assert.True(t, result.Success)
		assert.NoError(t, result.Error)
		assert.True(t, result.Processed > 0)
	}
}

func TestBatchProcessor_ProcessBatches_Concurrent(t *testing.T) {
	logger := zerolog.Nop()
	config := BatchProcessorConfig{
		BatchSize:          2,
		MaxConcurrentBatch: 3, // Concurrent processing
		BatchTimeout:       time.Minute,
		ThresholdSize:      1,
	}

	bp := NewBatchProcessor(config, logger)

	input := []string{"a", "b", "c", "d", "e", "f"}
	processedCount := 0

	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		processedCount += len(batch)
		return nil
	}

	ctx := context.Background()
	results, err := bp.ProcessBatches(ctx, input, processFunc)

	require.NoError(t, err)
	assert.Equal(t, 3, len(results)) // 6 items / 2 batch size = 3 batches
	assert.Equal(t, 6, processedCount)

	// Verify all results are successful
	for _, result := range results {
		assert.True(t, result.Success)
		assert.NoError(t, result.Error)
		assert.Equal(t, 2, result.Processed) // Each batch has 2 items
	}
}

func TestBatchProcessor_ProcessBatches_WithError(t *testing.T) {
	logger := zerolog.Nop()
	config := BatchProcessorConfig{
		BatchSize:          2,
		MaxConcurrentBatch: 1,
		BatchTimeout:       time.Minute,
		ThresholdSize:      1,
	}

	bp := NewBatchProcessor(config, logger)

	input := []string{"a", "b", "c", "d"}

	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		if batchIndex == 1 { // Second batch fails
			return NewError("batch processing failed")
		}
		return nil
	}

	ctx := context.Background()
	results, err := bp.ProcessBatches(ctx, input, processFunc)

	require.NoError(t, err) // Overall processing doesn't fail
	assert.Equal(t, 2, len(results))

	// First batch should succeed
	assert.True(t, results[0].Success)
	assert.NoError(t, results[0].Error)

	// Second batch should fail
	assert.False(t, results[1].Success)
	assert.Error(t, results[1].Error)
}

func TestBatchProcessor_ProcessBatches_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	config := BatchProcessorConfig{
		BatchSize:          1,
		MaxConcurrentBatch: 1,
		BatchTimeout:       time.Minute,
		ThresholdSize:      1,
	}

	bp := NewBatchProcessor(config, logger)

	input := []string{"a", "b", "c"}

	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		if batchIndex == 1 {
			time.Sleep(100 * time.Millisecond) // Simulate work
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	results, err := bp.ProcessBatches(ctx, input, processFunc)

	// Should return context error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")

	// Some batches might have been processed before cancellation
	assert.True(t, len(results) <= 3)
}

func TestBatchProcessor_GetBatchingStats(t *testing.T) {
	logger := zerolog.Nop()
	config := BatchProcessorConfig{BatchSize: 10}
	bp := NewBatchProcessor(config, logger)

	tests := []struct {
		name              string
		inputSize         int
		expectedBatches   int
		expectedRemaining int
	}{
		{
			name:              "exact multiple",
			inputSize:         20,
			expectedBatches:   2,
			expectedRemaining: 0,
		},
		{
			name:              "with remainder",
			inputSize:         23,
			expectedBatches:   3,
			expectedRemaining: 3,
		},
		{
			name:              "less than batch size",
			inputSize:         5,
			expectedBatches:   1,
			expectedRemaining: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches, remaining := bp.GetBatchingStats(tt.inputSize)
			assert.Equal(t, tt.expectedBatches, batches)
			assert.Equal(t, tt.expectedRemaining, remaining)
		})
	}
}

func TestDefaultBatchProcessorConfig(t *testing.T) {
	config := DefaultBatchProcessorConfig()

	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 1, config.MaxConcurrentBatch)
	assert.Equal(t, 30*time.Minute, config.BatchTimeout)
	assert.Equal(t, 500, config.ThresholdSize)
}
