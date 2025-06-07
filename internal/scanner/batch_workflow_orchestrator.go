package scanner

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// BatchWorkflowOrchestrator handles batch processing for scan operations
type BatchWorkflowOrchestrator struct {
	logger         zerolog.Logger
	batchProcessor *common.BatchProcessor
	scanner        *Scanner
	targetManager  *urlhandler.TargetManager
}

// NewBatchWorkflowOrchestrator creates a new batch workflow orchestrator
func NewBatchWorkflowOrchestrator(
	gCfg *config.GlobalConfig,
	scanner *Scanner,
	logger zerolog.Logger,
) *BatchWorkflowOrchestrator {
	bpConfig := gCfg.ScanBatchConfig.ToBatchProcessorConfig()

	return &BatchWorkflowOrchestrator{
		logger:         logger.With().Str("component", "BatchWorkflowOrchestrator").Logger(),
		batchProcessor: common.NewBatchProcessor(bpConfig, logger),
		scanner:        scanner,
		targetManager:  urlhandler.NewTargetManager(logger),
	}
}

// BatchScanResult holds the result of batch scan processing
type BatchScanResult struct {
	SummaryData      models.ScanSummaryData
	ReportFilePaths  []string
	BatchResults     []common.BatchResult
	TotalBatches     int
	ProcessedBatches int
	UsedBatching     bool
	InterruptedAt    int // Which batch was interrupted (0 means completed)
}

// ExecuteBatchScan executes scan workflow in batches
func (bwo *BatchWorkflowOrchestrator) ExecuteBatchScan(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	scanTargetsFile string,
	scanSessionID string,
	targetSource string,
	scanMode string,
) (*BatchScanResult, error) {
	bwo.logger.Info().
		Str("targets_file", scanTargetsFile).
		Str("session_id", scanSessionID).
		Str("mode", scanMode).
		Msg("Starting batch scan execution")

	// Validate inputs
	if gCfg == nil {
		return nil, common.NewError("global config cannot be nil")
	}
	if scanTargetsFile == "" {
		return nil, common.NewError("scan targets file cannot be empty")
	}
	if scanSessionID == "" {
		return nil, common.NewError("scan session ID cannot be empty")
	}

	// Load targets from file
	targets, determinedSource, err := bwo.targetManager.LoadAndSelectTargets(scanTargetsFile)
	if err != nil {
		return nil, common.WrapError(err, "failed to load scan targets")
	}

	if len(targets) == 0 {
		return nil, common.NewError("no valid targets found in source: %s", determinedSource)
	}

	targetURLs := bwo.targetManager.GetTargetStrings(targets)

	// Log target loading info
	bwo.logger.Info().
		Int("total_targets_loaded", len(targetURLs)).
		Str("source", determinedSource).
		Msg("Successfully loaded targets from file")

	// Check if batching is needed
	useBatching := bwo.batchProcessor.ShouldUseBatching(len(targetURLs))

	if !useBatching {
		bwo.logger.Info().
			Int("target_count", len(targetURLs)).
			Int("threshold", gCfg.ScanBatchConfig.ThresholdSize).
			Msg("Target count below batching threshold, processing all at once")

		// Execute single scan workflow
		summaryData, _, reportPaths, err := bwo.scanner.ExecuteSingleScanWorkflowWithReporting(
			ctx,
			gCfg,
			bwo.logger,
			targetURLs,
			scanSessionID,
			targetSource,
			scanMode,
		)

		return &BatchScanResult{
			SummaryData:      summaryData,
			ReportFilePaths:  reportPaths,
			BatchResults:     []common.BatchResult{},
			TotalBatches:     1,
			ProcessedBatches: 1,
			UsedBatching:     false,
			InterruptedAt:    0,
		}, err
	}

	return bwo.executeBatchedScan(ctx, gCfg, targetURLs, scanSessionID, targetSource, scanMode)
}

