package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
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

// DefaultHTTPClientConfig returns the default HTTP client configuration
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:               30 * time.Second,
		InsecureSkipVerify:    true,
		FollowRedirects:       true,
		MaxRedirects:          10,
		UserAgent:             "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // 0 means no limit
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialTimeout:           10 * time.Second,
		KeepAlive:             30 * time.Second,
		CustomHeaders: map[string]string{
			"Accept":                    "*/*",
			"Accept-Language":           "en-US,en;q=0.9",
			"Accept-Encoding":           "gzip, deflate",
			"Connection":                "keep-alive",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
		},
	}
}

// FastHTTPClient wraps fasthttp.Client to provide compatibility with the existing interface
type FastHTTPClient struct {
	client       *fasthttp.Client
	config       HTTPClientConfig
	logger       zerolog.Logger
	retryHandler *RetryHandler
}

// NewHTTPClient creates a new HTTP client with the given configuration using fasthttp
func NewHTTPClient(config HTTPClientConfig, logger zerolog.Logger) (*FastHTTPClient, error) {
	client := &fasthttp.Client{
		ReadTimeout:         config.Timeout,
		WriteTimeout:        config.Timeout,
		MaxIdleConnDuration: config.IdleConnTimeout,
	}

	// Configure TLS
	client.TLSConfig = &tls.Config{
		InsecureSkipVerify: config.InsecureSkipVerify,
	}

	// Configure dialer
	client.Dial = func(addr string) (net.Conn, error) {
		return fasthttp.DialTimeout(addr, config.DialTimeout)
	}

	// Configure connection limits
	client.MaxConnsPerHost = config.MaxConnsPerHost
	if client.MaxConnsPerHost == 0 {
		client.MaxConnsPerHost = 512 // fasthttp default
	}

	// Configure proxy if specified
	if config.Proxy != "" {
		_, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, WrapError(err, "failed to parse proxy URL")
		}
		// Note: fasthttp has limited proxy support out of the box
		// For full proxy support, consider using fasthttp with proxy packages
		logger.Info().Str("proxy", config.Proxy).Msg("HTTP client configured with proxy (proxy support limited)")
	}

	logger.Debug().
		Dur("timeout", config.Timeout).
		Bool("insecure_skip_verify", config.InsecureSkipVerify).
		Bool("follow_redirects", config.FollowRedirects).
		Int("max_redirects", config.MaxRedirects).
		Msg("FastHTTP client created")

	return &FastHTTPClient{
		client: client,
		config: config,
		logger: logger,
	}, nil
}

// Do performs an HTTP request using fasthttp
func (c *FastHTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
	// Create fasthttp request and response
	fastReq := fasthttp.AcquireRequest()
	fastResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(fastReq)
	defer fasthttp.ReleaseResponse(fastResp)

	// Set URL and method
	fastReq.SetRequestURI(req.URL)
	fastReq.Header.SetMethod(req.Method)

	// Set custom headers from config first (default headers)
	for key, value := range c.config.CustomHeaders {
		fastReq.Header.Set(key, value)
	}

	// Set request-specific headers (these can override defaults)
	for key, value := range req.Headers {
		fastReq.Header.Set(key, value)
	}

	// Set user agent (ensure it's always set)
	if c.config.UserAgent != "" {
		fastReq.Header.SetUserAgent(c.config.UserAgent)
	}

	// Ensure Accept header is set if not provided
	if fastReq.Header.Peek("Accept") == nil {
		fastReq.Header.Set("Accept", "*/*")
	}

	// Set body if provided
	if req.Body != nil {
		fastReq.SetBodyStream(req.Body, -1)
	}

	// Perform request with redirect handling
	var err error
	if c.config.FollowRedirects {
		err = c.client.DoRedirects(fastReq, fastResp, c.config.MaxRedirects)
	} else {
		err = c.client.Do(fastReq, fastResp)
	}

	if err != nil {
		return nil, WrapError(err, "fasthttp request failed")
	}

	// Convert response
	resp := &HTTPResponse{
		StatusCode: fastResp.StatusCode(),
		Headers:    make(map[string]string),
		Body:       append([]byte(nil), fastResp.Body()...),
	}

	// Copy headers
	fastResp.Header.VisitAll(func(key, value []byte) {
		resp.Headers[string(key)] = string(value)
	})

	return resp, nil
}

// SetRetryHandler sets the retry handler for this client
func (c *FastHTTPClient) SetRetryHandler(retryHandler *RetryHandler) {
	c.retryHandler = retryHandler
}

