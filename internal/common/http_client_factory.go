package common

import (
	"time"

	"github.com/rs/zerolog"
)

// HTTPClientFactory provides methods to create common HTTP client configurations
type HTTPClientFactory struct {
	logger zerolog.Logger
}

// NewHTTPClientFactory creates a new HTTP client factory
func NewHTTPClientFactory(logger zerolog.Logger) *HTTPClientFactory {
	return &HTTPClientFactory{logger: logger}
}

// CreateDiscordClient creates an HTTP client optimized for Discord webhook calls
func (f *HTTPClientFactory) CreateDiscordClient(timeout time.Duration) (*HTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithFollowRedirects(true).
		WithMaxRedirects(3).
		WithHTTP2(true).
		Build()
}

// CreateMonitorClient creates an HTTP client optimized for file monitoring
func (f *HTTPClientFactory) CreateMonitorClient(timeout time.Duration, insecureSkipVerify bool) (*HTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithInsecureSkipVerify(insecureSkipVerify).
		WithFollowRedirects(true).
		WithMaxRedirects(5).
		WithConnectionPooling(50, 10, 0).
		WithHTTP2(true).
		Build()
}

// CreateBasicClient creates a basic HTTP client with minimal configuration
func (f *HTTPClientFactory) CreateBasicClient(timeout time.Duration) (*HTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithHTTP2(true).
		Build()
}
