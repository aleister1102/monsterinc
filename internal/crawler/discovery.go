package crawler

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// DiscoverURL processes a raw URL for potential crawling
// This function validates, normalizes, and queues URLs for crawling based on scope rules
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	// Check context cancellation early to avoid unnecessary processing
	if cr.isContextCancelled() {
		return
	}

	normalizedURL, shouldSkip := cr.processRawURL(rawURL, base)
	if shouldSkip {
		return
	}

	// Final context check before queuing for crawling
	if cr.isContextCancelled() {
		return
	}

	if cr.isURLAlreadyDiscovered(normalizedURL) {
		return
	}

	// Check if URL should be skipped due to pattern similarity (auto-calibrate)
	if cr.patternDetector.ShouldSkipURL(normalizedURL) {
		cr.addDiscoveredURL(normalizedURL)
		return
	}

	// Only check content length if enabled in config
	if cr.config.EnableContentLengthCheck && cr.shouldSkipURLByContentLength(normalizedURL) {
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
	// Skip content length check if max content length is 0 (unlimited)
	if cr.maxContentLength <= 0 {
		return false
	}

	// Create a short timeout context for HEAD request (5 seconds max)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &common.HTTPRequest{
		URL:     normalizedURL,
		Method:  "HEAD",
		Headers: make(map[string]string),
		Context: ctx,
	}

	resp, err := cr.httpClient.DoWithRetry(req)
	if err != nil {
		// If HEAD request fails, don't skip - let the main crawler handle it
		cr.logger.Debug().Str("url", normalizedURL).Err(err).Msg("HEAD request failed, allowing URL")
		return false
	}

	return cr.checkContentLength(resp, normalizedURL)
}

// checkContentLength validates response content length
func (cr *Crawler) checkContentLength(resp *common.HTTPResponse, normalizedURL string) bool {
	contentLength := resp.Headers["Content-Length"]
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
