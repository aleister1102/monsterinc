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
	cr.logger.Info().Int("active_threads", cr.threads).Msg("Waiting for threads to complete")
	cr.collector.Wait()
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
		Msg("Summary")
}