// executeBatchedScan executes scan in batches
func (bwo *BatchWorkflowOrchestrator) executeBatchedScan(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	targetURLs []string,
	scanSessionID string,
	targetSource string,
	scanMode string,
) (*BatchScanResult, error) {
	batchCount, _ := bwo.batchProcessor.GetBatchingStats(len(targetURLs))

	bwo.logger.Info().
		Int("total_targets", len(targetURLs)).
		Int("batch_count", batchCount).
		Msg("Starting batched scan execution")

	// Update progress display - initialize batch processing
	if bwo.scanner.progressDisplay != nil {
		bwo.scanner.progressDisplay.UpdateScanProgress(0, int64(batchCount), "Batch Processing", fmt.Sprintf("Starting batch scan: %d batches", batchCount))
		bwo.scanner.progressDisplay.UpdateBatchProgress(common.ProgressTypeScan, 0, batchCount)
	}

	// Optimize configuration for memory efficiency during batch processing
	batchSize, _ := bwo.batchProcessor.GetBatchingStats(len(targetURLs))
	bwo.optimizeConfigForMemoryEfficiency(gCfg, batchSize)

	var allReportPaths []string
	var aggregatedSummary models.ScanSummaryData
	var lastBatchError error
	processedBatches := 0
	interruptedAt := 0

	// Initialize summary data
	aggregatedSummary = models.GetDefaultScanSummaryData()
	aggregatedSummary.ScanSessionID = scanSessionID
	aggregatedSummary.TargetSource = targetSource
	aggregatedSummary.ScanMode = scanMode
	aggregatedSummary.Targets = targetURLs
	aggregatedSummary.TotalTargets = len(targetURLs)

	// Process function for each batch
	processFunc := func(ctx context.Context, batch []string, batchIndex int) error {
		batchNumber := batchIndex + 1 // Make it 1-based for display

		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_number", batchNumber).
			Int("batch_size", len(batch)).
			Int("progress", batchNumber).
			Int("total", batchCount).
			Msg("Processing scan batch")

		// Update progress display - current batch progress
		if bwo.scanner.progressDisplay != nil {
			bwo.scanner.progressDisplay.UpdateScanProgress(
				int64(batchNumber-1),
				int64(batchCount),
				"Batch Processing",
				fmt.Sprintf("Processing batch %d/%d (%d targets)", batchNumber, batchCount, len(batch)),
			)
			bwo.scanner.progressDisplay.UpdateBatchProgress(common.ProgressTypeScan, batchNumber-1, batchCount)
		}

		// Log memory usage before batch
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		bwo.logger.Debug().
			Uint64("alloc_mb", memStats.Alloc/1024/1024).
			Uint64("sys_mb", memStats.Sys/1024/1024).
			Int("batch_index", batchIndex).
			Msg("Memory before batch processing")

		// Create batch-specific session ID
		batchSessionID := fmt.Sprintf("%s-batch-%d", scanSessionID, batchIndex)

		// Execute scan for this batch
		batchSummary, _, batchReportPaths, err := bwo.scanner.ExecuteSingleScanWorkflowWithReporting(
			ctx,
			gCfg,
			bwo.logger,
			batch,
			batchSessionID,
			targetSource,
			scanMode,
		)

		if err != nil {
			bwo.logger.Error().
				Err(err).
				Int("batch_index", batchIndex).
				Int("batch_number", batchNumber).
				Msg("Batch scan failed")

			// Update progress display - batch failed
			if bwo.scanner.progressDisplay != nil {
				bwo.scanner.progressDisplay.UpdateScanProgress(
					int64(batchNumber),
					int64(batchCount),
					"Batch Failed",
					fmt.Sprintf("Batch %d/%d failed: %v", batchNumber, batchCount, err),
				)
				bwo.scanner.progressDisplay.UpdateBatchProgress(common.ProgressTypeScan, batchNumber, batchCount)
			}

			lastBatchError = err
			return err
		}

		// Aggregate results
		bwo.aggregateBatchResults(&aggregatedSummary, batchSummary)
		allReportPaths = append(allReportPaths, batchReportPaths...)
		processedBatches++

		// Update progress display - batch completed
		if bwo.scanner.progressDisplay != nil {
			completedTargets := processedBatches * len(batch) // Approximate
			if processedBatches == batchCount {
				completedTargets = len(targetURLs) // Exact for last batch
			}

			bwo.scanner.progressDisplay.UpdateScanProgress(
				int64(batchNumber),
				int64(batchCount),
				"Batch Completed",
				fmt.Sprintf("Completed batch %d/%d (%d targets processed)", batchNumber, batchCount, completedTargets),
			)
			bwo.scanner.progressDisplay.UpdateBatchProgress(common.ProgressTypeScan, batchNumber, batchCount)
		}

		// Force garbage collection after each batch to free memory
		runtime.GC()
		runtime.ReadMemStats(&memStats)

		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_number", batchNumber).
			Int("batch_targets", len(batch)).
			Int("total_processed", processedBatches).
			Uint64("alloc_mb_after_gc", memStats.Alloc/1024/1024).
			Msg("Batch scan completed successfully with GC")

		return nil
	}

	batchResults, err := bwo.batchProcessor.ProcessBatches(ctx, targetURLs, processFunc)

	// Check if processing was interrupted
	if err != nil || processedBatches < batchCount {
		interruptedAt = processedBatches + 1
		bwo.logger.Warn().
			Err(err).
			Int("processed_batches", processedBatches).
			Int("total_batches", batchCount).
			Int("interrupted_at", interruptedAt).
			Msg("Batch processing was interrupted or failed")

		// Update progress display - interrupted
		if bwo.scanner.progressDisplay != nil {
			bwo.scanner.progressDisplay.UpdateScanProgress(
				int64(processedBatches),
				int64(batchCount),
				"Interrupted",
				fmt.Sprintf("Batch processing interrupted at %d/%d", processedBatches, batchCount),
			)
		}
	} else {
		// Update progress display - all batches completed
		if bwo.scanner.progressDisplay != nil {
			bwo.scanner.progressDisplay.UpdateScanProgress(
				int64(batchCount),
				int64(batchCount),
				"Batch Complete",
				fmt.Sprintf("All %d batches completed successfully (%d targets)", batchCount, len(targetURLs)),
			)
		}
	}

	// Finalize aggregated summary
	bwo.finalizeBatchSummary(&aggregatedSummary, processedBatches, batchCount, lastBatchError, interruptedAt > 0)

	result := &BatchScanResult{
		SummaryData:      aggregatedSummary,
		ReportFilePaths:  allReportPaths,
		BatchResults:     batchResults,
		TotalBatches:     batchCount,
		ProcessedBatches: processedBatches,
		UsedBatching:     true,
		InterruptedAt:    interruptedAt,
	}

	bwo.logger.Info().
		Int("total_batches", batchCount).
		Int("processed_batches", processedBatches).
		Bool("interrupted", interruptedAt > 0).
		Int("total_reports", len(allReportPaths)).
		Msg("Batch scan execution completed")

	// Log comprehensive summary
	bwo.logBatchProcessingSummary(result)

	return result, err
}

