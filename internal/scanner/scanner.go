package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"

	"github.com/rs/zerolog"
)

// Scanner handles the core logic of scanning operations
// Focuses on coordinating crawler, httpx probing, and diff/storage operations
type Scanner struct {
	config             *config.GlobalConfig
	logger             zerolog.Logger
	parquetReader      *datastore.ParquetReader
	parquetWriter      *datastore.ParquetWriter
	pathExtractor      *extractor.PathExtractor
	configBuilder      *ConfigBuilder
	crawlerExecutor    *CrawlerExecutor
	httpxExecutor      *HTTPXExecutor
	diffProcessor      *DiffStorageProcessor
	progressDisplay    *common.ProgressDisplayManager
	urlPreprocessor    *URLPreprocessor
	resourceLimiter    *common.ResourceLimiter
	notificationHelper interface {
		SendScanStartNotification(ctx context.Context, summary models.ScanSummaryData)
		SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData, serviceType notifier.NotificationServiceType, reportFilePaths []string)
		SendScanInterruptNotification(ctx context.Context, summary models.ScanSummaryData)
	}
}

// NewScanner creates a new Scanner instance with required dependencies
func NewScanner(
	globalConfig *config.GlobalConfig,
	logger zerolog.Logger,
	pReader *datastore.ParquetReader,
	pWriter *datastore.ParquetWriter,
) *Scanner {
	scanner := &Scanner{
		config:        globalConfig,
		logger:        logger.With().Str("module", "Scanner").Logger(),
		parquetReader: pReader,
		parquetWriter: pWriter,
		configBuilder: NewConfigBuilder(globalConfig, logger),
	}

	// Initialize executors
	scanner.crawlerExecutor = NewCrawlerExecutor(logger)
	scanner.httpxExecutor = NewHTTPXExecutor(logger)

	// Initialize path extractor with error handling
	if pathExtractor, err := extractor.NewPathExtractor(globalConfig.ExtractorConfig, logger); err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize path extractor")
	} else {
		scanner.pathExtractor = pathExtractor
	}

	// Initialize diff processor with URL differ
	if urlDiffer, err := differ.NewUrlDiffer(pReader, logger); err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize URL differ")
	} else {
		scanner.diffProcessor = NewDiffStorageProcessor(logger, pWriter, urlDiffer)
	}

	// Initialize URL preprocessor
	preprocessorConfig := URLPreprocessorConfig{
		URLNormalization: globalConfig.CrawlerConfig.URLNormalization,
		AutoCalibrate:    globalConfig.CrawlerConfig.AutoCalibrate,
		EnableBatching:   true,
		BatchSize:        1000,
		MaxWorkers:       globalConfig.CrawlerConfig.MaxConcurrentRequests, // Same as crawler threads
		EnableParallel:   true,
	}
	scanner.urlPreprocessor = NewURLPreprocessor(preprocessorConfig, logger)

	// Initialize resource limiter
	resourceLimiterConfig := common.ResourceLimiterConfig{
		MaxMemoryMB:        globalConfig.ResourceLimiterConfig.MaxMemoryMB,
		MaxGoroutines:      globalConfig.ResourceLimiterConfig.MaxGoroutines,
		CheckInterval:      time.Duration(globalConfig.ResourceLimiterConfig.CheckIntervalSecs) * time.Second,
		MemoryThreshold:    globalConfig.ResourceLimiterConfig.MemoryThreshold,
		GoroutineWarning:   globalConfig.ResourceLimiterConfig.GoroutineWarning,
		SystemMemThreshold: globalConfig.ResourceLimiterConfig.SystemMemThreshold,
		CPUThreshold:       globalConfig.ResourceLimiterConfig.CPUThreshold,
		EnableAutoShutdown: false, // Disable auto-shutdown for scanner, main handles this
	}
	scanner.resourceLimiter = common.NewResourceLimiter(resourceLimiterConfig, logger)
	scanner.resourceLimiter.Start()

	logger.Info().
		Int("crawler_threads", globalConfig.CrawlerConfig.MaxConcurrentRequests).
		Int("preprocessor_workers", preprocessorConfig.MaxWorkers).
		Bool("parallel_enabled", preprocessorConfig.EnableParallel).
		Msg("URL preprocessor configured with parallel processing based on crawler threads")

	return scanner
}

