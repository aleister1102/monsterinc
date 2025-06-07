package scanner

import (
	"context"
	"fmt"
	"time"

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
	bpConfig := common.BatchProcessorConfig{
		BatchSize:          gCfg.BatchProcessorConfig.BatchSize,
		MaxConcurrentBatch: gCfg.BatchProcessorConfig.MaxConcurrentBatch,
		BatchTimeout:       time.Duration(gCfg.BatchProcessorConfig.BatchTimeoutMins) * time.Minute,
		ThresholdSize:      gCfg.BatchProcessorConfig.ThresholdSize,
	}

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

	// Load targets from file
	targets, determinedSource, err := bwo.targetManager.LoadAndSelectTargets(scanTargetsFile)
	if err != nil {
		return nil, common.WrapError(err, "failed to load scan targets")
	}

	if len(targets) == 0 {
		return nil, common.NewError("no valid targets found in source: %s", determinedSource)
	}

	targetURLs := bwo.targetManager.GetTargetStrings(targets)

	// Check if batching is needed
	useBatching := bwo.batchProcessor.ShouldUseBatching(len(targetURLs))

	if !useBatching {
		bwo.logger.Info().
			Int("target_count", len(targetURLs)).
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
		Msg("Starting batched scan execution\n\n\n\n\n\n=============================")

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
		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_size", len(batch)).
			Int("progress", batchIndex+1).
			Int("total", batchCount).
			Msg("Processing scan batch")

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
				Msg("Batch scan failed")
			lastBatchError = err
			return err
		}

		// Aggregate results
		bwo.aggregateBatchResults(&aggregatedSummary, batchSummary)
		allReportPaths = append(allReportPaths, batchReportPaths...)
		processedBatches++

		bwo.logger.Info().
			Int("batch_index", batchIndex).
			Int("batch_targets", len(batch)).
			Int("total_processed", processedBatches).
			Msg("Batch scan completed successfully")

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
