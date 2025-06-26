package monitor

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// BatchURLManager handles URL monitoring in batches
type BatchURLManager struct {
	urlManager     *URLManager
	batchProcessor *common.BatchProcessor
	logger         zerolog.Logger
	batchConfig    config.MonitorBatchConfig
}

// NewBatchURLManager creates a new BatchURLManager with the given config
func NewBatchURLManager(batchConfig config.MonitorBatchConfig, logger zerolog.Logger) *BatchURLManager {
	batchProcessorConfig := batchConfig.ToBatchProcessorConfig()
	batchProcessor := common.NewBatchProcessor(batchProcessorConfig, logger)

	managerLogger := logger.With().Str("component", "BatchURLManager").Logger()
	managerLogger.Info().
		Int("max_concurrent_batch", batchConfig.GetEffectiveMaxConcurrentBatch()).
		Int("batch_size", batchConfig.BatchSize).
		Msg("Monitor batch configuration initialized")

	return &BatchURLManager{
		urlManager:     NewURLManager(logger),
		batchProcessor: batchProcessor,
		logger:         managerLogger,
		batchConfig:    batchConfig,
	}
}

// BatchMonitorResult holds the result of batch monitoring
type BatchMonitorResult struct {
	ProcessedURLs    []string
	ChangedURLs      []string // Add tracking for changed URLs
	BatchResults     []common.BatchResult
	TotalBatches     int
	ProcessedBatches int
	UsedBatching     bool
	InterruptedAt    int // Which batch was interrupted (0 means completed)
}

// BatchMonitorCycleTracker tracks the current state of batch monitoring
type BatchMonitorCycleTracker struct {
	AllURLs          []string
	CurrentBatch     int
	TotalBatches     int
	ProcessedBatches int
	CycleID          string
	UsedBatching     bool
}

// LoadURLsInBatches loads URLs from file and prepares them for batch processing
func (bum *BatchURLManager) LoadURLsInBatches(
	ctx context.Context,
	inputFileOption string,
) (*BatchMonitorCycleTracker, error) {
	bum.logger.Info().
		Str("file", inputFileOption).
		Msg("Loading URLs for batch monitoring")

	// Load URLs using the regular URL manager approach
	err := bum.urlManager.LoadAndMonitorFromSources(inputFileOption)
	if err != nil {
		return nil, WrapError(err, "failed to load monitor URLs from file")
	}

	allURLs := bum.urlManager.GetCurrentURLs()

	// Check if batching is needed
	useBatching := bum.batchProcessor.ShouldUseBatching(len(allURLs))

	var totalBatches int
	if useBatching {
		totalBatches, _ = bum.batchProcessor.GetBatchingStats(len(allURLs))
	} else {
		totalBatches = 1
	}

	bum.logger.Info().
		Int("total_urls", len(allURLs)).
		Bool("use_batching", useBatching).
		Int("total_batches", totalBatches).
		Msg("URLs loaded for batch monitoring")

	return &BatchMonitorCycleTracker{
		AllURLs:          allURLs,
		CurrentBatch:     0,
		TotalBatches:     totalBatches,
		ProcessedBatches: 0,
		CycleID:          "",
		UsedBatching:     useBatching,
	}, nil
}

// GetNextBatch returns the next batch of URLs to monitor
func (bum *BatchURLManager) GetNextBatch(tracker *BatchMonitorCycleTracker) ([]string, bool) {
	if !tracker.UsedBatching {
		// If not using batching, return all URLs once
		if tracker.ProcessedBatches == 0 {
			tracker.ProcessedBatches = 1
			return tracker.AllURLs, false // hasMore = false
		}
		return nil, false
	}

	// Check if we have more batches to process
	if tracker.CurrentBatch >= tracker.TotalBatches {
		return nil, false // No more batches
	}

	// Calculate batch boundaries
	start := tracker.CurrentBatch * bum.batchConfig.BatchSize
	end := start + bum.batchConfig.BatchSize
	if end > len(tracker.AllURLs) {
		end = len(tracker.AllURLs)
	}

	batch := tracker.AllURLs[start:end]

	bum.logger.Info().
		Int("batch_index", tracker.CurrentBatch).
		Int("batch_size", len(batch)).
		Int("start", start).
		Int("end", end).
		Int("total_urls", len(tracker.AllURLs)).
		Msg("Generated monitor batch")

	// Update tracker
	tracker.CurrentBatch++
	hasMore := tracker.CurrentBatch < tracker.TotalBatches

	return batch, hasMore
}