// SetNotificationHelper sets the notification helper for the scanner
func (s *Scanner) SetNotificationHelper(notificationHelper interface {
	SendScanStartNotification(ctx context.Context, summary models.ScanSummaryData)
	SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData, serviceType notifier.NotificationServiceType, reportFilePaths []string)
	SendScanInterruptNotification(ctx context.Context, summary models.ScanSummaryData)
}) {
	s.notificationHelper = notificationHelper
}

// SetProgressDisplay đặt progress display manager
func (s *Scanner) SetProgressDisplay(progressDisplay *common.ProgressDisplayManager) {
	s.progressDisplay = progressDisplay

	// Pass progress display to executors
	if s.httpxExecutor != nil {
		s.httpxExecutor.SetProgressDisplay(progressDisplay)
	}
	if s.crawlerExecutor != nil {
		s.crawlerExecutor.SetProgressDisplay(progressDisplay)
	}
}

// ExecuteSingleScanWorkflowWithReporting performs complete scan workflow with reporting
func (s *Scanner) ExecuteSingleScanWorkflowWithReporting(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	appLogger zerolog.Logger,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
	scanMode string,
) (models.ScanSummaryData, []models.ProbeResult, []string, error) {
	startTime := time.Now()
	probeResults, urlDiffResults, err := s.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
	if err != nil {
		return models.ScanSummaryData{}, nil, nil, err
	}

	// Generate HTML reports if we have results
	var reportFilePaths []string
	if len(probeResults) > 0 {
		reportGenerator := NewReportGenerator(&gCfg.ReporterConfig, s.logger)
		reportInput := NewReportGenerationInputWithDiff(probeResults, urlDiffResults, scanSessionID)
		reportPaths, reportErr := reportGenerator.GenerateReports(reportInput)
		if reportErr != nil {
			s.logger.Warn().Err(reportErr).Msg("Failed to generate reports")
		} else {
			reportFilePaths = reportPaths
		}
	}

	// Note: We use original seedURLs in summary for reporting purposes,
	// but the actual processing used the preprocessed URLs
	summaryBuilder := NewSummaryBuilder(s.logger)
	summaryInput := &SummaryInput{
		ScanSessionID:   scanSessionID,
		TargetSource:    targetSource,
		ScanMode:        scanMode,
		Targets:         seedURLs,
		StartTime:       startTime,
		ProbeResults:    probeResults,
		URLDiffResults:  urlDiffResults,
		ReportFilePaths: reportFilePaths,
	}
	summary := summaryBuilder.BuildSummary(summaryInput)

	return summary, probeResults, reportFilePaths, nil
}

// ExecuteCompleteScanWorkflow performs the complete scan workflow including diffing
func (s *Scanner) ExecuteCompleteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, error) {
	probeResults, urlDiffResults, err := s.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
	if err != nil {
		return models.ScanSummaryData{}, nil, nil, err
	}

	// Build summary using new SummaryBuilder
	summaryBuilder := NewSummaryBuilder(s.logger)
	summaryInput := &SummaryInput{
		ScanSessionID:  scanSessionID,
		TargetSource:   targetSource,
		Targets:        seedURLs,
		ProbeResults:   probeResults,
		URLDiffResults: urlDiffResults,
	}
	summary := summaryBuilder.BuildSummary(summaryInput)

	return summary, probeResults, urlDiffResults, nil
}

