package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// executeHTTPXProbing orchestrates the HTTPX probing process on discovered URLs.
// It validates inputs, runs the probing, and processes results.
func (s *Scanner) executeHTTPXProbing(ctx context.Context, input HTTPXProbingInput) ([]models.ProbeResult, error) {
	if !s.shouldRunHTTPXProbing(input) {
		s.logger.Info().Str("session_id", input.ScanSessionID).Msg("No URLs discovered, skipping HTTPX probing")
		return nil, nil
	}

	if err := s.checkContextCancellation(ctx, "HTTPX probing start"); err != nil {
		return nil, err
	}

	s.logHTTPXProbingStart(input)

	runnerResults, err := s.runHTTPXRunner(ctx, input.HttpxRunnerConfig, input.PrimaryRootTargetURL, input.ScanSessionID)
	if err != nil {
		return s.handleHTTPXError(ctx, runnerResults, input, err)
	}

	if err := s.checkContextCancellation(ctx, "HTTPX probing completion"); err != nil {
		// Process partial results before returning cancellation error
		finalResults := s.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
		return finalResults, err
	}

	finalResults := s.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
	s.logHTTPXProbingCompletion(input.ScanSessionID, len(finalResults))

	return finalResults, nil
}

// shouldRunHTTPXProbing determines if HTTPX probing should be executed.
func (s *Scanner) shouldRunHTTPXProbing(input HTTPXProbingInput) bool {
	return len(input.DiscoveredURLs) > 0
}

// checkContextCancellation checks if the context has been cancelled and logs appropriately.
func (s *Scanner) checkContextCancellation(ctx context.Context, operation string) error {
	if cancelled := common.CheckCancellationWithLog(ctx, s.logger, operation); cancelled.Cancelled {
		return cancelled.Error
	}
	return nil
}

// logHTTPXProbingStart logs the start of HTTPX probing.
func (s *Scanner) logHTTPXProbingStart(input HTTPXProbingInput) {
	s.logger.Info().
		Int("url_count", len(input.DiscoveredURLs)).
		Str("session_id", input.ScanSessionID).
		Msg("Starting HTTPX probing")
}

// logHTTPXProbingCompletion logs the completion of HTTPX probing.
func (s *Scanner) logHTTPXProbingCompletion(sessionID string, resultCount int) {
	s.logger.Info().
		Int("count", resultCount).
		Str("session_id", sessionID).
		Msg("Processed probe results from current scan")
}

// runHTTPXRunner creates and executes the HTTPX runner.
func (s *Scanner) runHTTPXRunner(ctx context.Context, runnerConfig *httpxrunner.Config, primaryRootTargetURL string, scanSessionID string) ([]models.ProbeResult, error) {
	runner, err := s.createHTTPXRunner(runnerConfig, primaryRootTargetURL, scanSessionID)
	if err != nil {
		return nil, err
	}

	if err := runner.Run(ctx); err != nil {
		return s.handleRunnerError(ctx, runner, scanSessionID, err)
	}

	return runner.GetResults(), nil
}

// createHTTPXRunner creates a new HTTPX runner instance.
func (s *Scanner) createHTTPXRunner(runnerConfig *httpxrunner.Config, primaryRootTargetURL string, scanSessionID string) (*httpxrunner.Runner, error) {
	runner, err := httpxrunner.NewRunner(runnerConfig, primaryRootTargetURL, s.logger)
	if err != nil {
		s.logger.Error().Err(err).Str("session_id", scanSessionID).Msg("Failed to create HTTPX runner")
		return nil, common.WrapError(err, fmt.Sprintf("failed to create HTTPX runner for session %s", scanSessionID))
	}
	return runner, nil
}

// handleRunnerError handles errors that occur during runner execution.
func (s *Scanner) handleRunnerError(ctx context.Context, runner *httpxrunner.Runner, scanSessionID string, err error) ([]models.ProbeResult, error) {
	s.logger.Warn().Err(err).Str("session_id", scanSessionID).Msg("HTTPX probing encountered errors")

	if ctx.Err() == context.Canceled {
		s.logger.Info().Str("session_id", scanSessionID).Msg("HTTPX probing cancelled")
		return runner.GetResults(), ctx.Err()
	}

	return runner.GetResults(), common.WrapError(err, fmt.Sprintf("httpx runner execution failed for session %s", scanSessionID))
}

// handleHTTPXError handles errors that occur during HTTPX probing process.
func (s *Scanner) handleHTTPXError(ctx context.Context, runnerResults []models.ProbeResult, input HTTPXProbingInput, err error) ([]models.ProbeResult, error) {
	if ctx.Err() == context.Canceled {
		s.logger.Info().Str("session_id", input.ScanSessionID).Msg("HTTPX probing cancelled during runner execution")
		finalResults := s.processHTTPXResults(runnerResults, input.DiscoveredURLs, input.SeedURLs, input.ScanSessionID)
		return finalResults, ctx.Err()
	}
	return nil, err
}

// processHTTPXResults maps raw httpx results to models.ProbeResult and assigns RootTargetURL.
func (s *Scanner) processHTTPXResults(
	runnerResults []models.ProbeResult,
	discoveredURLs []string,
	seedURLs []string,
	scanSessionID string,
) []models.ProbeResult {
	processedResults := make([]models.ProbeResult, 0, len(discoveredURLs))
	probeResultMap := s.createProbeResultMap(runnerResults)

	for _, urlString := range discoveredURLs {
		probeResult := s.createProbeResultForURL(urlString, probeResultMap, seedURLs, scanSessionID)
		processedResults = append(processedResults, probeResult)
	}

	return processedResults
}

// createProbeResultMap creates a map of URL to ProbeResult for fast lookup.
func (s *Scanner) createProbeResultMap(runnerResults []models.ProbeResult) map[string]models.ProbeResult {
	probeResultMap := make(map[string]models.ProbeResult, len(runnerResults))
	for _, result := range runnerResults {
		probeResultMap[result.InputURL] = result
	}
	return probeResultMap
}

// createProbeResultForURL creates a ProbeResult for a specific URL.
func (s *Scanner) createProbeResultForURL(
	urlString string,
	probeResultMap map[string]models.ProbeResult,
	seedURLs []string,
	scanSessionID string,
) models.ProbeResult {
	rootTargetURL := urlhandler.GetRootTargetForURL(urlString, seedURLs)

	if probeResult, exists := probeResultMap[urlString]; exists {
		probeResult.RootTargetURL = rootTargetURL
		return probeResult
	}

	// Create error entry for URLs without probe results
	s.logMissingProbeResult(urlString, scanSessionID)
	return s.createErrorProbeResult(urlString, rootTargetURL)
}

// logMissingProbeResult logs when no probe result is found for a URL.
func (s *Scanner) logMissingProbeResult(urlString, scanSessionID string) {
	s.logger.Warn().
		Str("url", urlString).
		Str("session_id", scanSessionID).
		Msg("No probe result from httpx for discovered URL, creating error entry")
}

// createErrorProbeResult creates an error ProbeResult for URLs that weren't probed.
func (s *Scanner) createErrorProbeResult(urlString, rootTargetURL string) models.ProbeResult {
	return models.ProbeResult{
		InputURL:      urlString,
		Error:         "No response or error during httpx probe",
		Timestamp:     time.Now(),
		RootTargetURL: rootTargetURL,
	}
}
