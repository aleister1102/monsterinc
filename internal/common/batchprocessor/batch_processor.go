package batchprocessor

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// BatchProcessorConfig holds configuration for batch processing
type BatchProcessorConfig struct {
	BatchSize          int           // Max items per batch (default: 100)
	MaxConcurrentBatch int           // Max concurrent batches (default: 1 for sequential processing)
	BatchTimeout       time.Duration // Timeout per batch (default: 30 minutes)
	ThresholdSize      int           // Minimum size to trigger batching (default: 500)
}

// DefaultBatchProcessorConfig returns default configuration
func DefaultBatchProcessorConfig() BatchProcessorConfig {
	return BatchProcessorConfig{
		BatchSize:          100,
		MaxConcurrentBatch: 1,
		BatchTimeout:       30 * time.Minute,
		ThresholdSize:      500,
	}
}

// BatchResult holds the result of a batch processing
type BatchResult struct {
	BatchIndex int
	Success    bool
	Error      error
	Processed  int
	Timestamp  time.Time
}

// BatchProcessor handles splitting large datasets into smaller batches
type BatchProcessor struct {
	config BatchProcessorConfig
	logger zerolog.Logger
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(config BatchProcessorConfig, logger zerolog.Logger) *BatchProcessor {
	return &BatchProcessor{
		config: config,
		logger: logger.With().Str("component", "BatchProcessor").Logger(),
	}
}

// ProcessFunc defines the function signature for processing a batch
type ProcessFunc func(ctx context.Context, batch []string, batchIndex int) error

// ShouldUseBatching determines if batching should be used based on input size
func (bp *BatchProcessor) ShouldUseBatching(inputSize int) bool {
	return inputSize > bp.config.ThresholdSize
}

// SplitIntoBatches splits a slice of strings into batches
func (bp *BatchProcessor) SplitIntoBatches(input []string) [][]string {
	if len(input) <= bp.config.BatchSize {
		return [][]string{input}
	}

	var batches [][]string
	for i := 0; i < len(input); i += bp.config.BatchSize {
		end := i + bp.config.BatchSize
		if end > len(input) {
			end = len(input)
		}
		batches = append(batches, input[i:end])
	}

	return batches
}

// ProcessBatches processes all batches sequentially or concurrently based on config
func (bp *BatchProcessor) ProcessBatches(
	ctx context.Context,
	input []string,
	processFunc ProcessFunc,
) ([]BatchResult, error) {
	if !bp.ShouldUseBatching(len(input)) {
		bp.logger.Info().
			Int("input_size", len(input)).
			Int("threshold", bp.config.ThresholdSize).
			Msg("Input size below threshold, processing as single batch")

		err := processFunc(ctx, input, 0)
		result := BatchResult{
			BatchIndex: 0,
			Success:    err == nil,
			Error:      err,
			Processed:  len(input),
			Timestamp:  time.Now(),
		}
		return []BatchResult{result}, err
	}

	batches := bp.SplitIntoBatches(input)
	bp.logger.Info().
		Int("total_items", len(input)).
		Int("batch_count", len(batches)).
		Int("batch_size", bp.config.BatchSize).
		Msg("Starting batch processing")

	if bp.config.MaxConcurrentBatch == 1 {
		return bp.processSequentially(ctx, batches, processFunc)
	}

	return bp.processConcurrently(ctx, batches, processFunc)
}

// processSequentially processes batches one by one
func (bp *BatchProcessor) processSequentially(
	ctx context.Context,
	batches [][]string,
	processFunc ProcessFunc,
) ([]BatchResult, error) {
	results := make([]BatchResult, 0, len(batches))

	for i, batch := range batches {
		select {
		case <-ctx.Done():
			bp.logger.Info().
				Int("completed_batches", i).
				Int("total_batches", len(batches)).
				Msg("Batch processing interrupted by context cancellation")
			return results, ctx.Err()
		default:
		}

		bp.logger.Info().
			Int("batch_index", i).
			Int("batch_size", len(batch)).
			Int("progress", i+1).
			Int("total", len(batches)).
			Msg("Processing batch")

		batchCtx, cancel := context.WithTimeout(ctx, bp.config.BatchTimeout)

		start := time.Now()
		err := processFunc(batchCtx, batch, i)
		duration := time.Since(start)

		result := BatchResult{
			BatchIndex: i,
			Success:    err == nil,
			Error:      err,
			Processed:  len(batch),
			Timestamp:  time.Now(),
		}
		results = append(results, result)

		bp.logger.Info().
			Int("batch_index", i).
			Bool("success", err == nil).
			Dur("duration", duration).
			Int("processed", len(batch)).
			Msg("Batch processing completed")

		cancel()

		if err != nil {
			bp.logger.Error().
				Err(err).
				Int("batch_index", i).
				Msg("Batch processing failed")
			// Continue processing other batches even if one fails
		}
	}

	return results, nil
}

// processConcurrently processes batches concurrently with limit
func (bp *BatchProcessor) processConcurrently(
	ctx context.Context,
	batches [][]string,
	processFunc ProcessFunc,
) ([]BatchResult, error) {
	semaphore := make(chan struct{}, bp.config.MaxConcurrentBatch)
	results := make([]BatchResult, len(batches))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, batch := range batches {
		select {
		case <-ctx.Done():
			bp.logger.Info().
				Int("started_batches", i).
				Int("total_batches", len(batches)).
				Msg("Batch processing interrupted by context cancellation")
			return results[:i], ctx.Err()
		case semaphore <- struct{}{}:
		}

		wg.Add(1)
		go func(batchIndex int, batchData []string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			bp.logger.Info().
				Int("batch_index", batchIndex).
				Int("batch_size", len(batchData)).
				Msg("Starting concurrent batch processing")

			batchCtx, cancel := context.WithTimeout(ctx, bp.config.BatchTimeout)
			defer cancel()

			start := time.Now()
			err := processFunc(batchCtx, batchData, batchIndex)
			duration := time.Since(start)

			result := BatchResult{
				BatchIndex: batchIndex,
				Success:    err == nil,
				Error:      err,
				Processed:  len(batchData),
				Timestamp:  time.Now(),
			}

			mu.Lock()
			results[batchIndex] = result
			mu.Unlock()

			bp.logger.Info().
				Int("batch_index", batchIndex).
				Bool("success", err == nil).
				Dur("duration", duration).
				Int("processed", len(batchData)).
				Msg("Concurrent batch processing completed")

			if err != nil {
				bp.logger.Error().
					Err(err).
					Int("batch_index", batchIndex).
					Msg("Concurrent batch processing failed")
			}
		}(i, batch)
	}

	wg.Wait()
	return results, nil
}

// GetBatchingStats returns statistics about batch processing
func (bp *BatchProcessor) GetBatchingStats(inputSize int) (batches int, remainingItems int) {
	if !bp.ShouldUseBatching(inputSize) {
		return 1, 0
	}

	batches = (inputSize + bp.config.BatchSize - 1) / bp.config.BatchSize
	remainingItems = inputSize % bp.config.BatchSize
	if remainingItems == 0 && inputSize > 0 {
		remainingItems = bp.config.BatchSize
	}

	return batches, remainingItems
}