// aggregateBatchResults aggregates results from individual batches
func (bwo *BatchWorkflowOrchestrator) aggregateBatchResults(
	aggregated *models.ScanSummaryData,
	batchSummary models.ScanSummaryData,
) {
	// Aggregate probe stats
	aggregated.ProbeStats.TotalProbed += batchSummary.ProbeStats.TotalProbed
	aggregated.ProbeStats.SuccessfulProbes += batchSummary.ProbeStats.SuccessfulProbes
	aggregated.ProbeStats.FailedProbes += batchSummary.ProbeStats.FailedProbes
	aggregated.ProbeStats.DiscoverableItems += batchSummary.ProbeStats.DiscoverableItems

	// Aggregate diff stats
	aggregated.DiffStats.New += batchSummary.DiffStats.New
	aggregated.DiffStats.Old += batchSummary.DiffStats.Old
	aggregated.DiffStats.Existing += batchSummary.DiffStats.Existing
	aggregated.DiffStats.Changed += batchSummary.DiffStats.Changed

	// Aggregate scan duration
	aggregated.ScanDuration += batchSummary.ScanDuration

	// Collect error messages
	aggregated.ErrorMessages = append(aggregated.ErrorMessages, batchSummary.ErrorMessages...)

	// Add retries attempted
	aggregated.RetriesAttempted += batchSummary.RetriesAttempted
}

