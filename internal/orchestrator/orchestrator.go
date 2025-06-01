package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// ScanOrchestrator handles the core logic of a scan workflow,
// including crawling, probing, diffing, and storing results.
// It coordinates various components like crawler, httpxrunner, differ, and datastore writers.
type ScanOrchestrator struct {
	config        *config.GlobalConfig
	logger        zerolog.Logger
	parquetReader *datastore.ParquetReader
	parquetWriter *datastore.ParquetWriter
	pathExtractor *extractor.PathExtractor
	fetcher       *monitor.Fetcher
}

// NewScanOrchestrator creates a new ScanOrchestrator instance.
// It initializes required components like PathExtractor and Fetcher.
// It logs a fatal error and returns nil if initialization of critical components fails.
func NewScanOrchestrator(
	globalConfig *config.GlobalConfig,
	logger zerolog.Logger,
	reader *datastore.ParquetReader,
	writer *datastore.ParquetWriter,
) *ScanOrchestrator {
	pathExtractor, err := extractor.NewPathExtractor(globalConfig.ExtractorConfig, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PathExtractor in ScanOrchestrator")
		return nil
	}

	// Initialize HTTP client using common factory
	httpClientFactory := common.NewHTTPClientFactory(logger)
	httpClient, err := httpClientFactory.CreateMonitorClient(
		time.Duration(globalConfig.HttpxRunnerConfig.TimeoutSecs)*time.Second,
		false, // insecureSkipVerify - use config default
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create HTTP client for ScanOrchestrator")
		return nil
	}

	fetcher := monitor.NewFetcher(httpClient, logger, &globalConfig.MonitorConfig)

	return &ScanOrchestrator{
		config:        globalConfig,
		logger:        logger.With().Str("module", "ScanOrchestrator").Logger(),
		parquetReader: reader,
		parquetWriter: writer,
		pathExtractor: pathExtractor, // Assign initialized PathExtractor
		fetcher:       fetcher,       // Assign Fetcher
	}
}

// ExecuteScanWorkflow runs the full scan workflow: crawl -> probe -> diff -> store.
// - seedURLs: Initial URLs to start crawling from.
// - scanSessionID: A unique identifier for this scan run, used for logging and data storage.
// The function returns probe results, URL diff results, any secret findings, and an error if the workflow fails.
// Errors from underlying components are wrapped with context.
func (so *ScanOrchestrator) ExecuteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string) ([]models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	// Configure crawler
	crawlerConfig, primaryRootTargetURL, err := so.prepareScanConfiguration(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to prepare scan configuration for session %s: %w", scanSessionID, err)
	}

	// Execute crawler
	discoveredURLs, err := so.executeCrawler(ctx, crawlerConfig, scanSessionID, primaryRootTargetURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("crawler execution failed for session %s: %w", scanSessionID, err)
	}

	// Execute HTTPX probing
	probeResults, err := so.executeHTTPXProbing(ctx, discoveredURLs, seedURLs, primaryRootTargetURL, scanSessionID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("HTTPX probing failed for session %s: %w", scanSessionID, err)
	}

	// Process diffing and storage
	urlDiffResults, err := so.processDiffingAndStorage(ctx, probeResults, seedURLs, primaryRootTargetURL, scanSessionID)
	if err != nil {
		return probeResults, urlDiffResults, nil, fmt.Errorf("diffing and storage failed for session %s: %w", scanSessionID, err)
	}

	so.logger.Info().Str("session_id", scanSessionID).Msg("Scan workflow finished.")
	// Return all *original* probe results from this scan, and the diff results map
	return probeResults, urlDiffResults, nil, nil
}

// scanWorkflowInput holds the input parameters for building a scan summary.
type scanWorkflowInput struct {
	scanSessionID string
	targetSource  string
	targets       []string
	startTime     time.Time
}

// scanWorkflowResult holds the result parameters for building a scan summary.
type scanWorkflowResult struct {
	probeResults   []models.ProbeResult
	urlDiffResults map[string]models.URLDiffResult
	secretFindings []models.SecretFinding
	workflowError  error
}

// ExecuteHTMLWorkflow executes the complete scan workflow specifically for HTML URLs.
// This typically involves crawling, probing, diffing, and storage.
// - htmlURLs: A list of HTML URLs to be processed.
// - scanSessionID: Unique ID for the scan session.
// - targetSource: Identifier for the source of these target URLs.
// It returns a ScanSummaryData, probe results, URL diff results, secret findings, and any error encountered.
func (so *ScanOrchestrator) ExecuteHTMLWorkflow(ctx context.Context, htmlURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	// For HTML URLs, we run the full scan workflow which includes crawling
	return so.ExecuteCompleteScanWorkflow(ctx, htmlURLs, scanSessionID, targetSource)
}

