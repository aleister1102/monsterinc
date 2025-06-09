package scanner

import (
	"context"
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/rs/zerolog"
)

// CrawlerExecutor handles the execution of the crawler component
// Separates crawler execution logic from the main scanner
type CrawlerExecutor struct {
	logger          zerolog.Logger
	progressDisplay *common.ProgressDisplayManager
	crawlerManager  *CrawlerManager // Added crawler manager for singleton instance
}

// NewCrawlerExecutor creates a new crawler executor
func NewCrawlerExecutor(logger zerolog.Logger) *CrawlerExecutor {
	return &CrawlerExecutor{
		logger:         logger.With().Str("module", "CrawlerExecutor").Logger(),
		crawlerManager: NewCrawlerManager(logger), // Initialize crawler manager
	}
}

// SetProgressDisplay sets the progress display manager
func (ce *CrawlerExecutor) SetProgressDisplay(progressDisplay *common.ProgressDisplayManager) {
	ce.progressDisplay = progressDisplay
}

// CrawlerExecutionInput contains parameters for crawler execution
// Reduces function parameter count according to refactor principles
type CrawlerExecutionInput struct {
	Context              context.Context
	CrawlerConfig        *config.CrawlerConfig
	ScanSessionID        string
	PrimaryRootTargetURL string
}

// CrawlerExecutionResult contains the results from crawler execution
type CrawlerExecutionResult struct {
	DiscoveredURLs  []string
	CrawlerInstance *crawler.Crawler
	Error           error
}

// Execute runs the crawler using managed singleton instance and returns discovered URLs
func (ce *CrawlerExecutor) Execute(input CrawlerExecutionInput) *CrawlerExecutionResult {
	result := &CrawlerExecutionResult{}

	// Run crawler if seed URLs provided
	if len(input.CrawlerConfig.SeedURLs) == 0 {
		ce.logger.Info().Str("session_id", input.ScanSessionID).Msg("No seed URLs provided, skipping crawler")
		return result
	}

	// Check for context cancellation before starting crawler
	if cancelled := common.CheckCancellation(input.Context); cancelled.Cancelled {
		result.Error = cancelled.Error
		return result
	}

	// Update progress - starting crawler
	if ce.progressDisplay != nil {
		ce.progressDisplay.UpdateWorkflowProgress(1, 5, "Crawler", fmt.Sprintf("Starting crawler with %d seed URLs", len(input.CrawlerConfig.SeedURLs)))
	}

	ce.logger.Info().
		Int("seed_count", len(input.CrawlerConfig.SeedURLs)).
		Str("session_id", input.ScanSessionID).
		Str("primary_target", input.PrimaryRootTargetURL).
		Msg("Starting crawler using managed instance")

	// Note: Auto-calibrate remains enabled for discovered URLs during crawling
	// Only seed URLs have been preprocessed at Scanner level

	// Execute crawler batch using managed instance
	batchResult, err := ce.crawlerManager.ExecuteCrawlerBatch(
		input.Context,
		input.CrawlerConfig,
		input.CrawlerConfig.SeedURLs,
		input.ScanSessionID,
		ce.progressDisplay,
	)

	if err != nil {
		ce.logger.Error().Err(err).Msg("Failed to execute crawler batch")

		// Update progress - failed
		if ce.progressDisplay != nil {
			ce.progressDisplay.UpdateWorkflowProgress(1, 5, "Failed", fmt.Sprintf("Crawler execution failed: %v", err))
		}

		result.Error = fmt.Errorf("failed to execute crawler batch: %w", err)
		return result
	}

	result.DiscoveredURLs = batchResult.DiscoveredURLs
	result.CrawlerInstance = batchResult.CrawlerInstance

	// Update progress - crawler completed
	if ce.progressDisplay != nil {
		ce.progressDisplay.UpdateWorkflowProgress(1, 5, "Crawler Complete", fmt.Sprintf("Discovered %d URLs", len(result.DiscoveredURLs)))
	}

	ce.logger.Info().
		Int("discovered_count", len(result.DiscoveredURLs)).
		Str("session_id", input.ScanSessionID).
		Msg("Crawler completed using managed instance")

	// Only warn if no URLs discovered
	if len(result.DiscoveredURLs) == 0 {
		ce.logger.Warn().
			Str("session_id", input.ScanSessionID).
			Msg("No URLs discovered by crawler")
	}

	return result
}

// Shutdown gracefully shuts down the crawler executor and its managed crawler
func (ce *CrawlerExecutor) Shutdown() {
	ce.logger.Info().Msg("Shutting down crawler executor")
	if ce.crawlerManager != nil {
		ce.crawlerManager.Shutdown()
	}
	ce.logger.Info().Msg("Crawler executor shutdown complete")
}
