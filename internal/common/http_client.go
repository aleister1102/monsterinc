package common

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"
)

// HTTPClientConfig holds configuration for HTTP clients
type HTTPClientConfig struct {
	Timeout               time.Duration     // Request timeout
	InsecureSkipVerify    bool              // Skip TLS verification
	FollowRedirects       bool              // Whether to follow redirects
	MaxRedirects          int               // Maximum number of redirects to follow
	Proxy                 string            // Proxy URL (HTTP/SOCKS)
	CustomHeaders         map[string]string // Custom headers to add to all requests
	UserAgent             string            // User-Agent header
	MaxIdleConns          int               // Maximum idle connections
	MaxIdleConnsPerHost   int               // Maximum idle connections per host
	MaxConnsPerHost       int               // Maximum connections per host
	IdleConnTimeout       time.Duration     // Idle connection timeout
	TLSHandshakeTimeout   time.Duration     // TLS handshake timeout
	ExpectContinueTimeout time.Duration     // Expect 100-continue timeout
	DialTimeout           time.Duration     // Connection dial timeout
	KeepAlive             time.Duration     // Keep-alive duration
}

// DefaultHTTPClientConfig returns a default HTTP client configuration
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:               30 * time.Second,
		InsecureSkipVerify:    false,
		FollowRedirects:       true,
		MaxRedirects:          10,
		Proxy:                 "",
		CustomHeaders:         make(map[string]string),
		UserAgent:             "MonsterInc/1.0",
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // 0 means no limit
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
	}
}

// NewHTTPClient creates a new HTTP client with the given configuration
func NewHTTPClient(config HTTPClientConfig, logger zerolog.Logger) (*http.Client, error) {
	// Create custom transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
	}

	// Configure proxy if specified
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, WrapError(err, "failed to parse proxy URL")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		logger.Info().Str("proxy", config.Proxy).Msg("HTTP client configured with proxy")
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Configure redirect policy
	if !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if config.MaxRedirects > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		}
	}

	logger.Debug().
		Dur("timeout", config.Timeout).
		Bool("insecure_skip_verify", config.InsecureSkipVerify).
		Bool("follow_redirects", config.FollowRedirects).
		Int("max_redirects", config.MaxRedirects).
		Msg("HTTP client created")

	return client, nil
}

// HTTPClientBuilder provides a fluent interface for building HTTP clients
type HTTPClientBuilder struct {
	config HTTPClientConfig
	logger zerolog.Logger
}

// NewHTTPClientBuilder creates a new HTTP client builder
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

// WithMaxRedirects sets the maximum number of redirects
func (b *HTTPClientBuilder) WithMaxRedirects(max int) *HTTPClientBuilder {
	b.config.MaxRedirects = max
	return b
}

// WithProxy sets the proxy URL
func (b *HTTPClientBuilder) WithProxy(proxy string) *HTTPClientBuilder {
	b.config.Proxy = proxy
	return b
}

// WithUserAgent sets the User-Agent header
func (b *HTTPClientBuilder) WithUserAgent(userAgent string) *HTTPClientBuilder {
	b.config.UserAgent = userAgent
	return b
}

// WithCustomHeader adds a custom header
func (b *HTTPClientBuilder) WithCustomHeader(key, value string) *HTTPClientBuilder {
	if b.config.CustomHeaders == nil {
		b.config.CustomHeaders = make(map[string]string)
	}
	b.config.CustomHeaders[key] = value
	return b
}

// WithCustomHeaders sets multiple custom headers
func (b *HTTPClientBuilder) WithCustomHeaders(headers map[string]string) *HTTPClientBuilder {
	if b.config.CustomHeaders == nil {
		b.config.CustomHeaders = make(map[string]string)
	}
	for k, v := range headers {
		b.config.CustomHeaders[k] = v
	}
	return b
}

// WithConnectionPooling configures connection pooling settings
func (b *HTTPClientBuilder) WithConnectionPooling(maxIdle, maxIdlePerHost, maxPerHost int) *HTTPClientBuilder {
	b.config.MaxIdleConns = maxIdle
	b.config.MaxIdleConnsPerHost = maxIdlePerHost
	b.config.MaxConnsPerHost = maxPerHost
	return b
}

// WithTimeouts configures various timeout settings
func (b *HTTPClientBuilder) WithTimeouts(dial, tlsHandshake, idleConn, expectContinue time.Duration) *HTTPClientBuilder {
	b.config.DialTimeout = dial
	b.config.TLSHandshakeTimeout = tlsHandshake
	b.config.IdleConnTimeout = idleConn
	b.config.ExpectContinueTimeout = expectContinue
	return b
}

// WithKeepAlive sets the keep-alive duration
func (b *HTTPClientBuilder) WithKeepAlive(keepAlive time.Duration) *HTTPClientBuilder {
	b.config.KeepAlive = keepAlive
	return b
}

// Build creates the HTTP client with the configured settings
func (b *HTTPClientBuilder) Build() (*http.Client, error) {
	return NewHTTPClient(b.config, b.logger)
}

