package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// Scanner orchestrates the complete scan workflow including crawling,
// probing, diffing, and storing results. It follows the single responsibility
// principle by coordinating different scan components.
type Scanner struct {
	config        *config.GlobalConfig
	logger        zerolog.Logger
	parquetReader *datastore.ParquetReader
	parquetWriter *datastore.ParquetWriter
	pathExtractor *extractor.PathExtractor
}

// NewScanner creates a new Scanner instance with all required dependencies.
// Returns nil if critical components fail to initialize.
func NewScanner(
	globalConfig *config.GlobalConfig,
	logger zerolog.Logger,
	parquetReader *datastore.ParquetReader,
	parquetWriter *datastore.ParquetWriter,
) *Scanner {
	pathExtractor, err := extractor.NewPathExtractor(globalConfig.ExtractorConfig, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PathExtractor")
		return nil
	}

	return &Scanner{
		config:        globalConfig,
		logger:        logger.With().Str("module", "Scanner").Logger(),
		parquetReader: parquetReader,
		parquetWriter: parquetWriter,
		pathExtractor: pathExtractor,
	}
}

// ExecuteSingleScanWorkflowWithReporting runs a complete scan workflow and generates reports.
// This is the main entry point for executing a full scan cycle.
func (s *Scanner) ExecuteSingleScanWorkflowWithReporting(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	appLogger zerolog.Logger,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
	scanMode string,
) (models.ScanSummaryData, []models.ProbeResult, []string, error) {

	summaryData := s.initializeScanSummary(scanSessionID, targetSource, scanMode, seedURLs)

	if err := s.validateSeedURLs(seedURLs, scanSessionID, targetSource, scanMode); err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{err.Error()}
		return summaryData, nil, nil, err
	}

	workflowResult, err := s.executeScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)
	if err != nil {
		return s.handleWorkflowError(summaryData, workflowResult, appLogger, scanSessionID, err)
	}

	if ctx.Err() != nil {
		return s.handleWorkflowCancellation(summaryData, workflowResult, appLogger, scanSessionID)
	}

	reportPaths, err := s.generateReports(ctx, gCfg, workflowResult.ProbeResults, scanSessionID, scanMode, targetSource, appLogger)
	if err != nil {
		return s.handleReportError(summaryData, workflowResult, appLogger, err)
	}

	s.updateSummaryWithResults(summaryData, workflowResult)
	s.finalizeScanStatus(summaryData)

	return summaryData, workflowResult.ProbeResults, reportPaths, nil
}

// ExecuteCompleteScanWorkflow executes the complete scan workflow and builds summary data.
func (s *Scanner) ExecuteCompleteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, error) {

	startTime := time.Now()

	probeResults, urlDiffResults, workflowError := s.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)

	summaryData := s.buildScanSummary(ScanWorkflowInput{
		ScanSessionID: scanSessionID,
		TargetSource:  targetSource,
		Targets:       seedURLs,
		StartTime:     startTime,
	}, ScanWorkflowResult{
		ProbeResults:   probeResults,
		URLDiffResults: urlDiffResults,
		WorkflowError:  workflowError,
	})

	return summaryData, probeResults, urlDiffResults, workflowError
}

// ExecuteScanWorkflow runs the core scan workflow: crawl -> probe -> diff -> store.
func (s *Scanner) ExecuteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {

	crawlerConfig, primaryRootTargetURL, err := s.prepareScanConfiguration(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, common.WrapError(err, "failed to prepare scan configuration")
	}

	discoveredURLs, err := s.executeCrawler(ctx, crawlerConfig, scanSessionID, primaryRootTargetURL)
	if err != nil {
		return nil, nil, common.WrapError(err, "crawler execution failed")
	}

	httpxConfig := s.buildHTTPXConfig(discoveredURLs)
	httpxInput := HTTPXProbingInput{
		DiscoveredURLs:       discoveredURLs,
		SeedURLs:             seedURLs,
		PrimaryRootTargetURL: primaryRootTargetURL,
		ScanSessionID:        scanSessionID,
		HttpxRunnerConfig:    httpxConfig,
	}

	probeResults, err := s.executeHTTPXProbing(ctx, httpxInput)
	if err != nil {
		return nil, nil, common.WrapError(err, "HTTPX probing failed")
	}

	diffStoreInput := ProcessDiffingAndStorageInput{
		Ctx:                     ctx,
		CurrentScanProbeResults: probeResults,
		SeedURLs:                seedURLs,
		PrimaryRootTargetURL:    primaryRootTargetURL,
		ScanSessionID:           scanSessionID,
	}

	diffStoreOutput, err := s.processDiffingAndStorage(diffStoreInput)
	if err != nil {
		return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults,
			common.WrapError(err, "diffing and storage failed")
	}

	s.logger.Info().Str("session_id", scanSessionID).Msg("Scan workflow completed successfully")
	return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults, nil
}

// initializeScanSummary creates and initializes a scan summary with basic information.
func (s *Scanner) initializeScanSummary(scanSessionID, targetSource, scanMode string, seedURLs []string) models.ScanSummaryData {
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = scanSessionID
	summaryData.TargetSource = targetSource
	summaryData.ScanMode = scanMode
	summaryData.Targets = seedURLs
	summaryData.TotalTargets = len(seedURLs)
	return summaryData
}

// validateSeedURLs ensures that seed URLs are provided for the scan.
func (s *Scanner) validateSeedURLs(seedURLs []string, scanSessionID, targetSource, scanMode string) error {
	if len(seedURLs) == 0 {
		msg := "No seed URLs provided for scan workflow"
		s.logger.Error().Msg(msg)
		return common.NewError(msg)
	}
	return nil
}

