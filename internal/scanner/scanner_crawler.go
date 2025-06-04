package scanner

import (
	"context"
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
)

// executeCrawler runs the crawler and returns discovered URLs
func (so *Scanner) executeCrawler(
	ctx context.Context,
	crawlerConfig *config.CrawlerConfig,
	scanSessionID,
	primaryRootTargetURL string,
) ([]string, error) {
	var discoveredURLs []string

	// Run crawler if seed URLs provided
	if len(crawlerConfig.SeedURLs) > 0 {
		// Check for context cancellation before starting crawler
		if cancelled := common.CheckCancellationWithLog(ctx, so.logger, "crawler start"); cancelled.Cancelled {
			return nil, cancelled.Error
		}

		so.logger.Info().Int("seed_count", len(crawlerConfig.SeedURLs)).Str("session_id", scanSessionID).Str("primary_target", primaryRootTargetURL).Msg("Starting crawler")

		crawler, err := crawler.NewCrawler(crawlerConfig, so.logger)
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
