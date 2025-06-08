package scanner

import (
	"context"

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
	// Set MaxConcurrentBatch based on crawler threads if not already set
	scanBatchConfig := gCfg.ScanBatchConfig
	scanBatchConfig.SetMaxConcurrentFromCrawlerThreads(gCfg.CrawlerConfig.MaxConcurrentRequests)

	bpConfig := scanBatchConfig.ToBatchProcessorConfig()

	orchestratorLogger := logger.With().Str("component", "BatchWorkflowOrchestrator").Logger()
	orchestratorLogger.Info().
		Int("crawler_threads", gCfg.CrawlerConfig.MaxConcurrentRequests).
		Int("max_concurrent_batch", scanBatchConfig.GetEffectiveMaxConcurrentBatch()).
		Int("batch_size", scanBatchConfig.BatchSize).
		Msg("Scan batch configuration initialized based on crawler threads")

	return &BatchWorkflowOrchestrator{
		logger:         orchestratorLogger,
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
