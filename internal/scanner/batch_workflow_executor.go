package scanner

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	disableNotificationsKey contextKey = "disable_individual_notifications"
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

	// Optimize configuration for memory efficiency during batch processing
	batchSize, _ := bwo.batchProcessor.GetBatchingStats(len(targetURLs))
	bwo.optimizeConfigForMemoryEfficiency(gCfg, batchSize)

	var allReportPaths []string
	var aggregatedSummary summary.ScanSummaryData
	var lastBatchError error
	processedBatches := 0
	interruptedAt := 0

	// Always merge batch results to avoid separate reports per batch
	// But still respect max_probe_results_per_report_file for Discord file size limits
	var allProbeResults []httpxrunner.ProbeResult
	allURLDiffResults := make(map[string]differ.URLDiffResult)
	bwo.logger.Info().Msg("Aggregating all batch results into merged reports (respecting Discord file size limits)")

	// Initialize summary data
	aggregatedSummary = summary.GetDefaultScanSummaryData()
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

		// Create batch-specific session ID
		batchSessionID := fmt.Sprintf("%s-batch-%d", scanSessionID, batchIndex)

		var batchSummary summary.ScanSummaryData
		var err error

		// Always execute core workflow without generating reports per batch
		ctx = context.WithValue(ctx, disableNotificationsKey, true)
		batchProbeResults, batchURLDiffResults, err := bwo.scanner.ExecuteScanWorkflow(
			ctx,
			batch,
			batchSessionID,
		)

		if err != nil {
			bwo.logger.Error().
				Err(err).
				Int("batch_index", batchIndex).
				Int("batch_number", batchNumber).
				Msg("Batch scan workflow failed")

			lastBatchError = err
			return err
		}

		// Aggregate probe results and URL diff results
		allProbeResults = append(allProbeResults, batchProbeResults...)
		for url, diffResult := range batchURLDiffResults {
			allURLDiffResults[url] = diffResult
		}

		// Create summary for this batch (without reports)
		summaryBuilder := summary.NewSummaryBuilder(bwo.logger)
		summaryInput := &summary.SummaryInput{
			ScanSessionID:  batchSessionID,
			TargetSource:   targetSource,
			ScanMode:       scanMode,
			Targets:        batch,
			ScanDuration:   0, // Let BuildSummary calculate from StartTime
			ProbeResults:   batchProbeResults,
			URLDiffResults: batchURLDiffResults,
		}
		batchSummary = summaryBuilder.BuildSummary(summaryInput)

		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_probe_results", len(batchProbeResults)).
			Int("total_accumulated_results", len(allProbeResults)).
			Msg("Batch results accumulated for merged report")

		// Aggregate results
		bwo.aggregateBatchResults(&aggregatedSummary, batchSummary)
		processedBatches++
		// Force garbage collection after each batch to free memory
		runtime.GC()

		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_number", batchNumber).
			Int("batch_targets", len(batch)).
			Int("total_processed", processedBatches).
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
	}

	// Ensure crawler is fully shutdown before generating reports to prevent ongoing requests
	bwo.ensureCrawlerShutdown()

	// Generate merged report from all batch results if we have any
	if len(allProbeResults) > 0 && (err == nil && processedBatches > 0) {
		bwo.logger.Info().
			Int("total_probe_results", len(allProbeResults)).
			Int("total_url_diffs", len(allURLDiffResults)).
			Msg("Generating merged report from all batch results")

		reportGenerator := NewReportGenerator(&gCfg.ReporterConfig, bwo.logger)
		reportInput := NewReportGenerationInputWithDiff(allProbeResults, allURLDiffResults, scanSessionID)
		mergedReportPaths, reportErr := reportGenerator.GenerateReports(ctx, reportInput)

		if reportErr != nil {
			bwo.logger.Warn().Err(reportErr).Msg("Failed to generate merged report")
		} else {
			allReportPaths = mergedReportPaths
			bwo.logger.Info().
				Strs("report_paths", mergedReportPaths).
				Msg("Merged report generated successfully")
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
		Bool("used_merge_reports", true).
		Msg("Batch scan execution completed")

	// Log comprehensive summary
	bwo.logBatchProcessingSummary(result)

	return result, err
}

// ensureCrawlerShutdown ensures the crawler is fully shutdown before continuing
func (bwo *BatchWorkflowOrchestrator) ensureCrawlerShutdown() {
	bwo.logger.Info().Msg("Ensuring crawler is fully shutdown before generating reports")

	// Reset crawler executor to ensure cleanup
	if bwo.scanner != nil {
		bwo.scanner.ResetCrawler()
	}

	bwo.logger.Info().Msg("Crawler shutdown completed, safe to generate reports")
}
