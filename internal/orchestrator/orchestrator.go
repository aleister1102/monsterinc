package orchestrator

import (
	"context"
	"fmt"

	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"

	"github.com/rs/zerolog"
)

// Orchestrator handles the core logic of a scan workflow,
// including crawling, probing, diffing, and storing results.
// It coordinates various components like crawler, httpxrunner, differ, and datastore writers.
type Orchestrator struct {
	config        *config.GlobalConfig
	logger        zerolog.Logger
	parquetReader *datastore.ParquetReader
	parquetWriter *datastore.ParquetWriter
	pathExtractor *extractor.PathExtractor
	fetcher       *monitor.Fetcher
}

// NewOrchestrator creates a new Orchestrator instance.
// It initializes required components like PathExtractor and Fetcher.
// It logs a fatal error and returns nil if initialization of critical components fails.
func NewOrchestrator(
	globalConfig *config.GlobalConfig,
	logger zerolog.Logger,
	pReader *datastore.ParquetReader,
	pWriter *datastore.ParquetWriter,
) *Orchestrator {
	// Initialize PathExtractor
	pathExtractor, err := extractor.NewPathExtractor(globalConfig.ExtractorConfig, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PathExtractor in NewOrchestrator")
		return nil
	}

	// Initialize HTTP client using common factory for monitoring
	httpClientFactory := common.NewHTTPClientFactory(logger)
	httpClient, err := httpClientFactory.CreateMonitorClient(
		time.Duration(globalConfig.HttpxRunnerConfig.TimeoutSecs)*time.Second,
		false, // insecureSkipVerify - use config default
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create HTTP client for NewOrchestrator")
		return nil
	}

	// Initialize Fetcher
	fetcher := monitor.NewFetcher(httpClient, logger, &globalConfig.MonitorConfig)

	return &Orchestrator{
		config:        globalConfig,
		logger:        logger.With().Str("module", "NewOrchestrator").Logger(),
		parquetReader: pReader,
		parquetWriter: pWriter,
		pathExtractor: pathExtractor, // Assign initialized PathExtractor
		fetcher:       fetcher,       // Assign Fetcher
	}
}

// ExecuteCompleteScanWorkflow executes the complete scan workflow (crawl, probe, diff, store)
// and then builds a comprehensive ScanSummaryData object.
//   - seedURLs: Initial URLs to start the scan from.
//   - scanSessionID: Unique ID for this scan session.
//   - targetSource: Identifier for the source of these target URLs (e.g., filename, config, etc.).
//
// It returns the ScanSummaryData, probe results, URL diff results, and any error encountered during the workflow.
func (so *Orchestrator) ExecuteCompleteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
	targetSource string,
) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, error) {
	startTime := time.Now()

	probeResults, urlDiffResults, workflowError := so.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)

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
	summaryData := so.buildScanSummary(inputParams, resultParams)

	return summaryData, probeResults, urlDiffResults, workflowError
}