// executeScanWorkflow runs the scan workflow and returns consolidated results.
func (s *Scanner) executeScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID, targetSource string) (WorkflowResult, error) {
	summaryDataFromWorkflow, currentProbeResults, urlDiffResults, workflowErr := s.ExecuteCompleteScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)

	return WorkflowResult{
		SummaryData:    summaryDataFromWorkflow,
		ProbeResults:   currentProbeResults,
		URLDiffResults: urlDiffResults,
	}, workflowErr
}

// handleWorkflowError processes errors that occur during workflow execution.
func (s *Scanner) handleWorkflowError(summaryData models.ScanSummaryData, result WorkflowResult, logger zerolog.Logger, scanSessionID string, err error) (models.ScanSummaryData, []models.ProbeResult, []string, error) {
	logger.Error().Err(err).Str("scanSessionID", scanSessionID).Msg("Single scan workflow execution failed")

	s.updateSummaryWithResults(summaryData, result)
	summaryData.Status = string(models.ScanStatusFailed)

	if !common.ContainsCancellationError(summaryData.ErrorMessages) {
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Scan workflow failed: %v", err))
	}

	return summaryData, result.ProbeResults, nil, err
}

// handleWorkflowCancellation processes workflow cancellation scenarios.
func (s *Scanner) handleWorkflowCancellation(summaryData models.ScanSummaryData, result WorkflowResult, logger zerolog.Logger, scanSessionID string) (models.ScanSummaryData, []models.ProbeResult, []string, error) {
	logger.Info().Str("scanSessionID", scanSessionID).Msg("Single scan workflow interrupted by context cancellation")

	s.updateSummaryWithResults(summaryData, result)
	summaryData.Status = string(models.ScanStatusInterrupted)

	if !common.ContainsCancellationError(summaryData.ErrorMessages) {
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, "Scan workflow interrupted by context cancellation")
	}

	return summaryData, result.ProbeResults, nil, context.Canceled
}

// generateReports creates HTML reports from probe results.
func (s *Scanner) generateReports(ctx context.Context, gCfg *config.GlobalConfig, probeResults []models.ProbeResult, scanSessionID, scanMode, targetSource string, logger zerolog.Logger) ([]string, error) {
	if len(probeResults) == 0 && !gCfg.ReporterConfig.GenerateEmptyReport {
		logger.Info().Str("scanSessionID", scanSessionID).Msg("No probe results and GenerateEmptyReport is false. Skipping report generation")
		return nil, nil
	}

	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, logger)
	if err != nil {
		return nil, common.WrapError(err, "failed to initialize HTML reporter")
	}

	baseReportFilename := s.buildReportFilename(scanSessionID, scanMode, targetSource)

	probeResultsPtr := s.convertToPointerSlice(probeResults)

	generatedPaths, err := htmlReporter.GenerateReport(probeResultsPtr, baseReportFilename)
	if err != nil {
		return nil, common.WrapError(err, "failed to generate HTML report(s)")
	}

	s.logReportGeneration(logger, scanSessionID, generatedPaths)
	return generatedPaths, nil
}

// handleReportError processes errors that occur during report generation.
func (s *Scanner) handleReportError(summaryData models.ScanSummaryData, result WorkflowResult, logger zerolog.Logger, err error) (models.ScanSummaryData, []models.ProbeResult, []string, error) {
	summaryData.Status = string(models.ScanStatusFailed)
	msg := fmt.Sprintf("Failed to generate HTML report(s): %v", err)
	summaryData.ErrorMessages = append(summaryData.ErrorMessages, msg)
	logger.Error().Err(err).Msg(msg)

	s.updateSummaryWithResults(summaryData, result)
	return summaryData, result.ProbeResults, nil, err
}

// updateSummaryWithResults updates the summary data with workflow results.
func (s *Scanner) updateSummaryWithResults(summaryData models.ScanSummaryData, result WorkflowResult) {
	summaryData.ProbeStats = result.SummaryData.ProbeStats
	summaryData.DiffStats = result.SummaryData.DiffStats

	if result.SummaryData.ScanDuration > 0 {
		summaryData.ScanDuration = result.SummaryData.ScanDuration
	}

	summaryData.ErrorMessages = append(summaryData.ErrorMessages, result.SummaryData.ErrorMessages...)
}

// finalizeScanStatus sets the final status of the scan based on current state.
func (s *Scanner) finalizeScanStatus(summaryData models.ScanSummaryData) {
	if len(summaryData.ErrorMessages) == 0 {
		summaryData.Status = string(models.ScanStatusCompleted)
	} else {
		summaryData.Status = string(models.ScanStatusPartialComplete)
	}
}

// buildReportFilename creates a standardized filename for the report (without extension).
func (s *Scanner) buildReportFilename(scanSessionID, scanMode, targetSource string) string {
	sanitizedTargetSource := urlhandler.SanitizeFilename(targetSource)
	return fmt.Sprintf("%s_%s_%s_report", scanSessionID, scanMode, sanitizedTargetSource)
}

// convertToPointerSlice converts a slice of ProbeResult to a slice of pointers.
func (s *Scanner) convertToPointerSlice(probeResults []models.ProbeResult) []*models.ProbeResult {
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}
	return probeResultsPtr
}

// logReportGeneration logs the result of report generation.
func (s *Scanner) logReportGeneration(logger zerolog.Logger, scanSessionID string, generatedPaths []string) {
	if len(generatedPaths) == 0 {
		logger.Info().Str("scanSessionID", scanSessionID).Msg("HTML report generation resulted in no files")
	} else {
		logger.Info().Str("scanSessionID", scanSessionID).Strs("paths", generatedPaths).Msg("HTML report(s) generated successfully")
	}
}
