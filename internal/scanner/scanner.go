package scanner

import (
	"context"
	"fmt"
	"path/filepath"

	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// Scanner handles the core logic of a scan workflow,
// including crawling, probing, diffing, and storing results.
// It coordinates various components like crawler, httpxrunner, differ, and datastore writers.
type Scanner struct {
	config        *config.GlobalConfig
	logger        zerolog.Logger
	parquetReader *datastore.ParquetReader
	parquetWriter *datastore.ParquetWriter
	pathExtractor *extractor.PathExtractor
}

// NewScanner creates a new Scanner instance.
// It initializes required components like PathExtractor and Fetcher.
// It logs a fatal error and returns nil if initialization of critical components fails.
func NewScanner(
	globalConfig *config.GlobalConfig,
	logger zerolog.Logger,
	pReader *datastore.ParquetReader,
	pWriter *datastore.ParquetWriter,
) *Scanner {
	pathExtractor, err := extractor.NewPathExtractor(globalConfig.ExtractorConfig, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PathExtractor in NewScanner")
		return nil
	}

	return &Scanner{
		config:        globalConfig,
		logger:        logger.With().Str("module", "Scanner").Logger(),
		parquetReader: pReader,
		parquetWriter: pWriter,
		pathExtractor: pathExtractor,
	}
}

// ExecuteSingleScanWorkflowWithReporting encapsulates the core logic of running a single scan cycle,
// including orchestrating the scan, generating reports, and returning a summary.
// It does NOT send notifications; that is left to the caller.
func (s *Scanner) ExecuteSingleScanWorkflowWithReporting(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	appLogger zerolog.Logger,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
	scanMode string,
) (models.ScanSummaryData, []models.ProbeResult, []string, error) {
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = scanSessionID
	summaryData.TargetSource = targetSource
	summaryData.ScanMode = scanMode
	summaryData.Targets = seedURLs
	summaryData.TotalTargets = len(seedURLs)

	var probeResults []models.ProbeResult
	var workflowErr error
	var reportFilePaths []string

	summaryDataFromWorkflow, currentProbeResults, _, workflowErr := s.ExecuteCompleteScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)

	summaryData.ProbeStats = summaryDataFromWorkflow.ProbeStats
	summaryData.DiffStats = summaryDataFromWorkflow.DiffStats
	if summaryDataFromWorkflow.ScanDuration > 0 {
		summaryData.ScanDuration = summaryDataFromWorkflow.ScanDuration
	}
	summaryData.ErrorMessages = append(summaryData.ErrorMessages, summaryDataFromWorkflow.ErrorMessages...)

	if workflowErr != nil {
		appLogger.Error().Err(workflowErr).Str("scanSessionID", scanSessionID).Msg("Single scan workflow execution failed")
		summaryData.Status = string(models.ScanStatusFailed)
		if !common.ContainsCancellationError(summaryData.ErrorMessages) {
			summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Scan workflow failed: %v", workflowErr))
		}
		return summaryData, currentProbeResults, nil, workflowErr
	}
	if ctx.Err() != nil {
		appLogger.Info().Str("scanSessionID", scanSessionID).Msg("Single scan workflow interrupted by context cancellation after workflow completion call.")
		summaryData.Status = string(models.ScanStatusInterrupted)
		if !common.ContainsCancellationError(summaryData.ErrorMessages) {
			summaryData.ErrorMessages = append(summaryData.ErrorMessages, "Scan workflow interrupted by context cancellation.")
		}
		return summaryData, currentProbeResults, nil, ctx.Err()
	}

	appLogger.Info().Str("scanSessionID", scanSessionID).Int("probe_results_count", len(currentProbeResults)).Msg("Scan workflow completed successfully. Generating report...")
	probeResults = currentProbeResults

	if len(probeResults) > 0 || gCfg.ReporterConfig.GenerateEmptyReport {
		htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
		if err != nil {
			summaryData.Status = string(models.ScanStatusFailed)
			msg := fmt.Sprintf("Failed to initialize HTML reporter: %v", err)
			summaryData.ErrorMessages = append(summaryData.ErrorMessages, msg)
			appLogger.Error().Err(err).Msg(msg)
			return summaryData, probeResults, nil, fmt.Errorf(msg)
		}

		baseReportFilename := fmt.Sprintf("%s_%s_%s_report.html", scanSessionID, scanMode, urlhandler.SanitizeFilename(targetSource))
		baseReportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, baseReportFilename)

		probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
		for i := range probeResults {
			probeResultsPtr[i] = &probeResults[i]
		}

		generatedPaths, reportGenErr := htmlReporter.GenerateReport(probeResultsPtr, baseReportPath)
		if reportGenErr != nil {
			summaryData.Status = string(models.ScanStatusFailed)
			msg := fmt.Sprintf("Failed to generate HTML report(s): %v", reportGenErr)
			summaryData.ErrorMessages = append(summaryData.ErrorMessages, msg)
			appLogger.Error().Err(reportGenErr).Msg(msg)
		} else {
			reportFilePaths = generatedPaths
			if len(reportFilePaths) == 0 {
				appLogger.Info().Str("scanSessionID", scanSessionID).Msg("HTML report generation resulted in no files.")
				summaryData.ReportPath = ""
			} else {
				appLogger.Info().Str("scanSessionID", scanSessionID).Strs("paths", reportFilePaths).Msg("HTML report(s) generated successfully.")
				if len(reportFilePaths) == 1 {
					summaryData.ReportPath = reportFilePaths[0]
				} else {
					summaryData.ReportPath = fmt.Sprintf("Multiple report files generated (%d parts)", len(reportFilePaths))
				}
			}
		}
	} else {
		appLogger.Info().Str("scanSessionID", scanSessionID).Msg("No probe results and GenerateEmptyReport is false. Skipping report generation.")
		summaryData.ReportPath = ""
	}

	if workflowErr == nil && ctx.Err() == nil {
		if len(summaryData.ErrorMessages) == 0 {
			summaryData.Status = string(models.ScanStatusCompleted)
		} else {
			summaryData.Status = string(models.ScanStatusPartialComplete)
		}
	}

	return summaryData, probeResults, reportFilePaths, workflowErr
}