// ExecuteScanWorkflow runs the full scan workflow: crawl -> probe -> diff -> store.
//   - seedURLs: Initial URLs to start crawling from.
//   - scanSessionID: A unique identifier for this scan run, used for logging and data storage.
//
// The function returns probe results, URL diff results, and an error if the workflow fails.
// Errors from underlying components are wrapped with context.
func (so *Orchestrator) ExecuteScanWorkflow(
	ctx context.Context,
	seedURLs []string,
	scanSessionID string,
) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {
	// Configure crawler
	crawlerConfig, primaryRootTargetURL, err := so.prepareScanConfiguration(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare scan configuration for session %s: %w", scanSessionID, err)
	}

	// Execute crawler
	discoveredURLs, err := so.executeCrawler(ctx, crawlerConfig, scanSessionID, primaryRootTargetURL)
	if err != nil {
		return nil, nil, fmt.Errorf("crawler execution failed for session %s: %w", scanSessionID, err)
	}

	// Execute HTTPX probing
	httpxInput := HTTPXProbingInput{
		DiscoveredURLs:       discoveredURLs,
		SeedURLs:             seedURLs,
		PrimaryRootTargetURL: primaryRootTargetURL,
		ScanSessionID:        scanSessionID,
		HttpxRunnerConfig: &httpxrunner.Config{ // Constructing HttpxRunnerConfig from so.config
			Targets:              discoveredURLs,
			Method:               so.config.HttpxRunnerConfig.Method,
			RequestURIs:          so.config.HttpxRunnerConfig.RequestURIs,
			FollowRedirects:      so.config.HttpxRunnerConfig.FollowRedirects,
			Timeout:              so.config.HttpxRunnerConfig.TimeoutSecs,
			Retries:              so.config.HttpxRunnerConfig.Retries,
			Threads:              so.config.HttpxRunnerConfig.Threads,
			CustomHeaders:        so.config.HttpxRunnerConfig.CustomHeaders,
			Proxy:                so.config.HttpxRunnerConfig.Proxy,
			Verbose:              so.config.HttpxRunnerConfig.Verbose,
			TechDetect:           so.config.HttpxRunnerConfig.TechDetect,
			ExtractTitle:         so.config.HttpxRunnerConfig.ExtractTitle,
			ExtractStatusCode:    so.config.HttpxRunnerConfig.ExtractStatusCode,
			ExtractLocation:      so.config.HttpxRunnerConfig.ExtractLocation,
			ExtractContentLength: so.config.HttpxRunnerConfig.ExtractContentLength,
			ExtractServerHeader:  so.config.HttpxRunnerConfig.ExtractServerHeader,
			ExtractContentType:   so.config.HttpxRunnerConfig.ExtractContentType,
			ExtractIPs:           so.config.HttpxRunnerConfig.ExtractIPs,
			ExtractBody:          so.config.HttpxRunnerConfig.ExtractBody,
			ExtractHeaders:       so.config.HttpxRunnerConfig.ExtractHeaders,
		},
	}
	probeResults, err := so.executeHTTPXProbing(ctx, httpxInput)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPX probing failed for session %s: %w", scanSessionID, err)
	}

	// Process diffing and storage
	diffStoreInput := ProcessDiffingAndStorageInput{
		Ctx:                     ctx,
		CurrentScanProbeResults: probeResults,
		SeedURLs:                seedURLs,
		PrimaryRootTargetURL:    primaryRootTargetURL,
		ScanSessionID:           scanSessionID,
	}
	diffStoreOutput, err := so.processDiffingAndStorage(diffStoreInput)
	if err != nil {
		// If processDiffingAndStorage returns an error, we still have the original probeResults.
		// The URLDiffResults might be partially populated in diffStoreOutput.
		return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults, fmt.Errorf("diffing and storage failed for session %s: %w", scanSessionID, err)
	}

	so.logger.Info().Str("session_id", scanSessionID).Msg("Scan workflow finished.")
	// Return all *original* probe results from this scan, and the diff results map
	// The probeResults passed to buildScanSummary should be the ones updated with URLStatus.
	return diffStoreOutput.UpdatedScanProbeResults, diffStoreOutput.URLDiffResults, nil
}

// prepareScanConfiguration prepares crawler configuration for the scan
func (so *Orchestrator) prepareScanConfiguration(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
	// Configure crawler
	crawlerConfig := &so.config.CrawlerConfig
	// Important: Make a copy or ensure SeedURLs is set fresh for each call
	// to avoid issues if the underlying slice in globalConfig is modified elsewhere
	// or if multiple orchestrators run with modified global configs.
	currentCrawlerConfig := *crawlerConfig // Shallow copy is usually fine for config structs
	currentCrawlerConfig.SeedURLs = seedURLs

	// Auto-add seed hostnames to allowed hostnames if enabled and seeds provided via parameter
	if currentCrawlerConfig.AutoAddSeedHostnames && len(seedURLs) > 0 {
		seedHostnames := crawler.ExtractHostnamesFromSeedURLs(seedURLs, so.logger)
		if len(seedHostnames) > 0 {
			currentCrawlerConfig.Scope.AllowedHostnames = crawler.MergeAllowedHostnames(
				currentCrawlerConfig.Scope.AllowedHostnames,
				seedHostnames,
			)
			so.logger.Info().
				Strs("seed_hostnames", seedHostnames).
				Strs("original_allowed_hostnames", crawlerConfig.Scope.AllowedHostnames).
				Strs("final_allowed_hostnames", currentCrawlerConfig.Scope.AllowedHostnames).
				Str("session_id", scanSessionID).
				Msg("Orchestrator: Auto-added seed hostnames to allowed hostnames")
		}
	}

	var primaryRootTargetURL string
	if len(seedURLs) > 0 {
		primaryRootTargetURL = seedURLs[0] // Assuming the first seed is the primary target for this session
	} else {
		// Fallback if no seeds, though ideally, seedURLs should not be empty for a meaningful scan.
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
func (so *Orchestrator) buildScanSummary(
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
