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
}

// NewCrawlerExecutor creates a new crawler executor
func NewCrawlerExecutor(logger zerolog.Logger) *CrawlerExecutor {
	return &CrawlerExecutor{
		logger: logger.With().Str("module", "CrawlerExecutor").Logger(),
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

// Execute runs the crawler and returns discovered URLs
// Renamed from executeCrawler for clarity and moved to dedicated executor
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
		ce.progressDisplay.UpdateScanProgress(1, 4, "Crawler", fmt.Sprintf("Starting crawler with %d seed URLs", len(input.CrawlerConfig.SeedURLs)))
	}

	ce.logger.Info().
		Int("seed_count", len(input.CrawlerConfig.SeedURLs)).
		Str("session_id", input.ScanSessionID).
		Str("primary_target", input.PrimaryRootTargetURL).
		Msg("Starting crawler")

	crawlerInstance, err := crawler.NewCrawler(input.CrawlerConfig, ce.logger)
	if err != nil {
		ce.logger.Error().Err(err).Msg("Failed to initialize crawler")

		// Update progress - failed
		if ce.progressDisplay != nil {
			ce.progressDisplay.UpdateScanProgress(1, 4, "Failed", fmt.Sprintf("Crawler initialization failed: %v", err))
		}

		result.Error = fmt.Errorf("failed to initialize crawler: %w", err)
		return result
	}

	// Update progress - crawler running
	if ce.progressDisplay != nil {
		ce.progressDisplay.UpdateScanProgress(1, 4, "Crawling", "Crawler is running and discovering URLs")
	}

	crawlerInstance.Start(input.Context)

	// Đảm bảo crawler đã shutdown hoàn toàn trước khi lấy results
	crawlerInstance.EnsureFullShutdown()

	result.DiscoveredURLs = crawlerInstance.GetDiscoveredURLs()
	result.CrawlerInstance = crawlerInstance

	// Update progress - crawler completed
	if ce.progressDisplay != nil {
		ce.progressDisplay.UpdateScanProgress(1, 4, "Crawler Complete", fmt.Sprintf("Crawler completed: %d URLs discovered", len(result.DiscoveredURLs)))
	}

	ce.logger.Info().
		Int("discovered_count", len(result.DiscoveredURLs)).
		Str("session_id", input.ScanSessionID).
		Msg("Crawler completed")

	// Only warn if no URLs discovered - removed debug logs
	if len(result.DiscoveredURLs) == 0 {
		ce.logger.Warn().
			Str("session_id", input.ScanSessionID).
			Msg("No URLs discovered by crawler")
	}

	return result
}