// ExecuteCompleteScanWorkflow executes the complete scan workflow (crawl, probe, diff, store)
// and then builds a comprehensive ScanSummaryData object.
func (s *Scanner) ExecuteCompleteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, error) {
	startTime := time.Now()

	probeResults, urlDiffResults, workflowError := s.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)

	inputParams := scanWorkflowInput{
		ScanSessionID: scanSessionID,
		TargetSource:  targetSource,
		Targets:       seedURLs,
		StartTime:     startTime,
	}
	resultParams := scanWorkflowResult{
		ProbeResults:   probeResults,
		URLDiffResults: urlDiffResults,
		WorkflowError:  workflowError,
	}
	summaryData := s.buildScanSummary(inputParams, resultParams)

	return summaryData, probeResults, urlDiffResults, workflowError
}

// ExecuteScanWorkflow runs the full scan workflow: crawl -> probe -> diff -> store.
func (s *Scanner) ExecuteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {
	crawlerConfig, primaryRootTargetURL, err := s.prepareScanConfiguration(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare scan configuration for session %s: %w", scanSessionID, err)
	}

	discoveredURLs, err := s.executeCrawler(ctx, crawlerConfig, scanSessionID, primaryRootTargetURL)
	if err != nil {
		return nil, nil, fmt.Errorf("crawler execution failed for session %s: %w", scanSessionID, err)
	}

	httpxInput := HTTPXProbingInput{
		DiscoveredURLs:       discoveredURLs,
		SeedURLs:             seedURLs,
		PrimaryRootTargetURL: primaryRootTargetURL,
		ScanSessionID:        scanSessionID,
		HttpxRunnerConfig: &httpxrunner.Config{
			Targets:              discoveredURLs,
			Method:               s.config.HttpxRunnerConfig.Method,
			RequestURIs:          s.config.HttpxRunnerConfig.RequestURIs,
			FollowRedirects:      s.config.HttpxRunnerConfig.FollowRedirects,
			Timeout:              s.config.HttpxRunnerConfig.TimeoutSecs,
			Retries:              s.config.HttpxRunnerConfig.Retries,
			Threads:              s.config.HttpxRunnerConfig.Threads,
			CustomHeaders:        s.config.HttpxRunnerConfig.CustomHeaders,
			Proxy:                s.config.HttpxRunnerConfig.Proxy,
			Verbose:              s.config.HttpxRunnerConfig.Verbose,
			TechDetect:           s.config.HttpxRunnerConfig.TechDetect,
			ExtractTitle:         s.config.HttpxRunnerConfig.ExtractTitle,
			ExtractStatusCode:    s.config.HttpxRunnerConfig.ExtractStatusCode,
			ExtractLocation:      s.config.HttpxRunnerConfig.ExtractLocation,
			ExtractContentLength: s.config.HttpxRunnerConfig.ExtractContentLength,
			ExtractServerHeader:  s.config.HttpxRunnerConfig.ExtractServerHeader,
			ExtractContentType:   s.config.HttpxRunnerConfig.ExtractContentType,
			ExtractIPs:           s.config.HttpxRunnerConfig.ExtractIPs,
			ExtractBody:          s.config.HttpxRunnerConfig.ExtractBody,
			ExtractHeaders:       s.config.HttpxRunnerConfig.ExtractHeaders,
		},
	}
	probeResults, err := s.executeHTTPXProbing(ctx, httpxInput)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPX probing failed for session %s: %w", scanSessionID, err)
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
		return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults, fmt.Errorf("diffing and storage failed for session %s: %w", scanSessionID, err)
	}

	s.logger.Info().Str("session_id", scanSessionID).Msg("Scan workflow finished.")
	return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults, nil
}

