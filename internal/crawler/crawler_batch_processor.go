package crawler

import (
	"time"
)

// startURLBatchProcessor starts the background URL batch processor
func (cr *Crawler) startURLBatchProcessor() {
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

// stopURLBatchProcessor gracefully stops the URL batch processor
func (cr *Crawler) stopURLBatchProcessor() {
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
