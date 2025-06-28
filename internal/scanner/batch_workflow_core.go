package scanner

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/common/batchprocessor"
	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// BatchWorkflowOrchestrator handles batch processing for scan operations
type BatchWorkflowOrchestrator struct {
	logger          zerolog.Logger
	batchProcessor  *batchprocessor.BatchProcessor
	scanner         *Scanner
	targetManager   *urlhandler.TargetManager
	probeResults    []httpxrunner.ProbeResult
	urlDiffResults  map[string]differ.URLDiffResult
	reportFilePaths []string
	err             error
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

	// Update scanner logger to use the provided logger for this scan session
	scanner.UpdateLogger(logger)

	return &BatchWorkflowOrchestrator{
		logger:         orchestratorLogger,
		batchProcessor: batchprocessor.NewBatchProcessor(bpConfig, logger),
		scanner:        scanner,
		targetManager:  urlhandler.NewTargetManager(logger),
	}
}

// BatchScanResult holds the result of batch scan processing
type BatchScanResult struct {
	SummaryData      summary.ScanSummaryData
	ReportFilePaths  []string
	BatchResults     []batchprocessor.BatchResult
	TotalBatches     int
	ProcessedBatches int
	UsedBatching     bool
	InterruptedAt    int // Which batch was interrupted (0 means completed)
}

// BatchWorkflowResult holds the result of batch scan processing
type BatchWorkflowResult struct {
	ProbeResults    []httpxrunner.ProbeResult
	URLDiffResults  map[string]differ.URLDiffResult
	ReportFilePaths []string
	Err             error
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
		return nil, errorwrapper.NewError("global config cannot be nil")
	}
	if scanTargetsFile == "" {
		return nil, errorwrapper.NewError("scan targets file cannot be empty")
	}
	if scanSessionID == "" {
		return nil, errorwrapper.NewError("scan session ID cannot be empty")
	}

	// Load targets from file
	targets, determinedSource, err := bwo.targetManager.LoadAndSelectTargets(scanTargetsFile)
	if err != nil {
		return nil, errorwrapper.WrapError(err, "failed to load scan targets")
	}

	if len(targets) == 0 {
		return nil, errorwrapper.NewError("no valid targets found in source: %s", determinedSource)
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
			BatchResults:     []batchprocessor.BatchResult{},
			TotalBatches:     1,
			ProcessedBatches: 1,
			UsedBatching:     false,
			InterruptedAt:    0,
		}, err
	}

	return bwo.executeBatchedScan(ctx, gCfg, targetURLs, scanSessionID, targetSource, scanMode)
}

func (bwo *BatchWorkflowOrchestrator) executeScan(ctx context.Context, seedURLs []string, scanSessionID string) {
	bwo.logger.Info().Strs("seed_urls", seedURLs).Msg("Executing scan workflow for batch")
	probeResults, urlDiffResults, err := bwo.scanner.ExecuteScanWorkflow(
		ctx,
		seedURLs,
		scanSessionID,
	)
	if err != nil {
		bwo.err = err
	}
	bwo.probeResults = probeResults
	bwo.urlDiffResults = urlDiffResults
}

func (bwo *BatchWorkflowOrchestrator) getResult() *BatchWorkflowResult {
	return &BatchWorkflowResult{
		ProbeResults:    bwo.probeResults,
		URLDiffResults:  bwo.urlDiffResults,
		ReportFilePaths: bwo.reportFilePaths,
		Err:             bwo.err,
	}
}
