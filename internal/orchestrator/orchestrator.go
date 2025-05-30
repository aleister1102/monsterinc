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
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/secrets"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// ScanOrchestrator handles the core logic of a scan workflow.
type ScanOrchestrator struct {
	globalConfig   *config.GlobalConfig
	logger         zerolog.Logger
	parquetReader  *datastore.ParquetReader
	parquetWriter  *datastore.ParquetWriter
	pathExtractor  *extractor.PathExtractor
	secretDetector *secrets.SecretDetectorService
	fetcher        *monitor.Fetcher
	// latestProbeResults map[string][]models.ProbeResult // Potentially for caching latest results per target
}

// NewScanOrchestrator creates a new ScanOrchestrator.
func NewScanOrchestrator(
	cfg *config.GlobalConfig,
	logger zerolog.Logger,
	pqReader *datastore.ParquetReader,
	pqWriter *datastore.ParquetWriter,
	secDetector *secrets.SecretDetectorService,
) *ScanOrchestrator {
	// Initialize PathExtractor with ExtractorConfig
	// Ensure that the logger passed to PathExtractor is appropriately scoped if needed.
	// For now, using the orchestrator's base logger.
	pathExtractorInstance, err := extractor.NewPathExtractor(cfg.ExtractorConfig, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize PathExtractor in ScanOrchestrator")
		// Depending on error handling strategy, you might return nil or panic.
		// For a fatal initialization error, exiting or returning nil (if signature allows) is common.
		return nil // Or handle error more gracefully if NewScanOrchestrator can return an error.
	}

	// Initialize HTTP client using common factory
	httpClientFactory := common.NewHTTPClientFactory(logger)
	httpClient, err := httpClientFactory.CreateMonitorClient(
		time.Duration(cfg.HttpxRunnerConfig.TimeoutSecs)*time.Second,
		false, // insecureSkipVerify - use config default
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create HTTP client for ScanOrchestrator")
		return nil
	}

	fetcherInstance := monitor.NewFetcher(httpClient, logger, &cfg.MonitorConfig)

	return &ScanOrchestrator{
		globalConfig:   cfg,
		logger:         logger.With().Str("module", "ScanOrchestrator").Logger(),
		parquetReader:  pqReader,
		parquetWriter:  pqWriter,
		pathExtractor:  pathExtractorInstance, // Assign initialized PathExtractor
		secretDetector: secDetector,           // Assign SecretDetectorService
		fetcher:        fetcherInstance,       // Assign Fetcher
		// latestProbeResults: make(map[string][]models.ProbeResult),
	}
}