// prepareScanConfiguration prepares crawler configuration for the scan
func (s *Scanner) prepareScanConfiguration(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
	crawlerConfig := &s.config.CrawlerConfig
	currentCrawlerConfig := *crawlerConfig
	currentCrawlerConfig.SeedURLs = seedURLs

	if currentCrawlerConfig.AutoAddSeedHostnames && len(seedURLs) > 0 {
		seedHostnames := crawler.ExtractHostnamesFromSeedURLs(seedURLs, s.logger)
		if len(seedHostnames) > 0 {
			currentCrawlerConfig.Scope.AllowedHostnames = crawler.MergeAllowedHostnames(
				currentCrawlerConfig.Scope.AllowedHostnames,
				seedHostnames,
			)
			s.logger.Info().
				Strs("seed_hostnames", seedHostnames).
				Strs("original_allowed_hostnames", crawlerConfig.Scope.AllowedHostnames).
				Strs("final_allowed_hostnames", currentCrawlerConfig.Scope.AllowedHostnames).
				Str("session_id", scanSessionID).
				Msg("Scanner: Auto-added seed hostnames to allowed hostnames")
		}
	}

	var primaryRootTargetURL string
	if len(seedURLs) > 0 {
		primaryRootTargetURL = seedURLs[0]
	} else {
		primaryRootTargetURL = "unknown_target_" + scanSessionID
	}

	return &currentCrawlerConfig, primaryRootTargetURL, nil
}

// scanWorkflowInput holds the input parameters for building a ScanSummaryData object.
// It encapsulates details about the scan session and targets.
type scanWorkflowInput struct {
	ScanSessionID string    // Unique identifier for the current scan session.
	TargetSource  string    // Describes the origin of the scan targets (e.g., file name, configuration key).
	Targets       []string  // The list of initial target URLs or identifiers for the scan.
	StartTime     time.Time // The time when the scan workflow (or relevant part of it) started.
}

// scanWorkflowResult holds the output parameters from a scan workflow execution,
// used for building a ScanSummaryData object.
// It consolidates various results and any errors encountered.
type scanWorkflowResult struct {
	ProbeResults   []models.ProbeResult            // Results from the probing phase of the scan.
	URLDiffResults map[string]models.URLDiffResult // Results from the URL diffing phase, keyed by root target URL.
	WorkflowError  error                           // Any error that occurred during the overall workflow execution.
}

// buildScanSummary populates a ScanSummaryData object based on scan execution details.
func (s *Scanner) buildScanSummary(
	input scanWorkflowInput,
	result scanWorkflowResult,
) models.ScanSummaryData {
	scanDuration := time.Since(input.StartTime)
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = input.ScanSessionID
	summaryData.TargetSource = input.TargetSource
	summaryData.Targets = input.Targets
	summaryData.TotalTargets = len(input.Targets)
	summaryData.ScanDuration = scanDuration

	if result.ProbeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(result.ProbeResults)
		for _, pr := range result.ProbeResults {
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summaryData.ProbeStats.SuccessfulProbes++
			} else {
				summaryData.ProbeStats.FailedProbes++
			}
		}
	}

	if result.URLDiffResults != nil {
		for _, diffResult := range result.URLDiffResults {
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
		}
	}

	if result.WorkflowError != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", result.WorkflowError)}
	} else {
		summaryData.Status = string(models.ScanStatusCompleted)
	}
	return summaryData
}