// HTTPClientTransport wraps an HTTP client to add common request modifications
type HTTPClientTransport struct {
	client         *http.Client
	defaultHeaders map[string]string
	logger         zerolog.Logger
}

// NewHTTPClientTransport creates a transport wrapper around an HTTP client
func NewHTTPClientTransport(client *http.Client, defaultHeaders map[string]string, logger zerolog.Logger) *HTTPClientTransport {
	return &HTTPClientTransport{
		client:         client,
		defaultHeaders: defaultHeaders,
		logger:         logger,
	}
}

// Do executes an HTTP request with default headers applied
func (t *HTTPClientTransport) Do(req *http.Request) (*http.Response, error) {
	// Apply default headers
	for key, value := range t.defaultHeaders {
		if req.Header.Get(key) == "" {
			req.Header.Set(key, value)
		}
	}

	// Log request (debug level)
	t.logger.Debug().
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Msg("Executing HTTP request")

	start := time.Now()
	resp, err := t.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		t.logger.Error().
			Err(err).
			Str("method", req.Method).
			Str("url", req.URL.String()).
			Dur("duration", duration).
			Msg("HTTP request failed")
		return nil, WrapError(err, "HTTP request failed")
	}

	t.logger.Debug().
		Int("status_code", resp.StatusCode).
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Dur("duration", duration).
		Msg("HTTP request completed")

	return resp, nil
}

// GetClient returns the underlying HTTP client
func (t *HTTPClientTransport) GetClient() *http.Client {
	return t.client
}

// HTTPClientFactory provides methods to create common HTTP client configurations
type HTTPClientFactory struct {
	logger zerolog.Logger
}

// NewHTTPClientFactory creates a new HTTP client factory
func NewHTTPClientFactory(logger zerolog.Logger) *HTTPClientFactory {
	return &HTTPClientFactory{logger: logger}
}

// CreateDiscordClient creates an HTTP client optimized for Discord webhook calls
func (f *HTTPClientFactory) CreateDiscordClient(timeout time.Duration) (*http.Client, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithUserAgent("MonsterInc Discord Bot/1.0").
		WithFollowRedirects(true).
		WithMaxRedirects(3).
		Build()
}

// CreateMonitorClient creates an HTTP client optimized for file monitoring
func (f *HTTPClientFactory) CreateMonitorClient(timeout time.Duration, insecureSkipVerify bool) (*http.Client, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithInsecureSkipVerify(insecureSkipVerify).
		WithUserAgent("MonsterInc Monitor/1.0").
		WithFollowRedirects(true).
		WithMaxRedirects(5).
		WithConnectionPooling(50, 10, 0).
		Build()
}

// CreateCrawlerClient creates an HTTP client optimized for web crawling
func (f *HTTPClientFactory) CreateCrawlerClient(timeout time.Duration, proxy string, customHeaders map[string]string) (*http.Client, error) {
	builder := NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithUserAgent("MonsterInc Crawler/1.0").
		WithFollowRedirects(true).
		WithMaxRedirects(10).
		WithConnectionPooling(100, 20, 0)

	if proxy != "" {
		builder = builder.WithProxy(proxy)
	}

	if len(customHeaders) > 0 {
		builder = builder.WithCustomHeaders(customHeaders)
	}

	return builder.Build()
}

// CreateHTTPXClient creates an HTTP client compatible with httpx runner requirements
func (f *HTTPClientFactory) CreateHTTPXClient(timeout time.Duration, proxy string, followRedirects bool, maxRedirects int) (*http.Client, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithProxy(proxy).
		WithFollowRedirects(followRedirects).
		WithMaxRedirects(maxRedirects).
		WithUserAgent("MonsterInc HTTPX/1.0").
		WithConnectionPooling(200, 50, 0).
		Build()
}

// CreateBasicClient creates a basic HTTP client with minimal configuration
func (f *HTTPClientFactory) CreateBasicClient(timeout time.Duration) (*http.Client, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		Build()
}

// ValidateHTTPClientConfig validates an HTTP client configuration
func ValidateHTTPClientConfig(config HTTPClientConfig) error {
	var collector ErrorCollector

	if config.Timeout <= 0 {
		collector.Add(NewValidationError("timeout", config.Timeout, "must be positive"))
	}

	if config.MaxRedirects < 0 {
		collector.Add(NewValidationError("max_redirects", config.MaxRedirects, "must be non-negative"))
	}

	if config.Proxy != "" {
		if _, err := url.Parse(config.Proxy); err != nil {
			collector.Add(NewValidationError("proxy", config.Proxy, "must be a valid URL"))
		}
	}

	if config.MaxIdleConns < 0 {
		collector.Add(NewValidationError("max_idle_conns", config.MaxIdleConns, "must be non-negative"))
	}

	if config.MaxIdleConnsPerHost < 0 {
		collector.Add(NewValidationError("max_idle_conns_per_host", config.MaxIdleConnsPerHost, "must be non-negative"))
	}

	if config.MaxConnsPerHost < 0 {
		collector.Add(NewValidationError("max_conns_per_host", config.MaxConnsPerHost, "must be non-negative"))
	}

	return collector.Error()
}