// ExecuteMonitorOnlyWorkflow executes a workflow focused on monitoring non-HTML URLs (e.g., JS, JSON).
// This workflow typically involves probing the URLs and then diffing/storing the results, bypassing crawling.
// - monitorURLs: A list of non-HTML URLs to be monitored.
// - scanSessionID: Unique ID for the scan session.
// - targetSource: Identifier for the source of these target URLs.
// It returns a ScanSummaryData, probe results, URL diff results, secret findings (though typically none for this flow), and any error encountered.
func (so *ScanOrchestrator) ExecuteMonitorOnlyWorkflow(ctx context.Context, monitorURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	startTime := time.Now()
	var overallWorkflowError error

	// Build summary data - initialize early for status updates
	// summaryData := models.GetDefaultScanSummaryData() // Initializing summaryData is now part of buildScanSummary
	// summaryData.ScanSessionID = scanSessionID
	// summaryData.TargetSource = targetSource
	// summaryData.Targets = monitorURLs
	// summaryData.TotalTargets = len(monitorURLs)

	inputParams := scanWorkflowInput{
		scanSessionID: scanSessionID,
		targetSource:  targetSource,
		targets:       monitorURLs,
		startTime:     startTime,
	}

	var probeResults []models.ProbeResult
	var secretFindings []models.SecretFinding
	var urlDiffResults map[string]models.URLDiffResult

	if len(monitorURLs) > 0 {
		var err error
		probeResults, err = so.executeHTTPXProbing(ctx, monitorURLs, monitorURLs, monitorURLs[0], scanSessionID)
		if err != nil {
			overallWorkflowError = fmt.Errorf("monitor workflow probing failed: %w", err)
			resultParams := scanWorkflowResult{
				probeResults:  probeResults,
				workflowError: overallWorkflowError,
			}
			summaryData := so.buildScanSummary(inputParams, resultParams)
			return summaryData, probeResults, nil, nil, overallWorkflowError
		}

		urlDiffResults, err = so.processDiffingAndStorage(ctx, probeResults, monitorURLs, monitorURLs[0], scanSessionID)
		if err != nil {
			overallWorkflowError = fmt.Errorf("monitor workflow diffing/storage failed: %w", err)
			resultParams := scanWorkflowResult{
				probeResults:   probeResults,
				urlDiffResults: urlDiffResults,
				secretFindings: secretFindings,
				workflowError:  overallWorkflowError,
			}
			summaryData := so.buildScanSummary(inputParams, resultParams)
			return summaryData, probeResults, urlDiffResults, secretFindings, overallWorkflowError
		}
	}

	resultParams := scanWorkflowResult{
		probeResults:   probeResults,
		urlDiffResults: urlDiffResults,
		secretFindings: secretFindings,
		workflowError:  overallWorkflowError,
	}
	summaryData := so.buildScanSummary(inputParams, resultParams)
	return summaryData, probeResults, urlDiffResults, secretFindings, overallWorkflowError
}

// ExecuteCompleteScanWorkflow executes the complete scan workflow (crawl, probe, diff, store)
// and then builds a comprehensive ScanSummaryData object.
// - seedURLs: Initial URLs to start the scan from.
// - scanSessionID: Unique ID for this scan session.
// - targetSource: Identifier for the source of these target URLs (e.g., filename, config).
// It returns the ScanSummaryData, probe results, URL diff results, secret findings, and any error encountered during the workflow.
func (so *ScanOrchestrator) ExecuteCompleteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	startTime := time.Now()

	probeResults, urlDiffResults, secretFindings, workflowError := so.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)

	inputParams := scanWorkflowInput{
		scanSessionID: scanSessionID,
		targetSource:  targetSource,
		targets:       seedURLs,
		startTime:     startTime,
	}
	resultParams := scanWorkflowResult{
		probeResults:   probeResults,
		urlDiffResults: urlDiffResults,
		secretFindings: secretFindings,
		workflowError:  workflowError,
	}
	summaryData := so.buildScanSummary(inputParams, resultParams)

	return summaryData, probeResults, urlDiffResults, secretFindings, workflowError
}

