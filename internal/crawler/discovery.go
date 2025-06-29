package crawler

import (
	"net/url"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common/urlhandler"
)

// DiscoverURL normalizes, validates and potentially adds a URL to the crawl queue.
// This method is called from various places (HTML parsing, redirects, etc.)
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	if rawURL == "" {
		return
	}

	// Apply URL normalization if available
	var normalizedRawURL string
	if cr.config.URLNormalization.StripFragments || cr.config.URLNormalization.StripTrackingParams {
		normalizer := urlhandler.NewURLNormalizer(cr.config.URLNormalization)
		if normalized, err := normalizer.NormalizeURL(rawURL); err == nil {
			normalizedRawURL = normalized
			if normalized != rawURL {
				cr.logger.Debug().
					Str("original_url", rawURL).
					Str("normalized_url", normalized).
					Msg("URL normalized during discovery")
			}
		} else {
			normalizedRawURL = rawURL
			cr.logger.Debug().
				Str("url", rawURL).
				Err(err).
				Msg("Failed to normalize URL, using original")
		}
	} else {
		normalizedRawURL = rawURL
	}

	normalizedURL, shouldProcess := cr.processRawURL(normalizedRawURL, base)
	if !shouldProcess {
		return
	}

	// Check if URL should be skipped due to pattern similarity
	// Only apply auto-calibrate if it's enabled and URLs haven't been preprocessed at Scanner level
	if cr.config.AutoCalibrate.Enabled && cr.patternDetector.ShouldSkipURL(normalizedURL) {
		cr.logger.Debug().
			Str("url", normalizedURL).
			Msg("Skipping URL due to pattern similarity (auto-calibrate)")
		return
	}

	// Check scope, duplicates, and content-length as before
	if !cr.isURLInScope(normalizedURL) {
		return
	}

	if cr.isURLAlreadyDiscovered(normalizedURL) {
		return
	}

	cr.queueURLForVisit(normalizedURL)
}

// processRawURL resolves and validates the raw URL
func (cr *Crawler) processRawURL(rawURL string, base *url.URL) (string, bool) {
	absURL, err := urlhandler.ResolveURL(rawURL, base)
	if err != nil {
		cr.logger.Warn().
			Str("raw_url", rawURL).
			Str("base", base.String()).
			Err(err).
			Msg("Could not resolve URL")

		cr.addDiscoveredURL(rawURL)
		return "", false
	}

	normalizedURL := strings.TrimSpace(absURL)
	if normalizedURL == "" {
		return "", false
	}

	if !cr.isURLInScope(normalizedURL) {
		return "", false
	}

	return normalizedURL, true
}

// isURLInScope checks if URL is within crawler scope
func (cr *Crawler) isURLInScope(normalizedURL string) bool {
	if cr.scope == nil {
		return true
	}

	isAllowed, err := cr.scope.IsURLAllowed(normalizedURL)
	if err != nil {
		cr.logger.Warn().Str("url", normalizedURL).Err(err).Msg("Scope check encountered an issue")
		return false
	}

	return isAllowed
}

// isURLAlreadyDiscovered checks if URL was already discovered
func (cr *Crawler) isURLAlreadyDiscovered(normalizedURL string) bool {
	cr.mutex.RLock()
	exists := cr.discoveredURLs[normalizedURL]
	cr.mutex.RUnlock()
	return exists
}

// queueURLForVisit adds URL to batched queue for crawling
func (cr *Crawler) queueURLForVisit(normalizedURL string) {
	cr.mutex.Lock()

	// Double-check after acquiring write lock
	if cr.discoveredURLs[normalizedURL] {
		cr.mutex.Unlock()
		cr.logger.Debug().Str("url", normalizedURL).Msg("URL already discovered, skipping")
		return
	}

	cr.discoveredURLs[normalizedURL] = true
	cr.mutex.Unlock()

	cr.logger.Debug().Str("url", normalizedURL).Msg("Queueing URL for visit")

	// Try to send to batch queue, fallback to immediate processing if queue is full
	select {
	case cr.urlQueue <- normalizedURL:
		// Successfully queued for batch processing
		cr.logger.Debug().Str("url", normalizedURL).Msg("URL queued for batch processing")
	default:
		// Queue full, process immediately
		cr.logger.Debug().Str("url", normalizedURL).Msg("Queue full, processing URL immediately")
		if err := cr.collector.Visit(normalizedURL); err != nil {
			cr.handleVisitError(normalizedURL, err)
		}
	}
}

// addDiscoveredURL safely adds URL to discovered list
func (cr *Crawler) addDiscoveredURL(url string) {
	cr.mutex.Lock()
	cr.discoveredURLs[url] = true
	cr.mutex.Unlock()
}
