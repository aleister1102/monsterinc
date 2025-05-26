package orchestrator

import (
	"context"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"monsterinc/internal/datastore"
	"monsterinc/internal/differ"
	"monsterinc/internal/httpxrunner"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"time"

	"github.com/rs/zerolog"
)

// ScanOrchestrator handles the core logic of a scan workflow.
type ScanOrchestrator struct {
	globalConfig  *config.GlobalConfig
	logger        zerolog.Logger
	parquetReader *datastore.ParquetReader
	parquetWriter *datastore.ParquetWriter
	// latestProbeResults map[string][]models.ProbeResult // Potentially for caching latest results per target
}

// NewScanOrchestrator creates a new ScanOrchestrator.
func NewScanOrchestrator(
	cfg *config.GlobalConfig,
	logger zerolog.Logger,
	pqReader *datastore.ParquetReader,
	pqWriter *datastore.ParquetWriter,
) *ScanOrchestrator {
	return &ScanOrchestrator{
		globalConfig:  cfg,
		logger:        logger,
		parquetReader: pqReader,
		parquetWriter: pqWriter,
		// latestProbeResults: make(map[string][]models.ProbeResult),
	}
}

// ExecuteScanWorkflow runs the full crawl -> probe -> diff -> store workflow.
// seedURLs are the initial URLs to start crawling from.
// scanSessionID is a unique identifier for this specific scan run, used for Parquet naming.
// ctx is used for cancellation of long-running operations.
func (so *ScanOrchestrator) ExecuteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {
	// Configure crawler
	crawlerCfg := &so.globalConfig.CrawlerConfig
	// Important: Make a copy or ensure SeedURLs is set fresh for each call
	// to avoid issues if the underlying slice in globalConfig is modified elsewhere
	// or if multiple orchestrators run with modified global configs.
	currentCrawlerCfg := *crawlerCfg // Shallow copy is usually fine for config structs
	currentCrawlerCfg.SeedURLs = seedURLs

	var discoveredURLs []string
	var primaryRootTargetURL string

	if len(seedURLs) > 0 {
		primaryRootTargetURL = seedURLs[0] // Assuming the first seed is the primary target for this session
	} else {
		// Fallback if no seeds, though ideally, seedURLs should not be empty for a meaningful scan.
		primaryRootTargetURL = "unknown_target_" + scanSessionID
	}

	// Run crawler if seed URLs provided
	if len(currentCrawlerCfg.SeedURLs) > 0 {
		// Check for context cancellation before starting crawler
		select {
		case <-ctx.Done():
			so.logger.Info().Msgf("[INFO] Orchestrator: Context cancelled before crawler start for session %s.", scanSessionID)
			return nil, nil, ctx.Err()
		default:
		}

		so.logger.Info().Msgf("[INFO] Orchestrator: Starting crawler with %d seed URLs for session %s. Primary target: %s", len(currentCrawlerCfg.SeedURLs), scanSessionID, primaryRootTargetURL)
		crawlerInstance, err := crawler.NewCrawler(&currentCrawlerCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("orchestrator: failed to initialize crawler: %w", err)
		}

		crawlerInstance.Start()
		discoveredURLs = crawlerInstance.GetDiscoveredURLs()
		so.logger.Info().Msgf("[INFO] Orchestrator: Crawler discovered %d URLs for session %s", len(discoveredURLs), scanSessionID)
	} else {
		so.logger.Info().Msgf("[INFO] Orchestrator: No seed URLs provided for crawler in session %s. Skipping crawler module.", scanSessionID)
	}

	// Run HTTPX probing
	var allProbeResultsForCurrentScan []models.ProbeResult // Renamed for clarity from allProbeResults

	if len(discoveredURLs) > 0 {
		// Check for context cancellation before starting HTTPX
		select {
		case <-ctx.Done():
			so.logger.Info().Msgf("[INFO] Orchestrator: Context cancelled before HTTPX probing for session %s.", scanSessionID)
			return nil, nil, ctx.Err()
		default:
		}

		so.logger.Info().Msgf("[INFO] Orchestrator: Starting HTTPX probing for %d URLs for session %s...", len(discoveredURLs), scanSessionID)

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
		probeRunner, err := httpxrunner.NewRunner(runnerCfg, primaryRootTargetURL)
		if err != nil {
			return nil, nil, fmt.Errorf("orchestrator: failed to create HTTPX runner for session %s: %w", scanSessionID, err)
		}

		if err := probeRunner.Run(ctx); err != nil { // Pass context to Run
			so.logger.Warn().Msgf("[WARN] Orchestrator: HTTPX probing encountered errors for session %s: %v", scanSessionID, err)
			// Continue processing with any results obtained, unless context was cancelled
			if ctx.Err() == context.Canceled {
				so.logger.Info().Msgf("[INFO] Orchestrator: HTTPX probing cancelled for session %s.", scanSessionID)
				return allProbeResultsForCurrentScan, nil, ctx.Err() // Return partial results if any, and context error
			}
		}

		// Check for context cancellation after HTTPX Run
		select {
		case <-ctx.Done():
			so.logger.Info().Msgf("[INFO] Orchestrator: Context cancelled after HTTPX probing for session %s.", scanSessionID)
			return allProbeResultsForCurrentScan, nil, ctx.Err() // Return partial results and context error
		default:
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
				allProbeResultsForCurrentScan = append(allProbeResultsForCurrentScan, models.ProbeResult{
					InputURL:      urlString,
					Error:         "No response or error during httpx probe",
					Timestamp:     time.Now(),
					RootTargetURL: rootTargetForThisURL,
				})
			}
		}
	} else {
		so.logger.Info().Msgf("[INFO] Orchestrator: No URLs discovered by crawler (or crawler skipped) for session %s. Skipping HTTPX probing.", scanSessionID)
	}
	so.logger.Info().Msgf("[INFO] Orchestrator: Processed %d probe results from current scan for session %s", len(allProbeResultsForCurrentScan), scanSessionID)

	// Group results by root target
	resultsByRootTarget := make(map[string][]models.ProbeResult)
	for _, pr := range allProbeResultsForCurrentScan { // Use results from current scan
		rtURL := pr.RootTargetURL
		if rtURL == "" {
			rtURL = primaryRootTargetURL          // Fallback to session's primary root target
			if rtURL == "" && len(seedURLs) > 0 { // Should not happen if seedURLs were present
				rtURL = seedURLs[0]
			} else if rtURL == "" {
				rtURL = pr.InputURL // Absolute fallback
			}
			so.logger.Debug().Msgf("[DEBUG] Orchestrator: Empty RootTargetURL for %s, falling back to %s", pr.InputURL, rtURL)
		}
		resultsByRootTarget[rtURL] = append(resultsByRootTarget[rtURL], pr)
	}

	// Run URL diffing and store to Parquet
	urlDiffer := differ.NewUrlDiffer(so.parquetReader, so.logger)
	allURLDiffResults := make(map[string]models.URLDiffResult)

	for rootTgt, resultsForRoot := range resultsByRootTarget {
		if rootTgt == "" {
			so.logger.Warn().Msgf("[WARN] Orchestrator: Skipping diffing/storage for empty root target. Session: %s", scanSessionID)
			continue
		}

		// Check for context cancellation during diffing/storing loop
		select {
		case <-ctx.Done():
			so.logger.Info().Msgf("[INFO] Orchestrator: Context cancelled during diff/store for target %s, session %s.", rootTgt, scanSessionID)
			// Return what has been processed so far along with the cancellation error
			return allProbeResultsForCurrentScan, allURLDiffResults, ctx.Err()
		default:
		}

		// Even if resultsForRoot is empty, we might have historical data, so we still need to run the differ
		// to identify 'old' URLs for this rootTgt.
		so.logger.Info().Msgf("[INFO] Orchestrator: Processing diff for root target: %s (current results: %d) for session %s", rootTgt, len(resultsForRoot), scanSessionID)

		// Create a slice of pointers to models.ProbeResult for UrlDiffer to modify status and timestamps
		currentScanProbesPtr := make([]*models.ProbeResult, len(resultsForRoot))
		for i := range resultsForRoot {
			currentScanProbesPtr[i] = &resultsForRoot[i]
		}

		diffResult, err := urlDiffer.Compare(currentScanProbesPtr, rootTgt)
		if err != nil {
			so.logger.Error().Msgf("[ERROR] Orchestrator: Failed to compare URLs for %s in session %s: %v. Skipping storage and diff summary for this target.", rootTgt, scanSessionID, err)
			continue
		}

		if diffResult == nil {
			so.logger.Warn().Msgf("[WARN] Orchestrator: DiffResult was nil for %s in session %s, though no explicit error. Skipping further processing for this target.", rootTgt, scanSessionID)
			continue
		}

		allURLDiffResults[rootTgt] = *diffResult
		so.logger.Info().Msgf("[INFO] Orchestrator: URL Diffing complete for %s in session %s. New: %d, Old: %d, Existing: %d. Total unique URLs in diff: %d",
			rootTgt, scanSessionID, diffResult.New, diffResult.Old, diffResult.Existing, len(diffResult.Results))

		probesToStoreThisTarget := make([]models.ProbeResult, 0, len(diffResult.Results))

		for _, diffedURL := range diffResult.Results {
			probesToStoreThisTarget = append(probesToStoreThisTarget, diffedURL.ProbeResult)
		}

		// Write to Parquet for the current rootTgt
		if so.parquetWriter != nil {
			if len(probesToStoreThisTarget) > 0 {
				so.logger.Info().Msgf("[INFO] Orchestrator: Writing %d probe results for target '%s' (session '%s') to Parquet...", len(probesToStoreThisTarget), rootTgt, scanSessionID)
				if err := so.parquetWriter.Write(probesToStoreThisTarget, scanSessionID, rootTgt); err != nil {
					so.logger.Error().Msgf("[ERROR] Orchestrator: Failed to write Parquet data for root target '%s' in session '%s': %v", rootTgt, scanSessionID, err)
				}
			} else {
				so.logger.Info().Msgf("[INFO] Orchestrator: No probe results to store to Parquet for target '%s' in session '%s'.", rootTgt, scanSessionID)
			}
		} else {
			so.logger.Info().Msgf("[INFO] Orchestrator: ParquetWriter is not initialized. Skipping Parquet storage for target '%s' in session '%s'.", rootTgt, scanSessionID)
		}
	} // End loop over resultsByRootTarget

	// Return all *original* probe results from this scan, and the diff results map
	return allProbeResultsForCurrentScan, allURLDiffResults, nil
}

// CountStatuses is a helper to count URL statuses from a diff result.
// func CountStatuses(diffResult *models.URLDiffResult, status models.URLStatus) int { // Removed: Moved to models.URLDiffResult method
// 	if diffResult == nil {
// 		return 0
// 	}
// 	count := 0
// 	for _, r := range diffResult.Results { // Iterate over .Results (slice)
// 		// Access URLStatus from the embedded ProbeResult
// 		if r.ProbeResult.URLStatus == string(status) { // Compare with string representation of the target status
// 			count++
// 		}
// 	}
// 	return count
// }
