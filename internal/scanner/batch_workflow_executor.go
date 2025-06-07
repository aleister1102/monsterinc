package scanner

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
)

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
