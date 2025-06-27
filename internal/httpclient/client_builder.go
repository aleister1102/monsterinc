package httpclient

import (
	"time"

	"github.com/rs/zerolog"
)

// HTTPClientBuilder builds HTTP clients with fluent interface
type HTTPClientBuilder struct {
	config HTTPClientConfig
	logger zerolog.Logger
}

// NewHTTPClientBuilder creates a new HTTPClientBuilder with default configuration
func NewHTTPClientBuilder(logger zerolog.Logger) *HTTPClientBuilder {
	return &HTTPClientBuilder{
		config: DefaultHTTPClientConfig(),
		logger: logger,
	}
}

// WithTimeout sets the request timeout
func (b *HTTPClientBuilder) WithTimeout(timeout time.Duration) *HTTPClientBuilder {
	b.config.Timeout = timeout
	return b
}

// WithInsecureSkipVerify sets whether to skip TLS verification
func (b *HTTPClientBuilder) WithInsecureSkipVerify(skip bool) *HTTPClientBuilder {
	b.config.InsecureSkipVerify = skip
	return b
}

// WithFollowRedirects sets whether to follow redirects
func (b *HTTPClientBuilder) WithFollowRedirects(follow bool) *HTTPClientBuilder {
	b.config.FollowRedirects = follow
	return b
}

// WithMaxRedirects sets the maximum number of redirects to follow
func (b *HTTPClientBuilder) WithMaxRedirects(max int) *HTTPClientBuilder {
	b.config.MaxRedirects = max
	return b
}

// WithUserAgent sets the User-Agent header
func (b *HTTPClientBuilder) WithUserAgent(userAgent string) *HTTPClientBuilder {
	b.config.UserAgent = userAgent
	return b
}

// WithMaxContentSize sets the maximum content size to fetch in bytes (0 for no limit)
func (b *HTTPClientBuilder) WithMaxContentSize(size int) *HTTPClientBuilder {
	b.config.MaxContentSize = size
	return b
}

// WithConnectionPooling sets connection pooling parameters
func (b *HTTPClientBuilder) WithConnectionPooling(maxIdle, maxIdlePerHost, maxPerHost int) *HTTPClientBuilder {
	b.config.MaxIdleConns = maxIdle
	b.config.MaxIdleConnsPerHost = maxIdlePerHost
	b.config.MaxConnsPerHost = maxPerHost
	return b
}

// WithHTTP2 enables or disables HTTP/2 support
func (b *HTTPClientBuilder) WithHTTP2(enabled bool) *HTTPClientBuilder {
	b.config.EnableHTTP2 = enabled
	return b
}

// Build creates and returns a new HTTPClient
func (b *HTTPClientBuilder) Build() (*HTTPClient, error) {
	return NewHTTPClient(b.config, b.logger)
}
