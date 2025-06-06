package crawler

import (
	"context"
	"net/url"
	"time"

	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// Start initiates the crawling process with configured seed URLs
func (cr *Crawler) Start(ctx context.Context) {
	cr.ctx = ctx
	cr.crawlStartTime = time.Now()

	// Start URL batch processor for improved performance
	cr.startURLBatchProcessor()

	// Ensure cleanup on exit
	defer cr.Stop()

	cr.logger.Info().
		Int("seed_count", len(cr.seedURLs)).
		Strs("seeds", cr.seedURLs).
		Msg("Starting crawl")

	cr.processSeedURLs()
	cr.waitForCompletion()
	cr.logSummary()
}

// processSeedURLs processes all seed URLs for crawling
func (cr *Crawler) processSeedURLs() {
	for _, seed := range cr.seedURLs {
		if cr.isContextCancelled() {
			return
		}

		cr.processSeedURL(seed)
	}
}

// processSeedURL processes a single seed URL
func (cr *Crawler) processSeedURL(seed string) {
	parsedSeed, err := urlhandler.ResolveURL(seed, nil)
	if err != nil {
		cr.logger.Error().Str("seed", seed).Err(err).Msg("Invalid or non-absolute seed URL")
		return
	}

	baseForSeed, _ := url.Parse(parsedSeed)
	cr.DiscoverURL(parsedSeed, baseForSeed)
}

// waitForCompletion waits for all crawling threads to complete
func (cr *Crawler) waitForCompletion() {
	cr.logger.Info().Int("active_threads", cr.threads).Msg("Waiting for crawler threads to complete")

	// Wait for all colly requests to complete
	cr.collector.Wait()

	cr.logger.Info().Msg("All crawler threads completed")
}

// logSummary logs the crawling summary statistics
func (cr *Crawler) logSummary() {
	duration := time.Since(cr.crawlStartTime)

	cr.mutex.RLock()
	visited := cr.totalVisited
	discovered := len(cr.discoveredURLs)
	errors := cr.totalErrors
	cr.mutex.RUnlock()

	cr.logger.Info().Strs("seeds", cr.seedURLs).Msg("Crawl finished")
	cr.logger.Info().
		Dur("duration", duration).
		Int("visited", visited).
		Int("discovered", discovered).
		Int("errors", errors).
		Msg("Crawl summary")
}

// Stop gracefully shuts down the crawler and its components
func (cr *Crawler) Stop() {
	cr.logger.Info().Msg("Stopping crawler...")

	// Stop URL batch processor first to prevent new URLs being queued
	if cr.urlQueue != nil {
		cr.stopURLBatchProcessor()
		cr.logger.Debug().Msg("URL batch processor stopped")
	}

	// Wait for colly collector to finish all current operations
	if cr.collector != nil {
		cr.collector.Wait()
		cr.logger.Debug().Msg("Colly collector stopped")
	}

	// Stop headless browser manager
	if cr.headlessBrowserManager != nil {
		cr.headlessBrowserManager.Stop()
		cr.logger.Debug().Msg("Headless browser manager stopped")
	}

	cr.logger.Info().Msg("Crawler stopped completely")
}