// buildScanSummary populates a ScanSummaryData object based on scan execution details.
func (so *ScanOrchestrator) buildScanSummary(
	input scanWorkflowInput,
	result scanWorkflowResult,
) models.ScanSummaryData {
	scanDuration := time.Since(input.startTime)
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = input.scanSessionID
	summaryData.TargetSource = input.targetSource
	summaryData.Targets = input.targets
	summaryData.TotalTargets = len(input.targets)
	summaryData.ScanDuration = scanDuration

	if result.probeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(result.probeResults)
		for _, pr := range result.probeResults {
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summaryData.ProbeStats.SuccessfulProbes++
			} else {
				summaryData.ProbeStats.FailedProbes++
			}
		}
	}

	if result.urlDiffResults != nil {
		for _, diffResult := range result.urlDiffResults {
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
		}
	}

	if result.workflowError != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", result.workflowError)}
	} else {
		summaryData.Status = string(models.ScanStatusCompleted)
	}
	return summaryData
}

// prepareScanConfiguration prepares crawler configuration for the scan
func (so *ScanOrchestrator) prepareScanConfiguration(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
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

// executeCrawler runs the crawler and returns discovered URLs
func (so *ScanOrchestrator) executeCrawler(ctx context.Context, crawlerConfig *config.CrawlerConfig, scanSessionID, primaryRootTargetURL string) ([]string, error) {
	var discoveredURLs []string

	// Run crawler if seed URLs provided
	if len(crawlerConfig.SeedURLs) > 0 {
		// Check for context cancellation before starting crawler
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "crawler start"); cancelled.Cancelled {
			return nil, cancelled.Error
		}

		so.logger.Info().Int("seed_count", len(crawlerConfig.SeedURLs)).Str("session_id", scanSessionID).Str("primary_target", primaryRootTargetURL).Msg("Starting crawler")

		// Create HTTP client for crawler using factory
		httpClientFactory := common.NewHTTPClientFactory(so.logger)
		crawlerClient, err := httpClientFactory.CreateCrawlerClient(
			time.Duration(crawlerConfig.RequestTimeoutSecs)*time.Second,
			"",                      // proxy - crawler config doesn't have proxy field
			make(map[string]string), // customHeaders - crawler config doesn't have custom headers
		)
		if err != nil {
			so.logger.Error().Err(err).Msg("Failed to create HTTP client for crawler")
			return nil, fmt.Errorf("orchestrator: failed to create crawler HTTP client: %w", err)
		}

		crawler, err := crawler.NewCrawler(crawlerConfig, crawlerClient, so.logger)
		if err != nil {
			so.logger.Error().Err(err).Msg("Failed to initialize crawler")
			return nil, fmt.Errorf("orchestrator: failed to initialize crawler: %w", err)
		}

		crawler.Start(ctx)
		discoveredURLs = crawler.GetDiscoveredURLs()
		so.logger.Info().Int("discovered_count", len(discoveredURLs)).Str("session_id", scanSessionID).Msg("Crawler finished")
	} else {
		so.logger.Info().Str("session_id", scanSessionID).Msg("No seed URLs provided, skipping crawler module.")
	}

	return discoveredURLs, nil
}

