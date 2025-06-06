package crawler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// DiscoverURL attempts to add a new URL to the crawl queue
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	if cr.isContextCancelled() {
		cr.logger.Debug().Str("raw_url", rawURL).Msg("Context cancelled, skipping URL discovery")
		return
	}

	normalizedURL, shouldSkip := cr.processRawURL(rawURL, base)
	if shouldSkip {
		return
	}

	if cr.isURLAlreadyDiscovered(normalizedURL) {
		return
	}

	if cr.shouldSkipURLByContentLength(normalizedURL) {
		cr.addDiscoveredURL(normalizedURL)
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
		return "", true
	}

	normalizedURL := strings.TrimSpace(absURL)
	if normalizedURL == "" {
		return "", true
	}

	if !cr.isURLInScope(normalizedURL) {
		return "", true
	}

	return normalizedURL, false
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

// shouldSkipURLByContentLength performs HEAD request to check content length
func (cr *Crawler) shouldSkipURLByContentLength(normalizedURL string) bool {
	headReq, err := http.NewRequest("HEAD", normalizedURL, nil)
	if err != nil {
		return false
	}

	resp, err := cr.httpClient.Do(headReq)
	if err != nil {
		cr.logger.Warn().Str("url", normalizedURL).Err(err).Msg("HEAD request failed")
		return false
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			cr.logger.Error().Err(err).Str("url", normalizedURL).Msg("Failed to close response body")
		}
	}()

	return cr.checkContentLength(resp, normalizedURL)
}

// checkContentLength validates response content length
func (cr *Crawler) checkContentLength(resp *http.Response, normalizedURL string) bool {
	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return false
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return false
	}

	if size > cr.maxContentLength {
		cr.logger.Info().
			Str("url", normalizedURL).
			Int64("size", size).
			Int64("max_size", cr.maxContentLength).
			Msg("Skip queue (Content-Length exceeded)")
		return true
	}

	return false
}

// queueURLForVisit adds URL to batched queue for crawling
func (cr *Crawler) queueURLForVisit(normalizedURL string) {
	cr.mutex.Lock()

	// Double-check after acquiring write lock
	if cr.discoveredURLs[normalizedURL] {
		cr.mutex.Unlock()
		return
	}

	cr.discoveredURLs[normalizedURL] = true
	cr.mutex.Unlock()

	// Try to send to batch queue, fallback to immediate processing if queue is full
	select {
	case cr.urlQueue <- normalizedURL:
		// Successfully queued for batch processing
	default:
		// Queue full, process immediately
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