// ExecuteScanWorkflow performs the core scan workflow: crawling, probing, and diffing
func (s *Scanner) ExecuteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {
	startTime := time.Now()

	// Log resource usage before scan workflow
	if s.resourceLimiter != nil {
		resourceUsageBefore := s.resourceLimiter.GetResourceUsage()
		s.logger.Info().
			Int64("memory_mb", resourceUsageBefore.AllocMB).
			Int("goroutines", resourceUsageBefore.Goroutines).
			Float64("system_mem_percent", resourceUsageBefore.SystemMemUsedPercent).
			Float64("cpu_percent", resourceUsageBefore.CPUUsagePercent).
			Str("session_id", scanSessionID).
			Msg("Resource usage before scan workflow")
	}

	// Note: Scan start notification is sent from the main entry point (main.go or scheduler)
	// to avoid duplicate notifications when this workflow is called from different contexts

	// Update progress: Starting URL preprocessing
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(0, 5, "Preprocessing", "Normalizing and filtering URLs")
	}

	// Step 0: Preprocess URLs (normalize and auto-calibrate)
	preprocessResult := s.urlPreprocessor.PreprocessURLs(seedURLs)
	processedSeedURLs := preprocessResult.ProcessedURLs

	s.logger.Info().
		Int("original_urls", len(seedURLs)).
		Int("processed_urls", len(processedSeedURLs)).
		Int("normalized", preprocessResult.Stats.Normalized).
		Int("skipped_by_pattern", preprocessResult.Stats.SkippedByPattern).
		Int("skipped_duplicate", preprocessResult.Stats.SkippedDuplicate).
		Msg("URL preprocessing completed")

	// Use processed URLs for the rest of the workflow
	if len(processedSeedURLs) == 0 {
		if s.progressDisplay != nil {
			s.progressDisplay.SetScanStatus(common.ProgressStatusError, "No URLs remaining after preprocessing")
		}

		// Send error notification
		if s.notificationHelper != nil {
			errorSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusFailed),
				ScanDuration:  time.Since(startTime),
				ErrorMessages: []string{"No URLs remaining after preprocessing"},
			}
			s.notificationHelper.SendScanCompletionNotification(ctx, errorSummary, notifier.ScanServiceNotification, nil)
		}

		return nil, nil, fmt.Errorf("no URLs remaining after preprocessing")
	}

	// Update progress: Starting crawler configuration
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(1, 5, "Crawler", "Configuring crawler")
	}

	// Step 1: Configure and execute crawler
	crawlerConfig, primaryRootTargetURL, err := s.configBuilder.BuildCrawlerConfig(processedSeedURLs, scanSessionID)
	if err != nil {
		if s.progressDisplay != nil {
			s.progressDisplay.SetScanStatus(common.ProgressStatusError, "Failed to build crawler config")
		}

		// Send error notification
		if s.notificationHelper != nil {
			errorSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusFailed),
				ScanDuration:  time.Since(startTime),
				ErrorMessages: []string{fmt.Sprintf("Failed to build crawler config: %v", err)},
			}
			s.notificationHelper.SendScanCompletionNotification(ctx, errorSummary, notifier.ScanServiceNotification, nil)
		}

		return nil, nil, fmt.Errorf("failed to build crawler config: %w", err)
	}

	crawlerInput := CrawlerExecutionInput{
		Context:              ctx,
		CrawlerConfig:        crawlerConfig,
		ScanSessionID:        scanSessionID,
		PrimaryRootTargetURL: primaryRootTargetURL,
	}

	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(1, 5, "Crawler", "Executing crawler")
	}

	// Check for context cancellation before crawler execution
	if ctx.Err() != nil {
		if s.notificationHelper != nil {
			interruptSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusInterrupted),
				ScanDuration:  time.Since(startTime),
				Component:     "crawler",
			}
			s.notificationHelper.SendScanInterruptNotification(ctx, interruptSummary)
		}
		return nil, nil, ctx.Err()
	}

	crawlerResult := s.crawlerExecutor.Execute(crawlerInput)
	if crawlerResult.Error != nil {
		if s.progressDisplay != nil {
			s.progressDisplay.SetScanStatus(common.ProgressStatusError, "Crawler execution failed")
		}

		// Send error notification
		if s.notificationHelper != nil {
			errorSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusFailed),
				ScanDuration:  time.Since(startTime),
				ErrorMessages: []string{fmt.Sprintf("Crawler execution failed: %v", crawlerResult.Error)},
			}
			s.notificationHelper.SendScanCompletionNotification(ctx, errorSummary, notifier.ScanServiceNotification, nil)
		}

		return nil, nil, fmt.Errorf("crawler execution failed: %w", crawlerResult.Error)
	}

	// Set crawler instance for HTTPX executor to use for root target tracking
	s.httpxExecutor.SetCrawlerInstance(crawlerResult.CrawlerInstance)

	// Step 2: Execute HTTPX probing
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(2, 5, "Probing", "Configuring HTTPX probing")
	}

	httpxConfig := s.configBuilder.BuildHTTPXConfig(crawlerResult.DiscoveredURLs)
	httpxInput := HTTPXExecutionInput{
		Context:              ctx,
		DiscoveredURLs:       crawlerResult.DiscoveredURLs,
		SeedURLs:             processedSeedURLs,
		PrimaryRootTargetURL: primaryRootTargetURL,
		ScanSessionID:        scanSessionID,
		HttpxRunnerConfig:    httpxConfig,
	}

	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(2, 5, "Probing", fmt.Sprintf("Probing %d URLs", len(crawlerResult.DiscoveredURLs)))
	}

	// Check for context cancellation before HTTPX execution
	if ctx.Err() != nil {
		if s.notificationHelper != nil {
			interruptSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusInterrupted),
				ScanDuration:  time.Since(startTime),
				Component:     "httpx",
			}
			s.notificationHelper.SendScanInterruptNotification(ctx, interruptSummary)
		}
		return nil, nil, ctx.Err()
	}

	httpxResult := s.httpxExecutor.Execute(httpxInput)
	if httpxResult.Error != nil {
		if s.progressDisplay != nil {
			s.progressDisplay.SetScanStatus(common.ProgressStatusError, "HTTPX execution failed")
		}

		// Send error notification
		if s.notificationHelper != nil {
			errorSummary := models.ScanSummaryData{
				ScanSessionID: scanSessionID,
				ScanMode:      "scan",
				TargetSource:  "scanner_workflow",
				Targets:       seedURLs,
				TotalTargets:  len(seedURLs),
				Status:        string(models.ScanStatusFailed),
				ScanDuration:  time.Since(startTime),
				ErrorMessages: []string{fmt.Sprintf("HTTPX execution failed: %v", httpxResult.Error)},
			}
			s.notificationHelper.SendScanCompletionNotification(ctx, errorSummary, notifier.ScanServiceNotification, nil)
		}

		return nil, nil, fmt.Errorf("HTTPX execution failed: %w", httpxResult.Error)
	}

	// Step 3: Process diffing and storage
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(3, 5, "Diffing", "Processing diffs and storage")
	}

	var urlDiffResults map[string]models.URLDiffResult
	if s.diffProcessor != nil {
		diffInput := ProcessDiffingAndStorageInput{
			Ctx:                     ctx,
			CurrentScanProbeResults: httpxResult.ProbeResults,
			SeedURLs:                processedSeedURLs,
			PrimaryRootTargetURL:    primaryRootTargetURL,
			ScanSessionID:           scanSessionID,
		}

		diffOutput, err := s.diffProcessor.ProcessDiffingAndStorage(diffInput)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Diffing and storage failed, continuing with results")
		} else {
			urlDiffResults = diffOutput.URLDiffResults
			httpxResult.ProbeResults = diffOutput.UpdatedScanProbeResults
		}
	}

	// Step 4: Workflow completed
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateScanProgress(4, 5, "Complete", "Scan workflow completed")
		s.progressDisplay.SetScanStatus(common.ProgressStatusComplete, fmt.Sprintf("Found %d probe results\n", len(httpxResult.ProbeResults)))
	}

	// Log resource usage after scan workflow
	if s.resourceLimiter != nil {
		resourceUsageAfter := s.resourceLimiter.GetResourceUsage()
		s.logger.Info().
			Int64("memory_mb", resourceUsageAfter.AllocMB).
			Int("goroutines", resourceUsageAfter.Goroutines).
			Float64("system_mem_percent", resourceUsageAfter.SystemMemUsedPercent).
			Float64("cpu_percent", resourceUsageAfter.CPUUsagePercent).
			Dur("scan_duration", time.Since(startTime)).
			Int("probe_results", len(httpxResult.ProbeResults)).
			Int("url_diffs", len(urlDiffResults)).
			Str("session_id", scanSessionID).
			Msg("Resource usage after scan workflow")
	}

	// NOTE: Notification is sent from the caller level (main.go or scheduler)
	// to avoid duplicate notifications and to include report file paths

	return httpxResult.ProbeResults, urlDiffResults, nil
}

// Shutdown gracefully shuts down the scanner and its components
func (s *Scanner) Shutdown() {
	s.logger.Info().Msg("Shutting down scanner")

	// Log final resource usage before shutdown
	if s.resourceLimiter != nil {
		finalUsage := s.resourceLimiter.GetResourceUsage()
		s.logger.Info().
			Int64("final_memory_mb", finalUsage.AllocMB).
			Int("final_goroutines", finalUsage.Goroutines).
			Float64("final_system_mem_percent", finalUsage.SystemMemUsedPercent).
			Float64("final_cpu_percent", finalUsage.CPUUsagePercent).
			Msg("Final resource usage at scanner shutdown")
	}

	// Shutdown crawler executor (which will shutdown the managed crawler)
	if s.crawlerExecutor != nil {
		s.crawlerExecutor.Shutdown()
	}

	// Shutdown HTTPX executor (which will shutdown the managed httpx runner)
	if s.httpxExecutor != nil {
		s.httpxExecutor.Shutdown()
	}

	// Stop resource limiter
	if s.resourceLimiter != nil {
		s.resourceLimiter.Stop()
	}

	s.logger.Info().Msg("Scanner shutdown complete")
}
