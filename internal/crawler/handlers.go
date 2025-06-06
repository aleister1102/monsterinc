package crawler

import (
	"errors"
	"strings"

	"github.com/gocolly/colly/v2"
)

// handleError processes colly error callbacks
func (cr *Crawler) handleError(r *colly.Response, e error) {
	cr.incrementErrorCount()

	if cr.isContextCancelled() {
		cr.logger.Warn().Str("url", r.Request.URL.String()).Err(e).Msg("Request failed after context cancellation")
		return
	}

	cr.logger.Error().
		Str("url", r.Request.URL.String()).
		Int("status", r.StatusCode).
		Err(e).
		Msg("Request failed")
}

// handleRequest processes colly request callbacks
func (cr *Crawler) handleRequest(r *colly.Request) {
	if cr.isContextCancelled() {
		cr.logger.Info().Str("url", r.URL.String()).Msg("Context cancelled, aborting request")
		r.Abort()
		return
	}

	if cr.shouldAbortRequest(r) {
		cr.logger.Info().
			Str("url", r.URL.String()).
			Str("path", r.URL.Path).
			Msg("Abort request (file extension not allowed)")
		r.Abort()
	}
}

// handleResponse processes colly response callbacks
func (cr *Crawler) handleResponse(r *colly.Response) {
	cr.incrementVisitedCount()

	if cr.isHTMLContent(r) {
		cr.extractAssetsFromResponse(r)
	}
}

// shouldAbortRequest checks if request should be aborted based on file extensions
func (cr *Crawler) shouldAbortRequest(r *colly.Request) bool {
	if cr.scope == nil {
		return false
	}

	path := r.URL.Path

	// Check disallowed file extensions using fast string operations
	for _, ext := range cr.scope.disallowedFileExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}

// isHTMLContent checks if response contains HTML content
func (cr *Crawler) isHTMLContent(r *colly.Response) bool {
	contentType := r.Headers.Get("Content-Type")
	return contentType != "" && strings.Contains(strings.ToLower(contentType), "text/html")
}

// extractAssetsFromResponse extracts assets from HTML response
func (cr *Crawler) extractAssetsFromResponse(r *colly.Response) {
	assets := ExtractAssetsFromHTML(r.Body, r.Request.URL, cr)
	if len(assets) > 0 {
		cr.logger.Info().
			Str("url", r.Request.URL.String()).
			Int("assets", len(assets)).
			Msg("Extracted assets")
	}
}

// isContextCancelled checks if context is cancelled
func (cr *Crawler) isContextCancelled() bool {
	if cr.ctx == nil {
		return false
	}

	select {
	case <-cr.ctx.Done():
		return true
	default:
		return false
	}
}

// incrementErrorCount safely increments error counter
func (cr *Crawler) incrementErrorCount() {
	cr.mutex.Lock()
	cr.totalErrors++
	cr.mutex.Unlock()
}

// incrementVisitedCount safely increments visited counter
func (cr *Crawler) incrementVisitedCount() {
	cr.mutex.Lock()
	cr.totalVisited++
	cr.mutex.Unlock()
}

// handleVisitError handles errors from colly Visit calls
func (cr *Crawler) handleVisitError(normalizedURL string, err error) {
	if strings.Contains(err.Error(), "already visited") || errors.Is(err, colly.ErrRobotsTxtBlocked) {
		return
	}

	cr.logger.Warn().
		Str("url", normalizedURL).
		Err(err).
		Msg("Error queueing visit")
}
