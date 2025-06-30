package httpclient

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/rs/zerolog"
)

// RetryHandler handles HTTP request retries with exponential backoff
type RetryHandler struct {
	maxRetries       int
	baseDelay        time.Duration
	maxDelay         time.Duration
	enableJitter     bool
	retryStatusCodes map[int]bool
	logger           zerolog.Logger
}

// RetryHandlerConfig configuration for retry handler
type RetryHandlerConfig struct {
	MaxRetries       int           `json:"max_retries"`
	BaseDelay        time.Duration `json:"base_delay"`
	MaxDelay         time.Duration `json:"max_delay"`
	EnableJitter     bool          `json:"enable_jitter"`
	RetryStatusCodes []int         `json:"retry_status_codes"`
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config RetryHandlerConfig, logger zerolog.Logger) *RetryHandler {
	statusCodeMap := make(map[int]bool)
	for _, code := range config.RetryStatusCodes {
		statusCodeMap[code] = true
	}

	return &RetryHandler{
		maxRetries:       config.MaxRetries,
		baseDelay:        config.BaseDelay,
		maxDelay:         config.MaxDelay,
		enableJitter:     config.EnableJitter,
		retryStatusCodes: statusCodeMap,
		logger:           logger.With().Str("component", "RetryHandler").Logger(),
	}
}

// ShouldRetry determines if a request should be retried based on status code
func (rh *RetryHandler) ShouldRetry(statusCode int, attempt int) bool {
	if attempt >= rh.maxRetries {
		return false
	}
	return rh.retryStatusCodes[statusCode]
}

// CalculateDelay calculates the delay for the next retry attempt using exponential backoff
func (rh *RetryHandler) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rh.baseDelay
	}

	// Exponential backoff: baseDelay * 2^attempt
	delay := rh.baseDelay * time.Duration(math.Pow(2, float64(attempt)))

	// Cap at max delay
	if delay > rh.maxDelay {
		delay = rh.maxDelay
	}

	// Add jitter to prevent thundering herd
	if rh.enableJitter {
		jitter := time.Duration(rand.Intn(int(delay.Milliseconds()/10))) * time.Millisecond
		delay += jitter
	}

	return delay
}

// WaitForRetry waits for the calculated delay before retrying
func (rh *RetryHandler) WaitForRetry(ctx context.Context, attempt int, statusCode int, url string) error {
	delay := rh.CalculateDelay(attempt)

	rh.logger.Warn().
		Str("url", url).
		Int("status_code", statusCode).
		Int("attempt", attempt+1).
		Int("max_retries", rh.maxRetries).
		Dur("delay", delay).
		Msg("Rate limited, waiting before retry")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// DoWithRetry executes an HTTP request with retry logic
func (rh *RetryHandler) DoWithRetry(ctx context.Context, client *HTTPClient, req *HTTPRequest) (*HTTPResponse, error) {
	var lastResp *HTTPResponse
	var lastErr error

	for attempt := 0; attempt <= rh.maxRetries; attempt++ {
		// Check context cancellation before each attempt
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			lastResp = nil

			// For network errors, retry immediately without delay for first few attempts
			if attempt < rh.maxRetries {
				rh.logger.Debug().
					Str("url", req.URL).
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
		if rh.ShouldRetry(resp.StatusCode, attempt) {
			if attempt < rh.maxRetries {
				if err := rh.WaitForRetry(ctx, attempt, resp.StatusCode, req.URL); err != nil {
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
		return nil, errorwrapper.WrapError(lastErr, "all retry attempts failed")
	}

	// Return the last response even if it had a retryable status code
	return lastResp, nil
}
