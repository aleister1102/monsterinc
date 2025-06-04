package common

import (
	"crypto/tls"
	"fmt"
	"io"
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

// ErrNotModified is returned when content has not been modified (HTTP 304).
var ErrNotModified = NewError("content not modified")

// Fetcher handles fetching file content from URLs.
type Fetcher struct {
	httpClient *http.Client
	logger     zerolog.Logger
	cfg        *HTTPClientFetcherConfig // Renamed from config.MonitorConfig for clarity within http_client
}

// HTTPClientFetcherConfig holds configuration specific to the Fetcher's needs,
// currently just MaxContentSize. This can be expanded if more monitor-specific
// configurations are needed by the Fetcher that are not part of general HTTPClientConfig.
type HTTPClientFetcherConfig struct {
	MaxContentSize int
	// Potentially add other fetcher-specific configs here like UserAgent if different from client's default
}

// NewFetcher creates a new Fetcher.
// It now takes HTTPClientFetcherConfig.
func NewFetcher(client *http.Client, logger zerolog.Logger, cfg *HTTPClientFetcherConfig) *Fetcher {
	return &Fetcher{
		httpClient: client,
		logger:     logger.With().Str("component", "Fetcher").Logger(),
		cfg:        cfg,
	}
}

// FetchFileContentInput holds parameters for FetchFileContent.
type FetchFileContentInput struct {
	URL                  string
	PreviousETag         string
	PreviousLastModified string
}

// FetchFileContentResult holds results from FetchFileContent.
type FetchFileContentResult struct {
	Content        []byte
	ContentType    string
	ETag           string
	LastModified   string
	HTTPStatusCode int
}

// FetchFileContent fetches the content of a file from the given URL with support for conditional GETs.
// It returns the content, content type, new ETag, new LastModified, and any error encountered.
// If the server returns 304 Not Modified, it returns ErrNotModified.
func (f *Fetcher) FetchFileContent(input FetchFileContentInput) (*FetchFileContentResult, error) {
	req, err := http.NewRequest("GET", input.URL, nil)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to create new HTTP request")
		return nil, WrapError(err, fmt.Sprintf("creating request for %s", input.URL))
	}

	// Add conditional headers if previous values are available
	if input.PreviousETag != "" {
		req.Header.Set("If-None-Match", input.PreviousETag)
	}
	if input.PreviousLastModified != "" {
		req.Header.Set("If-Modified-Since", input.PreviousLastModified)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to execute HTTP request")
		return nil, NewNetworkError(input.URL, "HTTP request failed", err)
	}
	defer resp.Body.Close()

	result := &FetchFileContentResult{
		ETag:           resp.Header.Get("ETag"),
		LastModified:   resp.Header.Get("Last-Modified"),
		ContentType:    resp.Header.Get("Content-Type"),
		HTTPStatusCode: resp.StatusCode,
	}

	if resp.StatusCode == http.StatusNotModified {
		f.logger.Debug().Str("url", input.URL).Msg("Content not modified (304)")
		return result, ErrNotModified
	}

	if resp.StatusCode != http.StatusOK {
		f.logger.Warn().Str("url", input.URL).Int("status_code", resp.StatusCode).Msg("Received non-OK HTTP status")
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // Read up to 1KB
		result.Content = bodyBytes
		return result, NewHTTPErrorWithURL(resp.StatusCode, string(bodyBytes), input.URL)
	}

	if resp.ContentLength > 0 && resp.ContentLength > int64(f.cfg.MaxContentSize) {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", resp.ContentLength, f.cfg.MaxContentSize)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to read response body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(bodyBytes) > f.cfg.MaxContentSize {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", len(bodyBytes), f.cfg.MaxContentSize)
	}

	result.Content = bodyBytes

	f.logger.Debug().Str("url", input.URL).Str("content_type", result.ContentType).Int("size", len(result.Content)).Msg("File content fetched successfully")
	return result, nil
}
