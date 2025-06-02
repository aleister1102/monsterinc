package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// HTTPXProbingInput holds the parameters for executeHTTPXProbing.
// This helps in reducing the number of direct parameters to the function.
// It also makes it easier to add or modify parameters in the future.
// Using a struct for parameters promotes better code organization and readability.
// According to the refactoring guidelines (refactor.mdc), functions should have few parameters.
// This struct helps achieve that by grouping related parameters together.
// It also aligns with task 1.3 (Reduce the number of parameters for functions; use structs for complex parameter groups).
type HTTPXProbingInput struct {
	DiscoveredURLs       []string
	SeedURLs             []string
	PrimaryRootTargetURL string
	ScanSessionID        string
	HttpxRunnerConfig    *httpxrunner.Config // This will be derived from globalConfig.HttpxRunnerConfig
}

// runHTTPXRunner creates and runs the httpx runner.
// It encapsulates the logic for setting up and executing the httpx tool.
func (so *Orchestrator) runHTTPXRunner(ctx context.Context, runnerConfig *httpxrunner.Config, primaryRootTargetURL string, scanSessionID string) ([]models.ProbeResult, error) {
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
			// Return partial results if any, and context error.
			// The caller (executeHTTPXProbing) will decide how to handle partial results.
			return runner.GetResults(), ctx.Err()
		}
		// If runner.Run failed with an error other than context cancellation,
		// return this error to the caller.
		return runner.GetResults(), fmt.Errorf("httpx runner.Run failed for session %s: %w", scanSessionID, err)
	}
	return runner.GetResults(), nil
}

// processHTTPXResults maps the raw httpx results to models.ProbeResult and assigns RootTargetURL.
// It handles cases where no probe result is found for a discovered URL.
func (so *Orchestrator) processHTTPXResults(
	runnerResults []models.ProbeResult,
	discoveredURLs []string,
	seedURLs []string,
	scanSessionID string,
) []models.ProbeResult {
	processedResults := make([]models.ProbeResult, 0, len(discoveredURLs))
	probeResultMap := make(map[string]models.ProbeResult)
	for _, r := range runnerResults {
		probeResultMap[r.InputURL] = r
	}

	for _, urlString := range discoveredURLs {
		rootTargetForThisURL := urlhandler.GetRootTargetForURL(urlString, seedURLs)
		if r, ok := probeResultMap[urlString]; ok {
			probeResult := r
			probeResult.RootTargetURL = rootTargetForThisURL
			processedResults = append(processedResults, probeResult)
		} else {
			so.logger.Warn().Str("url", urlString).Str("session_id", scanSessionID).Msg("No probe result from httpx for discovered URL, creating error entry.")
			processedResults = append(processedResults, models.ProbeResult{
				InputURL:      urlString,
				Error:         "No response or error during httpx probe",
				Timestamp:     time.Now(),
				RootTargetURL: rootTargetForThisURL,
			})
		}
	}
	return processedResults
}

// executeHTTPXProbing runs HTTPX probing on discovered URLs and returns probe results
func (so *Orchestrator) executeHTTPXProbing(ctx context.Context, input HTTPXProbingInput) ([]models.ProbeResult, error) {
	if len(input.DiscoveredURLs) == 0 {
		so.logger.Info().Str("session_id", input.ScanSessionID).Msg("No URLs discovered by crawler or crawler skipped, skipping HTTPX probing.")
		return nil, nil
	}

	// Check for context cancellation before starting HTTPX
	if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing start"); cancelled.Cancelled {
		return nil, cancelled.Error
	}

	so.logger.Info().Int("url_count", len(input.DiscoveredURLs)).Str("session_id", input.ScanSessionID).Msg("Starting HTTPX probing")

	runnerResults, err := so.runHTTPXRunner(ctx, input.HttpxRunnerConfig, input.PrimaryRootTargetURL, input.ScanSessionID)
	// err can be context.Canceled, a specific httpx runner error, or nil.
	// runnerResults might contain partial data if err is context.Canceled.

	// Check for context cancellation immediately after runHTTPXRunner if it returned an error
	// This handles the case where runHTTPXRunner itself was cancelled.
	if err != nil && ctx.Err() == context.Canceled {
		// If context was cancelled during runHTTPXRunner, process whatever partial results we might have.
		// This logging might be redundant if runHTTPXRunner already logged it.
		so.logger.Info().Str("session_id", input.ScanSessionID).Msg("HTTPX probing cancelled during runner execution.")
		// Process partial results if any, then return with the context error
		finalProbeResults := so.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
		return finalProbeResults, ctx.Err()
	} else if err != nil {
		// For other errors from runHTTPXRunner (not context cancellation)
		// We might still have partial runnerResults, decide if they should be processed or discarded.
		// For now, let's assume we return the error and any partial results are not processed further by processHTTPXResults.
		// The caller of executeHTTPXProbing will then receive this error.
		return nil, err // Or potentially process runnerResults if they are meaningful despite the error
	}

	// Check for context cancellation after HTTPX Run (if no error or non-cancellation error occurred in runHTTPXRunner)
	if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "HTTPX probing completion"); cancelled.Cancelled {
		// Process results obtained so far before returning the cancellation error
		finalProbeResults := so.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
		return finalProbeResults, cancelled.Error
	}

	finalProbeResults := so.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
	so.logger.Info().Int("count", len(finalProbeResults)).Str("session_id", input.ScanSessionID).Msg("Processed probe results from current scan")

	return finalProbeResults, nil
}