// ExecuteScanWorkflow runs the full crawl -> probe -> diff -> store workflow.
// seedURLs are the initial URLs to start crawling from.
// scanSessionID is a unique identifier for this specific scan run, used for Parquet naming.
// ctx is used for cancellation of long-running operations.
// Returns: probeResults, urlDiffResults, secretFindings, error
func (so *ScanOrchestrator) ExecuteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string) ([]models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	// Configure crawler
	crawlerCfg, primaryRootTargetURL, err := so.prepareScanConfiguration(seedURLs, scanSessionID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Execute crawler
	discoveredURLs, err := so.executeCrawler(ctx, crawlerCfg, scanSessionID, primaryRootTargetURL)
	if err != nil {
		return nil, nil, nil, err
	}

	// Execute HTTPX probing
	allProbeResultsForCurrentScan, err := so.executeHTTPXProbing(ctx, discoveredURLs, seedURLs, primaryRootTargetURL, scanSessionID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Execute secret detection
	allSecretFindings, err := so.executeSecretDetection(ctx, allProbeResultsForCurrentScan, scanSessionID)
	if err != nil {
		return allProbeResultsForCurrentScan, nil, nil, err
	}

	// Process diffing and storage
	allURLDiffResults, err := so.processDiffingAndStorage(ctx, allProbeResultsForCurrentScan, seedURLs, primaryRootTargetURL, scanSessionID)
	if err != nil {
		return allProbeResultsForCurrentScan, allURLDiffResults, allSecretFindings, err
	}

	so.logger.Info().Str("session_id", scanSessionID).Msg("Scan workflow finished.")
	// Return all *original* probe results from this scan, and the diff results map
	return allProbeResultsForCurrentScan, allURLDiffResults, allSecretFindings, nil
}

// ExecuteHTMLWorkflow executes both scan and monitor workflow for HTML URLs
func (so *ScanOrchestrator) ExecuteHTMLWorkflow(ctx context.Context, htmlURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	// For HTML URLs, we run the full scan workflow which includes crawling
	return so.ExecuteCompleteScanWorkflow(ctx, htmlURLs, scanSessionID, targetSource)
}

// ExecuteMonitorOnlyWorkflow executes only monitor workflow for non-HTML URLs (JS, JSON, etc.)
func (so *ScanOrchestrator) ExecuteMonitorOnlyWorkflow(ctx context.Context, monitorURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	startTime := time.Now()

	// Build summary data
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = scanSessionID
	summaryData.TargetSource = targetSource
	summaryData.Targets = monitorURLs
	summaryData.TotalTargets = len(monitorURLs)

	// For monitor-only workflow, we skip crawling and directly probe the URLs
	var allProbeResults []models.ProbeResult
	var allSecretFindings []models.SecretFinding
	var allURLDiffResults map[string]models.URLDiffResult

	if len(monitorURLs) > 0 {
		// Execute HTTPX probing directly on monitor URLs
		probeResults, err := so.executeHTTPXProbing(ctx, monitorURLs, monitorURLs, monitorURLs[0], scanSessionID)
		if err != nil {
			summaryData.Status = string(models.ScanStatusFailed)
			summaryData.ErrorMessages = []string{fmt.Sprintf("Monitor workflow probing failed: %v", err)}
			return summaryData, nil, nil, nil, err
		}
		allProbeResults = probeResults

		// Execute secret detection
		secretFindings, err := so.executeSecretDetection(ctx, allProbeResults, scanSessionID)
		if err != nil {
			summaryData.Status = string(models.ScanStatusFailed)
			summaryData.ErrorMessages = []string{fmt.Sprintf("Monitor workflow secret detection failed: %v", err)}
			return summaryData, allProbeResults, nil, nil, err
		}
		allSecretFindings = secretFindings

		// Process diffing and storage
		urlDiffResults, err := so.processDiffingAndStorage(ctx, allProbeResults, monitorURLs, monitorURLs[0], scanSessionID)
		if err != nil {
			summaryData.Status = string(models.ScanStatusFailed)
			summaryData.ErrorMessages = []string{fmt.Sprintf("Monitor workflow diffing/storage failed: %v", err)}
			return summaryData, allProbeResults, urlDiffResults, allSecretFindings, err
		}
		allURLDiffResults = urlDiffResults
	}

	scanDuration := time.Since(startTime)
	summaryData.ScanDuration = scanDuration

	// Populate probe stats
	if allProbeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(allProbeResults)
		for _, pr := range allProbeResults {
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summaryData.ProbeStats.SuccessfulProbes++
			} else {
				summaryData.ProbeStats.FailedProbes++
			}
		}
	}

	// Populate diff stats
	if allURLDiffResults != nil {
		for _, diffResult := range allURLDiffResults {
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
		}
	}

	// Calculate secret statistics
	summaryData.SecretStats = notifier.CalculateSecretStats(allSecretFindings)

	summaryData.Status = string(models.ScanStatusCompleted)
	return summaryData, allProbeResults, allURLDiffResults, allSecretFindings, nil
}

// ExecuteCompleteScanWorkflow executes the complete scan workflow and returns summary data
func (so *ScanOrchestrator) ExecuteCompleteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string, targetSource string) (models.ScanSummaryData, []models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, error) {
	startTime := time.Now()

	// Execute the scan workflow
	probeResults, urlDiffResults, secretFindings, workflowErr := so.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
	scanDuration := time.Since(startTime)

	// Build summary data
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = scanSessionID
	summaryData.TargetSource = targetSource
	summaryData.Targets = seedURLs
	summaryData.TotalTargets = len(seedURLs)
	summaryData.ScanDuration = scanDuration

	// Populate probe stats
	if probeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(probeResults)
		for _, pr := range probeResults {
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summaryData.ProbeStats.SuccessfulProbes++
			} else {
				summaryData.ProbeStats.FailedProbes++
			}
		}
	}

	// Populate diff stats
	if urlDiffResults != nil {
		for _, diffResult := range urlDiffResults {
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
		}
	}

	// Calculate secret statistics
	summaryData.SecretStats = notifier.CalculateSecretStats(secretFindings)

	// Handle workflow errors
	if workflowErr != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
		return summaryData, probeResults, urlDiffResults, secretFindings, workflowErr
	}

	summaryData.Status = string(models.ScanStatusCompleted)
	return summaryData, probeResults, urlDiffResults, secretFindings, nil
}