// finalizeBatchSummary finalizes the aggregated summary
func (bwo *BatchWorkflowOrchestrator) finalizeBatchSummary(
	summary *models.ScanSummaryData,
	processedBatches int,
	totalBatches int,
	lastError error,
	wasInterrupted bool,
) {
	// Determine final status
	if wasInterrupted {
		if processedBatches == 0 {
			summary.Status = string(models.ScanStatusFailed)
		} else {
			summary.Status = string(models.ScanStatusPartialComplete)
		}
	} else {
		summary.Status = string(models.ScanStatusCompleted)
	}

	// Add batch processing information to error messages for reporting
	if totalBatches > 1 {
		batchInfo := fmt.Sprintf("Batch processing: %d/%d batches completed", processedBatches, totalBatches)
		summary.ErrorMessages = append(summary.ErrorMessages, batchInfo)
	}

	if lastError != nil && !wasInterrupted {
		summary.Status = string(models.ScanStatusFailed)
	}
}

// GetBatchingInfo returns batching information for the given targets file
func (bwo *BatchWorkflowOrchestrator) GetBatchingInfo(scanTargetsFile string) (useBatching bool, batchCount int, err error) {
	targets, _, err := bwo.targetManager.LoadAndSelectTargets(scanTargetsFile)
	if err != nil {
		return false, 0, err
	}

	useBatching = bwo.batchProcessor.ShouldUseBatching(len(targets))
	if useBatching {
		batchCount, _ = bwo.batchProcessor.GetBatchingStats(len(targets))
	} else {
		batchCount = 1
	}

	return useBatching, batchCount, nil
}

// optimizeConfigForMemoryEfficiency tự động điều chỉnh config để tiết kiệm memory
func (bwo *BatchWorkflowOrchestrator) optimizeConfigForMemoryEfficiency(gCfg *config.GlobalConfig, batchSize int) {
	// Giảm concurrent requests cho crawler để tránh memory spike
	if gCfg.CrawlerConfig.MaxConcurrentRequests > 10 {
		originalConcurrent := gCfg.CrawlerConfig.MaxConcurrentRequests
		gCfg.CrawlerConfig.MaxConcurrentRequests = 10
		bwo.logger.Info().
			Int("original_concurrent", originalConcurrent).
			Int("optimized_concurrent", 10).
			Msg("Reduced crawler concurrent requests for memory efficiency")
	}

	// Giảm httpx threads nếu cần
	if gCfg.HttpxRunnerConfig.Threads > 30 {
		originalThreads := gCfg.HttpxRunnerConfig.Threads
		gCfg.HttpxRunnerConfig.Threads = 30
		bwo.logger.Info().
			Int("original_threads", originalThreads).
			Int("optimized_threads", 30).
			Msg("Reduced httpx threads for memory efficiency")
	}

	// Log optimization info
	bwo.logger.Info().
		Int("batch_size", batchSize).
		Int("crawler_concurrent", gCfg.CrawlerConfig.MaxConcurrentRequests).
		Int("httpx_threads", gCfg.HttpxRunnerConfig.Threads).
		Msg("Configuration optimized for memory efficiency during batch processing")
}

// logBatchProcessingSummary logs comprehensive summary of batch processing
func (bwo *BatchWorkflowOrchestrator) logBatchProcessingSummary(result *BatchScanResult) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	bwo.logger.Info().
		Int("total_batches", result.TotalBatches).
		Int("processed_batches", result.ProcessedBatches).
		Int("total_targets", result.SummaryData.TotalTargets).
		Int("successful_probes", result.SummaryData.ProbeStats.SuccessfulProbes).
		Int("failed_probes", result.SummaryData.ProbeStats.FailedProbes).
		Int("new_urls", result.SummaryData.DiffStats.New).
		Int("existing_urls", result.SummaryData.DiffStats.Existing).
		Int("old_urls", result.SummaryData.DiffStats.Old).
		Dur("total_duration", result.SummaryData.ScanDuration).
		Int("report_files", len(result.ReportFilePaths)).
		Bool("was_interrupted", result.InterruptedAt > 0).
		Uint64("final_memory_mb", memStats.Alloc/1024/1024).
		Str("status", result.SummaryData.Status).
		Msg("Batch processing summary completed")

	if result.InterruptedAt > 0 {
		bwo.logger.Warn().
			Int("interrupted_at_batch", result.InterruptedAt).
			Int("completed_batches", result.ProcessedBatches).
			Float64("completion_percentage", float64(result.ProcessedBatches)/float64(result.TotalBatches)*100).
			Msg("Batch processing was interrupted")
	}
}
