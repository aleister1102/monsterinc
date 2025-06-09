package crawler

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// DomainRateLimitTracker tracks rate limit errors per domain
type DomainRateLimitTracker struct {
	errorCounts   map[string]int
	blacklistedAt map[string]time.Time
	mutex         sync.RWMutex
	config        config.DomainRateLimitConfig
	logger        zerolog.Logger
	// URL normalizer for removing duplicates by fragment/tracking params
	urlNormalizer *urlhandler.URLNormalizer
}

// NewDomainRateLimitTracker creates a new domain rate limit tracker
func NewDomainRateLimitTracker(config config.DomainRateLimitConfig, normalizationConfig urlhandler.URLNormalizationConfig, logger zerolog.Logger) *DomainRateLimitTracker {
	return &DomainRateLimitTracker{
		errorCounts:   make(map[string]int),
		blacklistedAt: make(map[string]time.Time),
		config:        config,
		logger:        logger.With().Str("component", "DomainRateLimitTracker").Logger(),
		urlNormalizer: urlhandler.NewURLNormalizer(normalizationConfig),
	}
}

// ShouldSkipURL checks if URL should be skipped due to normalization duplicates
func (drlt *DomainRateLimitTracker) ShouldSkipURL(rawURL string) (bool, string) {
	if drlt.urlNormalizer == nil {
		return false, rawURL
	}

	normalizedURL, err := drlt.urlNormalizer.NormalizeURL(rawURL)
	if err != nil {
		drlt.logger.Debug().
			Str("url", rawURL).
			Err(err).
			Msg("Failed to normalize URL, using original")
		return false, rawURL
	}

	// Check if the normalized URL is different (indicating duplicates were removed)
	if normalizedURL != rawURL {
		drlt.logger.Debug().
			Str("original_url", rawURL).
			Str("normalized_url", normalizedURL).
			Msg("URL normalized - fragment/tracking params removed")
	}

	return false, normalizedURL
}

// RecordRateLimitError records a 429 error for a domain
func (drlt *DomainRateLimitTracker) RecordRateLimitError(domain string) bool {
	if !drlt.config.Enabled {
		return false
	}

	drlt.mutex.Lock()
	defer drlt.mutex.Unlock()

	drlt.errorCounts[domain]++

	if drlt.errorCounts[domain] >= drlt.config.MaxRateLimitErrors {
		drlt.blacklistedAt[domain] = time.Now()
		drlt.logger.Warn().
			Str("domain", domain).
			Int("error_count", drlt.errorCounts[domain]).
			Int("max_errors", drlt.config.MaxRateLimitErrors).
			Dur("blacklist_duration", time.Duration(drlt.config.BlacklistDurationMins)*time.Minute).
			Msg("Domain blacklisted due to excessive rate limiting")
		return true
	}

	drlt.logger.Debug().
		Str("domain", domain).
		Int("error_count", drlt.errorCounts[domain]).
		Int("max_errors", drlt.config.MaxRateLimitErrors).
		Msg("Rate limit error recorded for domain")

	return false
}

// IsDomainBlacklisted checks if a domain is currently blacklisted
func (drlt *DomainRateLimitTracker) IsDomainBlacklisted(domain string) bool {
	if !drlt.config.Enabled {
		return false
	}

	drlt.mutex.RLock()
	blacklistedAt, exists := drlt.blacklistedAt[domain]
	drlt.mutex.RUnlock()

	if !exists {
		return false
	}

	// Check if blacklist duration has expired
	blacklistDuration := time.Duration(drlt.config.BlacklistDurationMins) * time.Minute
	if time.Since(blacklistedAt) > blacklistDuration {
		drlt.removeFromBlacklist(domain)
		return false
	}

	return true
}

// removeFromBlacklist removes a domain from blacklist
func (drlt *DomainRateLimitTracker) removeFromBlacklist(domain string) {
	drlt.mutex.Lock()
	defer drlt.mutex.Unlock()

	delete(drlt.blacklistedAt, domain)
	drlt.errorCounts[domain] = 0

	drlt.logger.Info().
		Str("domain", domain).
		Msg("Domain removed from blacklist")
}

// CleanupOldEntries removes old blacklist entries
func (drlt *DomainRateLimitTracker) CleanupOldEntries() {
	if !drlt.config.Enabled {
		return
	}

	drlt.mutex.Lock()
	defer drlt.mutex.Unlock()

	clearAfter := time.Duration(drlt.config.BlacklistClearAfterHours) * time.Hour
	now := time.Now()

	for domain, blacklistedAt := range drlt.blacklistedAt {
		if now.Sub(blacklistedAt) > clearAfter {
			delete(drlt.blacklistedAt, domain)
			delete(drlt.errorCounts, domain)
			drlt.logger.Debug().
				Str("domain", domain).
				Msg("Cleared old blacklist entry")
		}
	}
}

// GetStats returns current tracker statistics
func (drlt *DomainRateLimitTracker) GetStats() map[string]interface{} {
	drlt.mutex.RLock()
	defer drlt.mutex.RUnlock()

	return map[string]interface{}{
		"total_domains":       len(drlt.errorCounts),
		"blacklisted_domains": len(drlt.blacklistedAt),
		"error_counts":        drlt.errorCounts,
	}
}

// RetryTransport wraps http.RoundTripper with retry logic for rate limiting
type RetryTransport struct {
	base             http.RoundTripper
	retryConfig      config.RetryConfig
	logger           zerolog.Logger
	retryStatusCodes map[int]bool
	domainTracker    *DomainRateLimitTracker
}

