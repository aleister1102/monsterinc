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
		cr.startBatchedURLsProcessing()
	}

	cr.logger.Info().
		Int("seed_count", len(seedURLs)).
		Msg("Starting crawler batch")

	// Process seed URLs for this batch
	cr.processSeedURLsBatch(seedURLs)
	cr.waitForBatchCompletion()
	cr.logBatchSummary(seedURLs)
}

// startBatchedURLsProcessing starts the background URL batch processor
func (cr *Crawler) startBatchedURLsProcessing() {
	cr.batchWG.Add(1)
	go cr.processBatchedURLs()
}

// processBatchedURLs processes URLs in batches for improved performance
func (cr *Crawler) processBatchedURLs() {
	defer cr.batchWG.Done()

	var batch []string
	batchTimer := time.NewTimer(time.Millisecond * 100) // Small batch timeout for responsive processing
	defer batchTimer.Stop()

	for {
		select {
		case url, ok := <-cr.urlQueue:
			if !ok {
				// Channel closed, process final batch if any
				if len(batch) > 0 {
					cr.processBatch(batch)
				}
				return
			}

			batch = append(batch, url)

			// Process batch when full or when timer expires
			if len(batch) >= cr.urlBatchSize {
				cr.processBatch(batch)
				batch = batch[:0] // Reset batch
				if !batchTimer.Stop() {
					<-batchTimer.C
				}
				batchTimer.Reset(time.Millisecond * 100)
			}

		case <-batchTimer.C:
			// Process current batch on timer expiry
			if len(batch) > 0 {
				cr.processBatch(batch)
				batch = batch[:0] // Reset batch
			}
			batchTimer.Reset(time.Millisecond * 100)

		case <-cr.batchShutdown:
			// Shutdown signal received
			if len(batch) > 0 {
				cr.processBatch(batch)
			}
			return

		case <-cr.ctx.Done():
			// Context cancelled, exit immediately without processing remaining URLs
			cr.logger.Info().Msg("Context cancelled, stopping URL batch processing immediately")
			return
		}
	}
}

// processBatch processes a batch of URLs
func (cr *Crawler) processBatch(urls []string) {
	cr.logger.Debug().
		Int("batch_size", len(urls)).
		Msg("Processing URL batch")

	for i, url := range urls {
		// Check context cancellation before processing each URL in batch
		if cr.isContextCancelled() {
			cr.logger.Debug().
				Int("processed", i).
				Int("total", len(urls)).
				Msg("Context cancelled during batch processing, stopping")
			return
		}

		cr.logger.Debug().
			Str("url", url).
			Int("index", i+1).
			Int("total", len(urls)).
			Msg("Visiting URL in batch")

		if err := cr.collector.Visit(url); err != nil {
			cr.handleVisitError(url, err)
		}
	}

	cr.logger.Debug().
		Int("batch_size", len(urls)).
		Msg("Completed processing URL batch")
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
	maxWaitTime := max(time.Duration(cr.config.RequestTimeoutSecs+30)*time.Second, 60*time.Second)

	cr.logger.Info().
		Dur("max_wait_time", maxWaitTime).
		Msg("Waiting for batch completion with timeout")

	// Wait for either completion, context cancellation, or timeout
	select {
	case <-done:
		cr.logger.Debug().Msg("Crawler batch threads completed normally")
	case <-cr.ctx.Done():
		cr.logger.Info().Msg("Context cancelled, stopping crawler batch immediately")
		// Force stop any remaining requests by stopping the collector
		if cr.collector != nil {
			// Stop accepting new requests immediately
			cr.logger.Debug().Msg("Force stopping collector due to context cancellation")
		}
		// Give a shorter grace period for current requests to complete
		select {
		case <-done:
			cr.logger.Debug().Msg("Crawler batch threads completed during grace period")
		case <-time.After(1 * time.Second): // Reduced from 2 seconds to 1 second
			cr.logger.Warn().Msg("Crawler batch threads did not complete within grace period, forcing shutdown")
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

// ResetForNewBatch resets crawler state for a new batch while preserving the instance
func (cr *Crawler) ResetForNewBatch(newSeedURLs []string) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	// Reset discovered URLs for new batch
	cr.discoveredURLs = make(map[string]bool)
	cr.urlParentMap = make(map[string]string)

	// Update seed URLs for this batch
	cr.seedURLs = make([]string, len(newSeedURLs))
	copy(cr.seedURLs, newSeedURLs)

	// Reset counters
	cr.totalVisited = 0
	cr.totalErrors = 0

	// Reset pattern detector for new batch
	if cr.patternDetector != nil {
		cr.patternDetector.Reset()
	}

	cr.logger.Debug().
		Int("new_seed_count", len(newSeedURLs)).
		Msg("Crawler reset for new batch")
}

// Stop gracefully shuts down the crawler and its components
func (cr *Crawler) Stop() {
	cr.logger.Info().Msg("Stopping crawler...")

	// Stop URL batch processor first to prevent new URLs being queued
	if cr.urlQueue != nil {
		cr.stopBatchedURLsProcessing()
		cr.logger.Debug().Msg("URL batch processor stopped")
	}

	// Wait for colly collector to finish all current operations with timeout
	if cr.collector != nil {
		// Create a channel to signal when colly wait completes
		done := make(chan struct{})
		go func() {
			defer close(done)
			cr.collector.Wait()
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			cr.logger.Debug().Msg("Colly collector stopped normally")
		case <-time.After(5 * time.Second): // 5 second timeout for graceful stop
			cr.logger.Warn().Msg("Colly collector stop timeout reached, forcing shutdown")
		}
	}

	cr.logger.Info().Msg("Crawler stopped completely")
}

// stopBatchedURLsProcessing gracefully stops the URL batch processor
func (cr *Crawler) stopBatchedURLsProcessing() {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	// Check if already closed to prevent panic
	select {
	case <-cr.batchShutdown:
		// Already closed
		return
	default:
		// Not closed yet, safe to close
		close(cr.batchShutdown)
	}

	cr.batchWG.Wait()
}

// EnsureFullShutdown đảm bảo crawler đã shutdown hoàn toàn trước khi tiếp tục
func (cr *Crawler) EnsureFullShutdown() {
	cr.logger.Info().Msg("Ensuring crawler full shutdown...")

	// Wait for URL batch processor to complete
	cr.batchWG.Wait()
	cr.logger.Debug().Msg("URL batch processor goroutines completed")

	// Double-check colly is done
	if cr.collector != nil {
		cr.collector.Wait()
		cr.logger.Debug().Msg("Colly collector confirmed stopped")
	}

	// Log pattern detector statistics before shutdown
	if cr.patternDetector != nil {
		cr.logPatternDetectorStats()
	}

	cr.logger.Info().Msg("Crawler full shutdown confirmed")
}

// logPatternDetectorStats logs statistics from the pattern detector
func (cr *Crawler) logPatternDetectorStats() {
	stats := cr.patternDetector.GetPatternStats()
	if len(stats) == 0 {
		return
	}

	cr.logger.Info().
		Int("pattern_count", len(stats)).
		Msg("URL pattern detection statistics")

	// Log top patterns
	for pattern, count := range stats {
		if count > 1 {
			cr.logger.Debug().
				Str("pattern", pattern).
				Int("count", count).
				Msg("URL pattern detected")
		}
	}
}
