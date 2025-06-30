package crawler

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/urlhandler"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// RetryTransport wraps http.RoundTripper with retry logic for rate limiting
type RetryTransport struct {
	base             http.RoundTripper
	retryConfig      config.RetryConfig
	logger           zerolog.Logger
	retryStatusCodes map[int]bool
}

// NewRetryTransport creates a new RetryTransport
func NewRetryTransport(base http.RoundTripper, retryConfig config.RetryConfig, urlNormalizationConfig urlhandler.URLNormalizationConfig, logger zerolog.Logger) *RetryTransport {
	statusCodeMap := make(map[int]bool)
	for _, code := range retryConfig.RetryStatusCodes {
		statusCodeMap[code] = true
	}

	return &RetryTransport{
		base:             base,
		retryConfig:      retryConfig,
		logger:           logger.With().Str("component", "RetryTransport").Logger(),
		retryStatusCodes: statusCodeMap,
	}
}

// RoundTrip implements http.RoundTripper with retry logic
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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