// prepareScanConfiguration prepares crawler configuration for the scan
func (so *ScanOrchestrator) prepareScanConfiguration(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
	// Configure crawler
	crawlerCfg := &so.globalConfig.CrawlerConfig
	// Important: Make a copy or ensure SeedURLs is set fresh for each call
	// to avoid issues if the underlying slice in globalConfig is modified elsewhere
	// or if multiple orchestrators run with modified global configs.
	currentCrawlerCfg := *crawlerCfg // Shallow copy is usually fine for config structs
	currentCrawlerCfg.SeedURLs = seedURLs

	// Auto-add seed hostnames to allowed hostnames if enabled and seeds provided via parameter
	if currentCrawlerCfg.AutoAddSeedHostnames && len(seedURLs) > 0 {
		seedHostnames := crawler.ExtractHostnamesFromSeedURLs(seedURLs, so.logger)
		if len(seedHostnames) > 0 {
			currentCrawlerCfg.Scope.AllowedHostnames = crawler.MergeAllowedHostnames(
				currentCrawlerCfg.Scope.AllowedHostnames,
				seedHostnames,
			)
			so.logger.Info().
				Strs("seed_hostnames", seedHostnames).
				Strs("original_allowed_hostnames", crawlerCfg.Scope.AllowedHostnames).
				Strs("final_allowed_hostnames", currentCrawlerCfg.Scope.AllowedHostnames).
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

	return &currentCrawlerCfg, primaryRootTargetURL, nil
}

// executeCrawler runs the crawler and returns discovered URLs
func (so *ScanOrchestrator) executeCrawler(ctx context.Context, crawlerCfg *config.CrawlerConfig, scanSessionID, primaryRootTargetURL string) ([]string, error) {
	var discoveredURLs []string

	// Run crawler if seed URLs provided
	if len(crawlerCfg.SeedURLs) > 0 {
		// Check for context cancellation before starting crawler
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "crawler start"); cancelled.Cancelled {
			return nil, cancelled.Error
		}

		so.logger.Info().Int("seed_count", len(crawlerCfg.SeedURLs)).Str("session_id", scanSessionID).Str("primary_target", primaryRootTargetURL).Msg("Starting crawler")

		// Create HTTP client for crawler using factory
		httpClientFactory := common.NewHTTPClientFactory(so.logger)
		crawlerClient, err := httpClientFactory.CreateCrawlerClient(
			time.Duration(crawlerCfg.RequestTimeoutSecs)*time.Second,
			"",                      // proxy - crawler config doesn't have proxy field
			make(map[string]string), // customHeaders - crawler config doesn't have custom headers
		)
		if err != nil {
			so.logger.Error().Err(err).Msg("Failed to create HTTP client for crawler")
			return nil, fmt.Errorf("orchestrator: failed to create crawler HTTP client: %w", err)
		}

		crawlerInstance, err := crawler.NewCrawler(crawlerCfg, crawlerClient, so.logger)
		if err != nil {
			so.logger.Error().Err(err).Msg("Failed to initialize crawler")
			return nil, fmt.Errorf("orchestrator: failed to initialize crawler: %w", err)
		}

		crawlerInstance.Start(ctx)
		discoveredURLs = crawlerInstance.GetDiscoveredURLs()
		so.logger.Info().Int("discovered_count", len(discoveredURLs)).Str("session_id", scanSessionID).Msg("Crawler finished")
	} else {
		so.logger.Info().Str("session_id", scanSessionID).Msg("No seed URLs provided, skipping crawler module.")
	}

	return discoveredURLs, nil
}

// executeHTTPXProbing runs HTTPX probing on discovered URLs and returns probe results
func (so *ScanOrchestrator) executeHTTPXProbing(ctx context.Context, discoveredURLs []string, seedURLs []string, primaryRootTargetURL, scanSessionID string) ([]models.ProbeResult, error) {
	var allProbeResultsForCurrentScan []models.ProbeResult

	if len(discoveredURLs) > 0 {
		// Check for context cancellation before starting HTTPX
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing start"); cancelled.Cancelled {
			return nil, cancelled.Error
		}

		so.logger.Info().Int("url_count", len(discoveredURLs)).Str("session_id", scanSessionID).Msg("Starting HTTPX probing")

		runnerCfg := &httpxrunner.Config{
			Targets:              discoveredURLs,
			Method:               so.globalConfig.HttpxRunnerConfig.Method,
			RequestURIs:          so.globalConfig.HttpxRunnerConfig.RequestURIs,
			FollowRedirects:      so.globalConfig.HttpxRunnerConfig.FollowRedirects,
			Timeout:              so.globalConfig.HttpxRunnerConfig.TimeoutSecs,
			Retries:              so.globalConfig.HttpxRunnerConfig.Retries,
			Threads:              so.globalConfig.HttpxRunnerConfig.Threads,
			CustomHeaders:        so.globalConfig.HttpxRunnerConfig.CustomHeaders,
			Proxy:                so.globalConfig.HttpxRunnerConfig.Proxy,
			Verbose:              so.globalConfig.HttpxRunnerConfig.Verbose,
			TechDetect:           so.globalConfig.HttpxRunnerConfig.TechDetect,
			ExtractTitle:         so.globalConfig.HttpxRunnerConfig.ExtractTitle,
			ExtractStatusCode:    so.globalConfig.HttpxRunnerConfig.ExtractStatusCode,
			ExtractLocation:      so.globalConfig.HttpxRunnerConfig.ExtractLocation,
			ExtractContentLength: so.globalConfig.HttpxRunnerConfig.ExtractContentLength,
			ExtractServerHeader:  so.globalConfig.HttpxRunnerConfig.ExtractServerHeader,
			ExtractContentType:   so.globalConfig.HttpxRunnerConfig.ExtractContentType,
			ExtractIPs:           so.globalConfig.HttpxRunnerConfig.ExtractIPs,
			ExtractBody:          so.globalConfig.HttpxRunnerConfig.ExtractBody,
			ExtractHeaders:       so.globalConfig.HttpxRunnerConfig.ExtractHeaders,
		}

		// The primaryRootTargetURL for httpxrunner is mostly for context in its internal logging or potentially file naming if it did that.
		// The actual grouping of results for diffing/parquet will use the more granular rootTargetForThisURL derived from seedURLs.
		probeRunner, err := httpxrunner.NewRunner(runnerCfg, primaryRootTargetURL, so.logger)
		if err != nil {
			so.logger.Error().Err(err).Str("session_id", scanSessionID).Msg("Failed to create HTTPX runner")
			return nil, fmt.Errorf("orchestrator: failed to create HTTPX runner for session %s: %w", scanSessionID, err)
		}

		if err := probeRunner.Run(ctx); err != nil { // Pass context to Run
			so.logger.Warn().Err(err).Str("session_id", scanSessionID).Msg("HTTPX probing encountered errors")
			// Continue processing with any results obtained, unless context was cancelled
			if ctx.Err() == context.Canceled {
				so.logger.Info().Str("session_id", scanSessionID).Msg("HTTPX probing cancelled.")
				return allProbeResultsForCurrentScan, ctx.Err() // Return partial results if any, and context error
			}
		}

		// Check for context cancellation after HTTPX Run
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing completion"); cancelled.Cancelled {
			return allProbeResultsForCurrentScan, cancelled.Error
		}

		probeResultsFromRunner := probeRunner.GetResults()
		resultMap := make(map[string]models.ProbeResult)
		for _, r := range probeResultsFromRunner {
			resultMap[r.InputURL] = r
		}

		// Map results back to discovered URLs and assign RootTargetURL
		for _, urlString := range discoveredURLs {
			rootTargetForThisURL := urlhandler.GetRootTargetForURL(urlString, seedURLs)
			if r, ok := resultMap[urlString]; ok {
				actualResult := r
				actualResult.RootTargetURL = rootTargetForThisURL
				allProbeResultsForCurrentScan = append(allProbeResultsForCurrentScan, actualResult)
			} else {
				so.logger.Warn().Str("url", urlString).Msg("No probe result from httpx for discovered URL, creating error entry.")
				allProbeResultsForCurrentScan = append(allProbeResultsForCurrentScan, models.ProbeResult{
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
	so.logger.Info().Int("count", len(allProbeResultsForCurrentScan)).Str("session_id", scanSessionID).Msg("Processed probe results from current scan")

	return allProbeResultsForCurrentScan, nil
}

// executeSecretDetection runs secret detection on probe results and returns secret findings
func (so *ScanOrchestrator) executeSecretDetection(ctx context.Context, allProbeResultsForCurrentScan []models.ProbeResult, scanSessionID string) ([]models.SecretFinding, error) {
	var allSecretFindings []models.SecretFinding

	if so.secretDetector != nil && so.globalConfig.SecretsConfig.Enabled {
		so.logger.Info().Str("session_id", scanSessionID).Msg("Starting secret detection on probe results")

		for _, pr := range allProbeResultsForCurrentScan {
			var bodyContent []byte

			if len(pr.Body) > 0 {
				bodyContent = []byte(pr.Body)
			} else {
				fetchResult, err := so.fetcher.FetchFileContent(monitor.FetchFileContentInput{
					URL: pr.InputURL,
				})
				if err != nil {
					so.logger.Warn().Err(err).Str("url", pr.InputURL).Msg("Failed to fetch content using Fetcher for secret detection")
					continue
				}

				if fetchResult.HTTPStatusCode == 200 && len(fetchResult.Content) > 0 {
					bodyContent = fetchResult.Content
				} else {
					continue
				}
			}

			secretFindings, err := so.secretDetector.ScanContent(pr.InputURL, bodyContent, pr.ContentType)
			if err != nil {
				so.logger.Warn().Err(err).Str("url", pr.InputURL).Msg("Failed to scan content for secrets")
				continue
			}

			if len(secretFindings) > 0 {
				so.logger.Info().Str("url", pr.InputURL).Int("findings_count", len(secretFindings)).Msg("Found secrets in content")
				allSecretFindings = append(allSecretFindings, secretFindings...)
			}
		}

		so.logger.Info().Int("total_secret_findings", len(allSecretFindings)).Str("session_id", scanSessionID).Msg("Secret detection completed")
	} else {
		so.logger.Debug().Str("session_id", scanSessionID).Msg("Secret detection disabled or not configured")
	}

	return allSecretFindings, nil
}

// processDiffingAndStorage processes URL diffing and stores results to Parquet
func (so *ScanOrchestrator) processDiffingAndStorage(ctx context.Context, allProbeResultsForCurrentScan []models.ProbeResult, seedURLs []string, primaryRootTargetURL, scanSessionID string) (map[string]models.URLDiffResult, error) {
	// Group results by root target
	resultsByRootTarget := make(map[string][]models.ProbeResult)
	// Create a map to store indices of probe results for each root target to update the original slice
	indicesByRootTarget := make(map[string][]int)

	for i, pr := range allProbeResultsForCurrentScan { // Use results from current scan
		rtURL := pr.RootTargetURL
		if rtURL == "" {
			rtURL = primaryRootTargetURL          // Fallback to session's primary root target
			if rtURL == "" && len(seedURLs) > 0 { // Should not happen if seedURLs were present
				rtURL = seedURLs[0]
			} else if rtURL == "" {
				rtURL = pr.InputURL // Absolute fallback
			}
		}
		resultsByRootTarget[rtURL] = append(resultsByRootTarget[rtURL], pr)
		indicesByRootTarget[rtURL] = append(indicesByRootTarget[rtURL], i) // Store index
	}

	// Run URL diffing and store to Parquet
	urlDiffer := differ.NewUrlDiffer(so.parquetReader, so.logger)
	allURLDiffResults := make(map[string]models.URLDiffResult)

	for rootTgt, resultsForRoot := range resultsByRootTarget {
		if rootTgt == "" {
			so.logger.Warn().Str("session_id", scanSessionID).Msg("Skipping diffing/storage for empty root target.")
			continue
		}

		// Check for context cancellation during diffing/storing loop
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "diff/store for target: "+rootTgt); cancelled.Cancelled {
			return allURLDiffResults, cancelled.Error
		}

		so.logger.Info().Str("root_target", rootTgt).Int("current_results_count", len(resultsForRoot)).Str("session_id", scanSessionID).Msg("Processing diff for root target")

		// Create a slice of pointers to models.ProbeResult from resultsForRoot for UrlDiffer
		// These pointers will point to copies, but we'll use the returned DiffedURL.ProbeResult to update the original allProbeResultsForCurrentScan
		currentScanProbesPtr := make([]*models.ProbeResult, len(resultsForRoot))
		for i := range resultsForRoot {
			// Create a temporary copy for UrlDiffer to modify.
			// This is safer if UrlDiffer modifies more than just URLStatus in the future.
			tempCopy := resultsForRoot[i]
			currentScanProbesPtr[i] = &tempCopy
		}

		diffResult, err := urlDiffer.Compare(currentScanProbesPtr, rootTgt)
		if err != nil {
			so.logger.Error().Err(err).Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("Failed to compare URLs. Skipping storage and diff summary for this target.")
			continue
		}

		if diffResult == nil {
			so.logger.Warn().Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("DiffResult was nil, though no explicit error. Skipping further processing for this target.")
			continue
		}

		allURLDiffResults[rootTgt] = *diffResult
		so.logger.Info().Str("root_target", rootTgt).Str("session_id", scanSessionID).Int("new", diffResult.New).Int("old", diffResult.Old).Int("existing", diffResult.Existing).Int("total_diff_urls", len(diffResult.Results)).Msg("URL Diffing complete for target.")

		// Update the original allProbeResultsForCurrentScan with the URLStatus and any other modifications from diffResult
		// Create a map of InputURL to updated ProbeResult from diffResult for efficient lookup
		updatedProbesMap := make(map[string]models.ProbeResult)
		for _, diffedURL := range diffResult.Results {
			updatedProbesMap[diffedURL.ProbeResult.InputURL] = diffedURL.ProbeResult
		}

		// Iterate through the original indices for this rootTgt and update allProbeResultsForCurrentScan
		for _, originalIndex := range indicesByRootTarget[rootTgt] {
			originalProbe := &allProbeResultsForCurrentScan[originalIndex] // Get a pointer to the original item
			if updatedProbe, ok := updatedProbesMap[originalProbe.InputURL]; ok {
				// Preserve fields that UrlDiffer doesn't touch, then update with fields UrlDiffer might have set (like URLStatus)
				// This ensures we don't lose data from the initial httpx scan if UrlDiffer only returns a subset of fields.
				// A safer approach is to copy only specific fields that UrlDiffer is known to modify.
				originalProbe.URLStatus = updatedProbe.URLStatus
				originalProbe.OldestScanTimestamp = updatedProbe.OldestScanTimestamp // Ensure this is also carried over if set by differ
				// Copy any other relevant fields that might have been updated by the differ.
			}
		}

		probesToStoreThisTarget := make([]models.ProbeResult, 0, len(diffResult.Results))
		for _, diffedURL := range diffResult.Results {
			probesToStoreThisTarget = append(probesToStoreThisTarget, diffedURL.ProbeResult)
		}

		// Write to Parquet for the current rootTgt
		if so.parquetWriter != nil {
			if len(probesToStoreThisTarget) > 0 {
				so.logger.Info().Int("count", len(probesToStoreThisTarget)).Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("Writing probe results to Parquet...")
				if err := so.parquetWriter.Write(ctx, probesToStoreThisTarget, scanSessionID, rootTgt); err != nil {
					so.logger.Error().Err(err).Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("Failed to write Parquet data")
					// Decide if this error should cause the entire workflow to fail or just log and continue for this target
					// If context was cancelled, the error will be ctx.Err() and should be propagated.
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return allURLDiffResults, err // Propagate context error
					}
				}
			} else {
				so.logger.Info().Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("No probe results to store to Parquet for target.")
			}
		} else {
			so.logger.Info().Str("root_target", rootTgt).Str("session_id", scanSessionID).Msg("ParquetWriter is not initialized. Skipping Parquet storage for target.")
		}
	} // End loop over resultsByRootTarget

	return allURLDiffResults, nil
}
