package scanner

import (
	"context"
	"fmt"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// Scanner handles the core logic of scanning operations
// Focuses on coordinating crawler, httpx probing, and diff/storage operations
type Scanner struct {
	config          *config.GlobalConfig
	logger          zerolog.Logger
	parquetReader   *datastore.ParquetReader
	parquetWriter   *datastore.ParquetWriter
	pathExtractor   *extractor.PathExtractor
	configBuilder   *ConfigBuilder
	crawlerExecutor *CrawlerExecutor
	httpxExecutor   *HTTPXExecutor
	diffProcessor   *DiffStorageProcessor
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

	return scanner
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

	// Build summary
	summaryBuilder := NewSummaryBuilder(s.logger)
	summaryInput := &SummaryInput{
		ScanSessionID:   scanSessionID,
		TargetSource:    targetSource,
		ScanMode:        scanMode,
		Targets:         seedURLs,
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
	// Step 1: Configure and execute crawler
	crawlerConfig, primaryRootTargetURL, err := s.configBuilder.BuildCrawlerConfig(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build crawler config: %w", err)
	}

	crawlerInput := CrawlerExecutionInput{
		Context:              ctx,
		CrawlerConfig:        crawlerConfig,
		ScanSessionID:        scanSessionID,
		PrimaryRootTargetURL: primaryRootTargetURL,
	}

	crawlerResult := s.crawlerExecutor.Execute(crawlerInput)
	if crawlerResult.Error != nil {
		return nil, nil, fmt.Errorf("crawler execution failed: %w", crawlerResult.Error)
	}

	// Set crawler instance for HTTPX executor to use for root target tracking
	s.httpxExecutor.SetCrawlerInstance(crawlerResult.CrawlerInstance)

	// Step 2: Execute HTTPX probing
	httpxConfig := s.configBuilder.BuildHTTPXConfig(crawlerResult.DiscoveredURLs)
	httpxInput := HTTPXExecutionInput{
		Context:              ctx,
		DiscoveredURLs:       crawlerResult.DiscoveredURLs,
		SeedURLs:             seedURLs,
		PrimaryRootTargetURL: primaryRootTargetURL,
		ScanSessionID:        scanSessionID,
		HttpxRunnerConfig:    httpxConfig,
	}

	httpxResult := s.httpxExecutor.Execute(httpxInput)
	if httpxResult.Error != nil {
		return nil, nil, fmt.Errorf("HTTPX execution failed: %w", httpxResult.Error)
	}

	// Step 3: Process diffing and storage
	var urlDiffResults map[string]models.URLDiffResult
	if s.diffProcessor != nil {
		diffInput := ProcessDiffingAndStorageInput{
			Ctx:                     ctx,
			CurrentScanProbeResults: httpxResult.ProbeResults,
			SeedURLs:                seedURLs,
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

	return httpxResult.ProbeResults, urlDiffResults, nil
}
