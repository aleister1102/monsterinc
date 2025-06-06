package crawler

import (
	"context"
	"time"
)

// RunBatch runs the crawler for a specific batch without full initialization/shutdown
func (cr *Crawler) RunBatch(ctx context.Context, seedURLs []string) {
	cr.ctx = ctx
	cr.crawlStartTime = time.Now()

	// Start URL batch processor if not already running
	if cr.urlQueue == nil {
		cr.initializeURLBatcher()
		cr.startURLBatchProcessor()
	}

	cr.logger.Info().
		Int("seed_count", len(seedURLs)).
		Msg("Starting crawler batch")

	// Process seed URLs for this batch
	cr.processSeedURLsBatch(seedURLs)
	cr.waitForBatchCompletion()
	cr.logBatchSummary(seedURLs)
}

// processSeedURLsBatch processes seed URLs for a single batch
func (cr *Crawler) processSeedURLsBatch(seedURLs []string) {
	cr.logger.Info().
		Int("total_seeds", len(seedURLs)).
		Msg("Starting to process seed URLs for batch")

	for i, seed := range seedURLs {
		// Check context cancellation at the start of each seed processing
		if cr.isContextCancelled() {
			cr.logger.Info().
				Int("processed", i).
				Int("total", len(seedURLs)).
				Msg("Context cancelled during seed URL processing, stopping batch")
			return
		}

		cr.logger.Debug().
			Str("seed", seed).
			Int("index", i+1).
			Int("total", len(seedURLs)).
			Msg("Processing seed URL")

		cr.processSeedURLDirect(seed)

		// Log progress every 10 URLs
		if (i+1)%10 == 0 || i == len(seedURLs)-1 {
			cr.logger.Info().
				Int("processed", i+1).
				Int("total", len(seedURLs)).
				Int("discovered_so_far", len(cr.discoveredURLs)).
				Msg("Seed URL processing progress")
		}
	}

	cr.logger.Info().
		Int("total_processed", len(seedURLs)).
		Int("total_discovered", len(cr.discoveredURLs)).
		Msg("Completed processing all seed URLs for batch")
}

// processSeedURLDirect processes a single seed URL directly via collector
func (cr *Crawler) processSeedURLDirect(seed string) {
	cr.logger.Debug().
		Str("seed", seed).
		Msg("Processing seed URL directly")

	if err := cr.collector.Visit(seed); err != nil {
		cr.handleVisitError(seed, err)
	}
}

// waitForBatchCompletion waits for batch completion without shutting down the crawler
func (cr *Crawler) waitForBatchCompletion() {
	cr.logger.Debug().Int("active_threads", cr.threads).Msg("Waiting for crawler batch threads to complete")

	// Create a channel to signal when colly completes this batch
	done := make(chan struct{})
	go func() {
		defer close(done)
		cr.logger.Debug().Msg("Starting to wait for collector...")
		cr.collector.Wait()
		cr.logger.Debug().Msg("Collector wait completed")
	}()

	// Maximum wait time for batch completion (longer than request timeout)
	maxWaitTime := time.Duration(cr.config.RequestTimeoutSecs+30) * time.Second
	if maxWaitTime < 60*time.Second {
		maxWaitTime = 60 * time.Second // Minimum 1 minute
	}

	cr.logger.Info().
		Dur("max_wait_time", maxWaitTime).
		Msg("Waiting for batch completion with timeout")

	// Wait for either completion, context cancellation, or timeout
	select {
	case <-done:
		cr.logger.Debug().Msg("Crawler batch threads completed normally")
	case <-cr.ctx.Done():
		cr.logger.Info().Msg("Context cancelled, stopping crawler batch")
		// Give a brief grace period for current requests to complete
		select {
		case <-done:
			cr.logger.Debug().Msg("Crawler batch threads completed during grace period")
		case <-time.After(2 * time.Second):
			cr.logger.Warn().Msg("Crawler batch threads did not complete within grace period")
		}
		return
	case <-time.After(maxWaitTime):
		cr.logger.Warn().
			Dur("timeout", maxWaitTime).
			Int("visited", cr.totalVisited).
			Int("errors", cr.totalErrors).
			Msg("Batch completion timeout reached, forcing continuation")
		return
	}
}

// logBatchSummary logs the batch crawling summary statistics
func (cr *Crawler) logBatchSummary(seedURLs []string) {
	duration := time.Since(cr.crawlStartTime)

	cr.mutex.RLock()
	visited := cr.totalVisited
	discovered := len(cr.discoveredURLs)
	errors := cr.totalErrors
	cr.mutex.RUnlock()

	cr.logger.Info().Strs("seeds", seedURLs).Msg("Crawler batch finished")
	cr.logger.Info().
		Dur("duration", duration).
		Int("visited", visited).
		Int("discovered", discovered).
		Int("errors", errors).
		Msg("Crawler batch summary")
}