// CompleteCurrentBatch marks the current batch as completed
func (bum *BatchURLManager) CompleteCurrentBatch(tracker *BatchMonitorCycleTracker) {
	tracker.ProcessedBatches++

	bum.logger.Info().
		Int("completed_batch", tracker.ProcessedBatches).
		Int("total_batches", tracker.TotalBatches).
		Bool("has_more", tracker.ProcessedBatches < tracker.TotalBatches).
		Msg("Monitor batch completed")
}

// ExecuteBatchMonitoring executes monitoring for a single batch
func (bum *BatchURLManager) ExecuteBatchMonitoring(
	ctx context.Context,
	urls []string,
	cycleID string,
	urlChecker *URLChecker,
	progressCallback func(processed, failed int),
) (*BatchMonitorResult, error) {
	bum.logger.Info().
		Int("urls_count", len(urls)).
		Str("cycle_id", cycleID).
		Msg("Executing batch monitoring")

	var processedURLs []string
	var changedURLs []string // Track changed URLs
	var failedCount int

	// Calculate total batches for batch info
	useBatching := bum.batchProcessor.ShouldUseBatching(len(urls))
	totalBatches := 1
	if useBatching {
		totalBatches, _ = bum.batchProcessor.GetBatchingStats(len(urls))
	}

	// Process function for monitoring URLs
	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		// Process each URL in the batch with optimized batching
		for i, url := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Create batch info for this batch
			batchInfo := models.NewBatchInfo(
				batchIndex+1, // batchIndex is 0-based, but we want 1-based numbering
				totalBatches,
				len(batch),
				i, // Current position in batch
			)

			// Execute URL check with batch context
			checkResult := urlChecker.CheckURLWithBatchContext(ctx, url, cycleID, batchInfo)
			if checkResult.Success {
				processedURLs = append(processedURLs, url)

				// Track changed URLs
				if checkResult.FileChangeInfo != nil {
					changedURLs = append(changedURLs, url)
				}
			} else {
				failedCount++
			}

			// Update progress callback periodically (every 10 items) to reduce overhead
			if progressCallback != nil && (i%10 == 0 || i == len(batch)-1) {
				progressCallback(len(processedURLs), failedCount)
			}
		}

		return nil
	}

	// Execute batch processing
	batchResults, err := bum.batchProcessor.ProcessBatches(ctx, urls, processFunc)
	if err != nil {
		bum.logger.Error().Err(err).
			Int("processed_urls", len(processedURLs)).
			Int("failed_urls", failedCount).
			Msg("Batch monitoring failed")
		return nil, err
	}

	// Final progress update
	if progressCallback != nil {
		progressCallback(len(processedURLs), failedCount)
	}

	bum.logger.Info().
		Int("batch_index", 0).
		Int("processed_urls", len(processedURLs)).
		Int("failed_urls", failedCount).
		Msg("Monitor batch processing completed")

	// Determine interrupted batch
	interruptedAt := 0
	for i, result := range batchResults {
		if !result.Success {
			interruptedAt = i + 1
			break
		}
	}

	return &BatchMonitorResult{
		ProcessedURLs:    processedURLs,
		ChangedURLs:      changedURLs,
		BatchResults:     batchResults,
		TotalBatches:     len(batchResults),
		ProcessedBatches: len(batchResults),
		UsedBatching:     len(batchResults) > 1,
		InterruptedAt:    interruptedAt,
	}, nil
}

// GetBatchingInfo returns information about how the URLs would be batched
func (bum *BatchURLManager) GetBatchingInfo(urlCount int) (useBatching bool, batchCount int, remainingItems int) {
	useBatching = bum.batchProcessor.ShouldUseBatching(urlCount)
	if useBatching {
		batchCount, remainingItems = bum.batchProcessor.GetBatchingStats(urlCount)
	} else {
		batchCount = 1
		remainingItems = 0
	}
	return
}

// UpdateLogger updates the logger for this component and its URLManager
func (bum *BatchURLManager) UpdateLogger(newLogger zerolog.Logger) {
	bum.logger = newLogger.With().Str("component", "BatchURLManager").Logger()
	bum.urlManager.UpdateLogger(newLogger)
}
