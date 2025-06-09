package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// HTTPXExecutor handles the execution of the HTTPX probing component
// Separates HTTPX execution logic from the main scanner
type HTTPXExecutor struct {
	logger          zerolog.Logger
	crawlerInstance *crawler.Crawler
	progressDisplay *common.ProgressDisplayManager
	httpxManager    *HTTPXManager // Added httpx manager for singleton instance
}

// NewHTTPXExecutor creates a new HTTPX executor
func NewHTTPXExecutor(logger zerolog.Logger) *HTTPXExecutor {
	return &HTTPXExecutor{
		logger:       logger.With().Str("module", "HTTPXExecutor").Logger(),
		httpxManager: NewHTTPXManager(logger),
	}
}

// SetCrawlerInstance sets the crawler instance for root target tracking
func (he *HTTPXExecutor) SetCrawlerInstance(crawlerInstance *crawler.Crawler) {
	he.crawlerInstance = crawlerInstance
}

// SetProgressDisplay sets the progress display manager
func (he *HTTPXExecutor) SetProgressDisplay(progressDisplay *common.ProgressDisplayManager) {
	he.progressDisplay = progressDisplay
}

// Shutdown gracefully shuts down the httpx executor and its managed components
func (he *HTTPXExecutor) Shutdown() {
	he.logger.Info().Msg("Shutting down HTTPX executor")

	if he.httpxManager != nil {
		he.httpxManager.Shutdown()
	}

	he.logger.Info().Msg("HTTPX executor shutdown complete")
}

// HTTPXExecutionInput holds the parameters for HTTPX probing execution
// Reduces function parameter count according to refactor principles
type HTTPXExecutionInput struct {
	Context              context.Context
	DiscoveredURLs       []string
	SeedURLs             []string
	PrimaryRootTargetURL string
	ScanSessionID        string
	HttpxRunnerConfig    *httpxrunner.Config
}

// HTTPXExecutionResult contains the results from HTTPX execution
type HTTPXExecutionResult struct {
	ProbeResults []models.ProbeResult
	Error        error
}

// Execute runs HTTPX probing on discovered URLs and returns probe results
// Renamed from executeHTTPXProbing for clarity and moved to dedicated executor
func (he *HTTPXExecutor) Execute(input HTTPXExecutionInput) *HTTPXExecutionResult {
	result := &HTTPXExecutionResult{}

	if len(input.DiscoveredURLs) == 0 {
		he.logger.Info().Str("session_id", input.ScanSessionID).Msg("No URLs for probing, skipping HTTPX")
		return result
	}

	// Check context cancellation early
	if cancelled := common.CheckCancellation(input.Context); cancelled.Cancelled {
		he.logger.Info().Str("session_id", input.ScanSessionID).Msg("Context cancelled before HTTPX execution")
		result.Error = cancelled.Error
		return result
	}

	he.logger.Info().Int("url_count", len(input.DiscoveredURLs)).Str("session_id", input.ScanSessionID).Msg("Starting HTTPX probing")

	// Update progress - starting probing
	if he.progressDisplay != nil {
		he.progressDisplay.UpdateScanProgress(2, 5, "Probing", fmt.Sprintf("Starting HTTPX probing of %d URLs\n", len(input.DiscoveredURLs)))
	}

	runnerResults, err := he.runHTTPXRunner(input.Context, input.HttpxRunnerConfig, input.PrimaryRootTargetURL, input.ScanSessionID)

	// Handle context cancellation during execution - immediate response
	if err != nil && (input.Context.Err() == context.Canceled || input.Context.Err() == context.DeadlineExceeded) {
		he.logger.Info().Str("session_id", input.ScanSessionID).Msg("HTTPX probing cancelled immediately")

		// Update progress - cancelled
		if he.progressDisplay != nil {
			he.progressDisplay.UpdateScanProgress(2, 5, "Cancelled", "HTTPX probing cancelled")
		}

		result.ProbeResults = he.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs)
		result.Error = input.Context.Err()
		return result
	} else if err != nil {
		he.logger.Error().Err(err).Str("session_id", input.ScanSessionID).Msg("HTTPX probing failed")

		// Update progress - failed
		if he.progressDisplay != nil {
			he.progressDisplay.UpdateScanProgress(2, 5, "Failed", fmt.Sprintf("HTTPX probing failed: %v", err))
		}

		result.Error = err
		return result
	}

	// Final context check after completion
	if cancelled := common.CheckCancellation(input.Context); cancelled.Cancelled {
		he.logger.Info().Str("session_id", input.ScanSessionID).Msg("Context cancelled after HTTPX completion")

		// Update progress - cancelled
		if he.progressDisplay != nil {
			he.progressDisplay.UpdateScanProgress(2, 5, "Cancelled", "HTTPX probing cancelled after completion")
		}

		result.ProbeResults = he.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs)
		result.Error = cancelled.Error
		return result
	}

	result.ProbeResults = he.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs)

	// Update progress - completed
	if he.progressDisplay != nil {
		he.progressDisplay.UpdateScanProgress(2, 5, "Probing Complete", fmt.Sprintf("HTTPX probing completed: %d results", len(result.ProbeResults)))
	}

	he.logger.Info().Int("count", len(result.ProbeResults)).Str("session_id", input.ScanSessionID).Msg("HTTPX probing completed successfully")

	return result
}

// runHTTPXRunner uses the managed httpx runner instead of creating new instances
func (he *HTTPXExecutor) runHTTPXRunner(ctx context.Context, runnerConfig *httpxrunner.Config, primaryRootTargetURL, scanSessionID string) ([]models.ProbeResult, error) {
	return he.httpxManager.ExecuteRunnerBatch(ctx, runnerConfig, primaryRootTargetURL, scanSessionID)
}

// processHTTPXResults maps the raw httpx results to models.ProbeResult and assigns RootTargetURL
// Handles cases where no probe result is found for a discovered URL
func (he *HTTPXExecutor) processHTTPXResults(
	runnerResults []models.ProbeResult,
	discoveredURLs []string,
	seedURLs []string,
) []models.ProbeResult {
	// Pre-allocate slice with exact capacity
	processedResults := make([]models.ProbeResult, 0, len(discoveredURLs))

	// Create map for O(1) lookup instead of nested loops
	probeResultMap := make(map[string]models.ProbeResult, len(runnerResults))
	for _, r := range runnerResults {
		probeResultMap[r.InputURL] = r
	}

	for _, urlString := range discoveredURLs {
		var rootTargetForThisURL string

		// Use crawler instance to get root target if available
		if he.crawlerInstance != nil {
			rootTargetForThisURL = he.crawlerInstance.GetRootTargetForDiscoveredURL(urlString)
		} else {
			// Fallback to urlhandler logic
			rootTargetForThisURL = urlhandler.GetRootTargetForURL(urlString, seedURLs)
		}

		if r, exists := probeResultMap[urlString]; exists {
			r.RootTargetURL = rootTargetForThisURL
			processedResults = append(processedResults, r)
		} else {
			// Create error entry for missing probe result
			processedResults = append(processedResults, models.ProbeResult{
				InputURL:      urlString,
				Error:         "No response from httpx probe",
				Timestamp:     time.Now(),
				RootTargetURL: rootTargetForThisURL,
			})
		}
	}

	return processedResults
}