// DoWithRetry performs an HTTP request with retry logic if retry handler is configured
func (c *FastHTTPClient) DoWithRetry(req *HTTPRequest) (*HTTPResponse, error) {
	if c.retryHandler == nil {
		// Fallback to regular Do method if no retry handler
		return c.Do(req)
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}

	return c.retryHandler.DoWithRetry(ctx, c, req)
}

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    io.Reader
	Context context.Context
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
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
func (b *HTTPClientBuilder) Build() (*FastHTTPClient, error) {
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
func (f *HTTPClientFactory) CreateDiscordClient(timeout time.Duration) (*FastHTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithUserAgent("MonsterInc Discord Bot/1.0").
		WithFollowRedirects(true).
		WithMaxRedirects(3).
		Build()
}

// CreateMonitorClient creates an HTTP client optimized for file monitoring
func (f *HTTPClientFactory) CreateMonitorClient(timeout time.Duration, insecureSkipVerify bool) (*FastHTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithInsecureSkipVerify(insecureSkipVerify).
		WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36").
		WithFollowRedirects(true).
		WithMaxRedirects(5).
		WithConnectionPooling(50, 10, 0).
		Build()
}

// CreateCrawlerClient creates an HTTP client optimized for web crawling
func (f *HTTPClientFactory) CreateCrawlerClient(timeout time.Duration, proxy string, customHeaders map[string]string, insecureSkipVerify bool) (*FastHTTPClient, error) {
	builder := NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36").
		WithFollowRedirects(true).
		WithMaxRedirects(10).
		WithConnectionPooling(100, 20, 0).
		WithInsecureSkipVerify(insecureSkipVerify)

	if proxy != "" {
		builder = builder.WithProxy(proxy)
	}

	if len(customHeaders) > 0 {
		builder = builder.WithCustomHeaders(customHeaders)
	}

	return builder.Build()
}

// CreateCrawlerClientWithRetry creates an HTTP client optimized for web crawling with retry support
func (f *HTTPClientFactory) CreateCrawlerClientWithRetry(timeout time.Duration, proxy string, customHeaders map[string]string, insecureSkipVerify bool, retryConfig RetryHandlerConfig) (*FastHTTPClient, error) {
	client, err := f.CreateCrawlerClient(timeout, proxy, customHeaders, insecureSkipVerify)
	if err != nil {
		return nil, err
	}

	// Configure retry handler if retries are enabled
	if retryConfig.MaxRetries > 0 {
		retryHandler := NewRetryHandler(retryConfig, f.logger)
		client.SetRetryHandler(retryHandler)

		f.logger.Info().
			Int("max_retries", retryConfig.MaxRetries).
			Dur("base_delay", retryConfig.BaseDelay).
			Dur("max_delay", retryConfig.MaxDelay).
			Bool("enable_jitter", retryConfig.EnableJitter).
			Ints("retry_status_codes", retryConfig.RetryStatusCodes).
			Msg("Crawler HTTP client configured with retry handler")
	}

	return client, nil
}

// CreateHTTPXClient creates an HTTP client compatible with httpx runner requirements
func (f *HTTPClientFactory) CreateHTTPXClient(timeout time.Duration, proxy string, followRedirects bool, maxRedirects int) (*FastHTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		WithProxy(proxy).
		WithFollowRedirects(followRedirects).
		WithMaxRedirects(maxRedirects).
		WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36").
		WithConnectionPooling(200, 50, 0).
		Build()
}

// CreateBasicClient creates a basic HTTP client with minimal configuration
func (f *HTTPClientFactory) CreateBasicClient(timeout time.Duration) (*FastHTTPClient, error) {
	return NewHTTPClientBuilder(f.logger).
		WithTimeout(timeout).
		Build()
}

// ErrNotModified is returned when content has not been modified (HTTP 304).
var ErrNotModified = NewError("content not modified")

// Fetcher handles fetching file content from URLs using fasthttp.
type Fetcher struct {
	httpClient *FastHTTPClient
	logger     zerolog.Logger
	cfg        *HTTPClientFetcherConfig
}

// HTTPClientFetcherConfig holds configuration specific to the Fetcher's needs
type HTTPClientFetcherConfig struct {
	MaxContentSize int
}

// NewFetcher creates a new Fetcher using FastHTTPClient.
func NewFetcher(client *FastHTTPClient, logger zerolog.Logger, cfg *HTTPClientFetcherConfig) *Fetcher {
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
	Context              context.Context
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
func (f *Fetcher) FetchFileContent(input FetchFileContentInput) (*FetchFileContentResult, error) {
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Prepare request
	headers := make(map[string]string)

	// Add conditional headers if previous values are available
	if input.PreviousETag != "" {
		headers["If-None-Match"] = input.PreviousETag
	}
	if input.PreviousLastModified != "" {
		headers["If-Modified-Since"] = input.PreviousLastModified
	}

	req := &HTTPRequest{
		URL:     input.URL,
		Method:  "GET",
		Headers: headers,
		Context: ctx,
	}

	resp, err := f.httpClient.DoWithRetry(req)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to execute HTTP request")
		return nil, NewNetworkError(input.URL, "HTTP request failed", err)
	}

	result := &FetchFileContentResult{
		ETag:           resp.Headers["ETag"],
		LastModified:   resp.Headers["Last-Modified"],
		ContentType:    resp.Headers["Content-Type"],
		HTTPStatusCode: resp.StatusCode,
	}

	if resp.StatusCode == 304 { // Not Modified
		f.logger.Debug().Str("url", input.URL).Msg("Content not modified (304)")
		return result, ErrNotModified
	}

	if resp.StatusCode != 200 {
		f.logger.Warn().Str("url", input.URL).Int("status_code", resp.StatusCode).Msg("Received non-OK HTTP status")
		// Limit error body to 1KB
		errorBody := resp.Body
		if len(errorBody) > 1024 {
			errorBody = errorBody[:1024]
		}
		result.Content = errorBody
		return result, NewHTTPErrorWithURL(resp.StatusCode, string(errorBody), input.URL)
	}

	if len(resp.Body) > f.cfg.MaxContentSize {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", len(resp.Body), f.cfg.MaxContentSize)
	}

	result.Content = resp.Body

	f.logger.Debug().Str("url", input.URL).Str("content_type", result.ContentType).Int("size", len(result.Content)).Msg("File content fetched successfully")
	return result, nil
}