// executeHTTPXProbing runs HTTPX probing on discovered URLs and returns probe results
func (so *ScanOrchestrator) executeHTTPXProbing(ctx context.Context, discoveredURLs []string, seedURLs []string, primaryRootTargetURL, scanSessionID string) ([]models.ProbeResult, error) {
	var probeResults []models.ProbeResult

	if len(discoveredURLs) > 0 {
		// Check for context cancellation before starting HTTPX
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing start"); cancelled.Cancelled {
			return nil, cancelled.Error
		}

		so.logger.Info().Int("url_count", len(discoveredURLs)).Str("session_id", scanSessionID).Msg("Starting HTTPX probing")

		runnerConfig := &httpxrunner.Config{
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
		}

		// The primaryRootTargetURL for httpxrunner is mostly for context in its internal logging or potentially file naming if it did that.
		// The actual grouping of results for diffing/parquet will use the more granular rootTargetForThisURL derived from seedURLs.
		runner, err := httpxrunner.NewRunner(runnerConfig, primaryRootTargetURL, so.logger)
		if err != nil {
			so.logger.Error().Err(err).Str("session_id", scanSessionID).Msg("Failed to create HTTPX runner")
			return nil, fmt.Errorf("orchestrator: failed to create HTTPX runner for session %s: %w", scanSessionID, err)
		}

		if err := runner.Run(ctx); err != nil { // Pass context to Run
			so.logger.Warn().Err(err).Str("session_id", scanSessionID).Msg("HTTPX probing encountered errors")
			// Continue processing with any results obtained, unless context was cancelled
			if ctx.Err() == context.Canceled {
				so.logger.Info().Str("session_id", scanSessionID).Msg("HTTPX probing cancelled.")
				return probeResults, ctx.Err() // Return partial results if any, and context error
			} else {
				// If runner.Run failed with an error other than context cancellation,
				// return this error to the caller.
				return probeResults, fmt.Errorf("httpx runner.Run failed for session %s: %w", scanSessionID, err)
			}
		}

		// Check for context cancellation after HTTPX Run
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing completion"); cancelled.Cancelled {
			return probeResults, cancelled.Error
		}

		runnerResults := runner.GetResults()
		probeResultMap := make(map[string]models.ProbeResult)
		for _, r := range runnerResults {
			probeResultMap[r.InputURL] = r
		}

		// Map results back to discovered URLs and assign RootTargetURL
		for _, urlString := range discoveredURLs {
			rootTargetForThisURL := urlhandler.GetRootTargetForURL(urlString, seedURLs)
			if r, ok := probeResultMap[urlString]; ok {
				probeResult := r
				probeResult.RootTargetURL = rootTargetForThisURL
				probeResults = append(probeResults, probeResult)
			} else {
				so.logger.Warn().Str("url", urlString).Msg("No probe result from httpx for discovered URL, creating error entry.")
				probeResults = append(probeResults, models.ProbeResult{
					InputURL:      urlString,
					Error:         "No response or error during httpx probe",
					Timestamp:     time.Now(),
					RootTargetURL: rootTargetForThisURL,
				})
			}
		}
	} else {
		so.logger.Info().Str("session_id", scanSessionID).Msg("No URLs discovered by crawler or crawler skipped, skipping HTTPX probing.")
	}
	so.logger.Info().Int("count", len(probeResults)).Str("session_id", scanSessionID).Msg("Processed probe results from current scan")

	return probeResults, nil
}

// diffAndPrepareStorageForTarget performs URL diffing for a specific target and prepares probe results for storage.
// It returns a new slice of ProbeResult with updated URLStatus and OldestScanTimestamp fields.
func (so *ScanOrchestrator) diffAndPrepareStorageForTarget(
	ctx context.Context,
	rootTarget string,
	probeResultsForTarget []models.ProbeResult, // Changed from []*models.ProbeResult
	scanSessionID string,
	urlDiffer *differ.UrlDiffer,
) (*models.URLDiffResult, []models.ProbeResult, error) { // Return []models.ProbeResult
	so.logger.Info().Str("root_target", rootTarget).Int("current_results_count", len(probeResultsForTarget)).Str("session_id", scanSessionID).Msg("Processing diff for root target")

	// Create a slice of pointers to copies for the differ, as it might modify them.
	probePointersForDiff := make([]*models.ProbeResult, len(probeResultsForTarget))
	for i := range probeResultsForTarget {
		copy := probeResultsForTarget[i]
		probePointersForDiff[i] = &copy
	}

	diffResult, err := urlDiffer.Compare(probePointersForDiff, rootTarget)
	if err != nil {
		so.logger.Error().Err(err).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Failed to compare URLs. Skipping storage and diff summary for this target.")
		return nil, nil, fmt.Errorf("urlDiffer.Compare failed for target %s, session %s: %w", rootTarget, scanSessionID, err)
	}

	if diffResult == nil {
		so.logger.Warn().Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("DiffResult was nil, though no explicit error. Skipping further processing for this target.")
		return nil, nil, nil // No error, but nothing to process
	}

	so.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Int("new", diffResult.New).Int("old", diffResult.Old).Int("existing", diffResult.Existing).Int("total_diff_urls", len(diffResult.Results)).Msg("URL Diffing complete for target.")

	updatedProbesToStore := make([]models.ProbeResult, 0, len(diffResult.Results))
	for _, diffedURL := range diffResult.Results {
		updatedProbesToStore = append(updatedProbesToStore, diffedURL.ProbeResult) // This ProbeResult has updated fields
	}

	return diffResult, updatedProbesToStore, nil
}