// NewRetryTransport creates a new RetryTransport
func NewRetryTransport(base http.RoundTripper, retryConfig config.RetryConfig, urlNormalizationConfig urlhandler.URLNormalizationConfig, logger zerolog.Logger) *RetryTransport {
	statusCodeMap := make(map[int]bool)
	for _, code := range retryConfig.RetryStatusCodes {
		statusCodeMap[code] = true
	}

	domainTracker := NewDomainRateLimitTracker(retryConfig.DomainLevelRateLimit, urlNormalizationConfig, logger)

	return &RetryTransport{
		base:             base,
		retryConfig:      retryConfig,
		logger:           logger.With().Str("component", "RetryTransport").Logger(),
		retryStatusCodes: statusCodeMap,
		domainTracker:    domainTracker,
	}
}

// RoundTrip implements http.RoundTripper with retry logic
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check if domain is blacklisted
	domain := req.URL.Hostname()
	if rt.domainTracker.IsDomainBlacklisted(domain) {
		rt.logger.Warn().
			Str("url", req.URL.String()).
			Str("domain", domain).
			Msg("Skipping request to blacklisted domain")
		return nil, &url.Error{
			Op:  "GET",
			URL: req.URL.String(),
			Err: ErrDomainBlacklisted,
		}
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= rt.retryConfig.MaxRetries; attempt++ {
		// Check context cancellation before each attempt
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}

		// Clone request for retry attempts (required for http.Request)
		reqClone := rt.cloneRequest(req)

		resp, err := rt.base.RoundTrip(reqClone)
		if err != nil {
			lastErr = err
			lastResp = nil

			// For network errors, retry immediately without delay for first few attempts
			if attempt < rt.retryConfig.MaxRetries {
				rt.logger.Debug().
					Str("url", req.URL.String()).
					Int("attempt", attempt+1).
					Err(err).
					Msg("Network error, retrying immediately")
				continue
			}
			break
		}

		lastResp = resp
		lastErr = nil

		// Check if we should retry based on status code
		if rt.shouldRetry(resp.StatusCode, attempt) {
			// Record rate limit error for domain tracking
			if resp.StatusCode == 429 {
				wasBlacklisted := rt.domainTracker.RecordRateLimitError(domain)
				if wasBlacklisted {
					rt.logger.Warn().
						Str("url", req.URL.String()).
						Str("domain", domain).
						Msg("Domain blacklisted due to excessive rate limiting, aborting retries")
					break
				}
			}

			if attempt < rt.retryConfig.MaxRetries {
				// Close response body before retry
				if resp.Body != nil {
					_ = resp.Body.Close()
				}

				if err := rt.waitForRetry(req.Context(), attempt, resp.StatusCode, req.URL.String()); err != nil {
					return nil, err
				}
				continue
			}
		}

		// Success or non-retryable error
		return resp, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}

	// Return the last response even if it had a retryable status code
	return lastResp, nil
}

// GetDomainTracker returns the domain tracker for cleanup and stats
func (rt *RetryTransport) GetDomainTracker() *DomainRateLimitTracker {
	return rt.domainTracker
}

// ErrDomainBlacklisted indicates a domain is temporarily blacklisted
var ErrDomainBlacklisted = &url.Error{
	Op:  "GET",
	URL: "",
	Err: &DomainBlacklistedError{},
}

// DomainBlacklistedError represents a domain blacklist error
type DomainBlacklistedError struct{}

func (e *DomainBlacklistedError) Error() string {
	return "domain is temporarily blacklisted due to excessive rate limiting"
}

// shouldRetry determines if a request should be retried based on status code
func (rt *RetryTransport) shouldRetry(statusCode int, attempt int) bool {
	if attempt >= rt.retryConfig.MaxRetries {
		return false
	}
	return rt.retryStatusCodes[statusCode]
}

// waitForRetry waits for the calculated delay before retrying
func (rt *RetryTransport) waitForRetry(ctx context.Context, attempt int, statusCode int, url string) error {
	delay := rt.calculateDelay(attempt)

	rt.logger.Warn().
		Str("url", url).
		Int("status_code", statusCode).
		Int("attempt", attempt+1).
		Int("max_retries", rt.retryConfig.MaxRetries).
		Dur("delay", delay).
		Msg("Rate limited, waiting before retry")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// calculateDelay calculates the delay for the next retry attempt using exponential backoff
func (rt *RetryTransport) calculateDelay(attempt int) time.Duration {
	baseDelay := time.Duration(rt.retryConfig.BaseDelaySecs) * time.Second
	maxDelay := time.Duration(rt.retryConfig.MaxDelaySecs) * time.Second

	if attempt <= 0 {
		return baseDelay
	}

	// Exponential backoff: baseDelay * 2^attempt
	delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))

	// Cap at max delay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter to prevent thundering herd
	if rt.retryConfig.EnableJitter {
		jitter := time.Duration(rand.Intn(int(delay.Milliseconds()/10))) * time.Millisecond
		delay += jitter
	}

	return delay
}

// cloneRequest creates a shallow clone of the HTTP request
func (rt *RetryTransport) cloneRequest(req *http.Request) *http.Request {
	// Clone the request
	reqClone := req.Clone(req.Context())

	// For GET requests, we don't need to worry about the body
	// For other methods, the body should be reusable (like bytes.Reader)
	return reqClone
}