// processDiffingAndStorage processes URL diffing and stores results to Parquet
func (so *ScanOrchestrator) processDiffingAndStorage(ctx context.Context, currentScanProbeResults []models.ProbeResult, seedURLs []string, primaryRootTargetURL, scanSessionID string) (map[string]models.URLDiffResult, error) {
	// Group results by root target
	probeResultsByRootTarget := make(map[string][]models.ProbeResult)
	// Maintain original indices to update the main currentScanProbeResults slice later if needed,
	// though direct modification is now reduced by diffAndPrepareStorageForTarget returning new slices.
	originalIndicesByRootTarget := make(map[string][]int)

	for i, probeResult := range currentScanProbeResults {
		rootTargetURL := probeResult.RootTargetURL
		if rootTargetURL == "" {
			rootTargetURL = primaryRootTargetURL
			if rootTargetURL == "" && len(seedURLs) > 0 {
				rootTargetURL = seedURLs[0]
			} else if rootTargetURL == "" {
				// Fallback to InputURL if no other root can be determined.
				// This is a safeguard, RootTargetURL should ideally always be set.
				rootTargetURL = probeResult.InputURL
				so.logger.Warn().Str("url", probeResult.InputURL).Str("session_id", scanSessionID).Msg("RootTargetURL was empty, falling back to InputURL for grouping.")
			}
		}
		probeResultsByRootTarget[rootTargetURL] = append(probeResultsByRootTarget[rootTargetURL], probeResult)
		originalIndicesByRootTarget[rootTargetURL] = append(originalIndicesByRootTarget[rootTargetURL], i)
	}

	urlDiffer := differ.NewUrlDiffer(so.parquetReader, so.logger)
	urlDiffResults := make(map[string]models.URLDiffResult)
	// This will hold all probe results that are candidates for writing, after diffing.
	// It replaces the direct modification of currentScanProbeResults for status updates.
	var allProbesToStore []models.ProbeResult

	for rootTarget, resultsForRoot := range probeResultsByRootTarget {
		if rootTarget == "" {
			so.logger.Warn().Str("session_id", scanSessionID).Msg("Skipping diffing/storage for empty root target.")
			continue
		}

		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "diff/store for target: "+rootTarget); cancelled.Cancelled {
			return urlDiffResults, cancelled.Error
		}

		// Pass the actual slice of ProbeResult, not pointers.
		// The helper function will return a new slice with updated fields.
		diffResultData, updatedProbesForTarget, err := so.diffAndPrepareStorageForTarget(ctx, rootTarget, resultsForRoot, scanSessionID, urlDiffer)

		if err != nil {
			// Logged in helper, decide if we should continue with other targets or fail
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return urlDiffResults, err // Propagate context error
			}
			continue // Skip this target due to other diffing error
		}
		if diffResultData == nil { // No error, but nil diff result means skip
			continue
		}

		urlDiffResults[rootTarget] = *diffResultData
		allProbesToStore = append(allProbesToStore, updatedProbesForTarget...)

		// Update the original currentScanProbeResults slice with the status from updatedProbesForTarget.
		// This ensures the summary data built later reflects the diff status correctly.
		// Create a map for efficient lookup of updated probes by InputURL.
		updatedProbesMap := make(map[string]models.ProbeResult, len(updatedProbesForTarget))
		for _, p := range updatedProbesForTarget {
			updatedProbesMap[p.InputURL] = p
		}

		// Iterate over the original indices for the current rootTarget
		// and update the corresponding entries in currentScanProbeResults.
		for _, originalIndex := range originalIndicesByRootTarget[rootTarget] {
			originalProbe := &currentScanProbeResults[originalIndex] // Get a pointer to modify in place
			if updatedProbe, ok := updatedProbesMap[originalProbe.InputURL]; ok {
				originalProbe.URLStatus = updatedProbe.URLStatus
				originalProbe.OldestScanTimestamp = updatedProbe.OldestScanTimestamp
			}
		}

		// Write to Parquet for the current rootTarget using updatedProbesForTarget
		if so.parquetWriter != nil {
			if len(updatedProbesForTarget) > 0 {
				so.logger.Info().Int("count", len(updatedProbesForTarget)).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Writing probe results to Parquet...")
				if err := so.parquetWriter.Write(ctx, updatedProbesForTarget, scanSessionID, rootTarget); err != nil {
					so.logger.Error().Err(err).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Failed to write Parquet data")
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return urlDiffResults, err
					}
				}
			} else {
				so.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("No probe results to store to Parquet for target.")
			}
		} else {
			so.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("ParquetWriter is not initialized. Skipping Parquet storage for target.")
		}
	}

	return urlDiffResults, nil
}
