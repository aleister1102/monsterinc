package comet

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

// ErrNotModified is returned when content has not been modified (HTTP 304).
var ErrNotModified = NewError("content not modified")

// HTTPClient wraps net/http.Client to provide compatibility with the existing interface
type HTTPClient struct {
	client       *http.Client
	config       HTTPClientConfig
	logger       zerolog.Logger
	retryHandler *RetryHandler
	bufferPool   sync.Pool
}

// NewHTTPClient creates a new HTTP client with the given configuration using net/http
func NewHTTPClient(config HTTPClientConfig, logger zerolog.Logger) (*HTTPClient, error) {
	// Create custom transport
	transport := &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	// Configure HTTP/2 support
	if config.EnableHTTP2 {
		if err := http2.ConfigureTransport(transport); err != nil {
			logger.Warn().Err(err).Msg("Failed to configure HTTP/2, falling back to HTTP/1.1")
		} else {
			logger.Debug().Msg("HTTP/2 support enabled")
		}
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

	// Configure redirect handling
	if !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if config.MaxRedirects > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", config.MaxRedirects)
			}
			return nil
		}
	}

	logger.Debug().
		Dur("timeout", config.Timeout).
		Bool("insecure_skip_verify", config.InsecureSkipVerify).
		Bool("follow_redirects", config.FollowRedirects).
		Int("max_redirects", config.MaxRedirects).
		Bool("http2_enabled", config.EnableHTTP2).
		Msg("HTTP client created")

	return &HTTPClient{
		client: client,
		config: config,
		logger: logger,
		bufferPool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate a buffer of a reasonable default size.
				// This avoids many small allocations. 32KB is a common choice.
				b := make([]byte, 32*1024)
				return &b
			},
		},
	}, nil
}

// Do performs an HTTP request, with retries if a retry handler is configured.
func (c *HTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
	// If a retry handler is configured, use it
	if c.retryHandler != nil {
		ctx := req.Context
		if ctx == nil {
			ctx = context.Background()
		}
		return c.retryHandler.DoWithRetry(ctx, c.do, req)
	}

	// Otherwise, perform a single request
	return c.do(req)
}

// do performs the actual HTTP request. It's an internal method used by Do.
func (c *HTTPClient) do(req *HTTPRequest) (*HTTPResponse, error) {
	// Create net/http request
	var body io.Reader
	if req.Body != nil {
		body = req.Body
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, body)
	if err != nil {
		return nil, WrapError(err, "failed to create HTTP request")
	}

	// Set context if provided
	if req.Context != nil {
		httpReq = httpReq.WithContext(req.Context)
	}

	// Set custom headers from config first (default headers)
	for key, value := range c.config.CustomHeaders {
		httpReq.Header.Set(key, value)
	}

	// Set request-specific headers (these can override defaults)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set user agent (ensure it's always set)
	if c.config.UserAgent != "" {
		httpReq.Header.Set("User-Agent", c.config.UserAgent)
	}

	// Ensure Accept header is set if not provided
	if httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "*/*")
	}

	// Perform request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, WrapError(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	// Use a buffer from the pool to read the response body
	bufPtr := c.bufferPool.Get().(*[]byte)
	defer c.bufferPool.Put(bufPtr)
	buf := bytes.NewBuffer((*bufPtr)[:0]) // Use the buffer's capacity, but start with length 0

	// Copy the response body into the buffer
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, WrapError(err, "failed to read response body")
	}

	// Create a copy of the bytes to return, so the buffer can be safely reused.
	bodyBytes := make([]byte, buf.Len())
	copy(bodyBytes, buf.Bytes())

	// Convert response
	httpResp := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       bodyBytes,
	}

	// Copy headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			httpResp.Headers[key] = values[0] // Take first value if multiple
		}
	}

	return httpResp, nil
}

// FetchContentInput holds parameters for FetchContent.
type FetchContentInput struct {
	URL                  string
	PreviousETag         string
	PreviousLastModified string
	Context              context.Context
	BypassCache          bool // When true, skips conditional headers to force fresh content
}

// FetchContentResult holds results from FetchContent.
type FetchContentResult struct {
	Content        []byte
	ContentType    string
	ETag           string
	LastModified   string
	HTTPStatusCode int
}

// FetchContent fetches the content of a file from the given URL with support for conditional GETs.
func (c *HTTPClient) FetchContent(input FetchContentInput) (*FetchContentResult, error) {
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Prepare request
	headers := make(map[string]string)

	// Add conditional headers if previous values are available and not bypassing cache
	if !input.BypassCache {
		if input.PreviousETag != "" {
			headers["If-None-Match"] = input.PreviousETag
		}
		if input.PreviousLastModified != "" {
			headers["If-Modified-Since"] = input.PreviousLastModified
		}
	}

	// Force fresh content when bypassing cache
	if input.BypassCache {
		headers["Cache-Control"] = "no-cache, no-store, must-revalidate"
		headers["Pragma"] = "no-cache"
		headers["Expires"] = "0"
	}

	req := &HTTPRequest{
		URL:     input.URL,
		Method:  "GET",
		Headers: headers,
		Context: ctx,
	}

	resp, err := c.Do(req) // Use the main Do method which handles retries automatically
	if err != nil {
		// Don't wrap network errors from Do, as they are already wrapped
		c.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to execute HTTP request")
		return nil, err
	}

	result := &FetchContentResult{
		ETag:           resp.Headers["Etag"],
		LastModified:   resp.Headers["Last-Modified"],
		ContentType:    resp.Headers["Content-Type"],
		HTTPStatusCode: resp.StatusCode,
	}

	if resp.StatusCode == 304 { // Not Modified
		c.logger.Debug().Str("url", input.URL).Msg("Content not modified (304)")
		return result, ErrNotModified
	}

	if resp.StatusCode != 200 {
		c.logger.Warn().Str("url", input.URL).Int("status_code", resp.StatusCode).Msg("Received non-OK HTTP status")
		errorBody := resp.Body
		if len(errorBody) > 1024 {
			errorBody = errorBody[:1024]
		}
		result.Content = errorBody
		return result, NewHTTPErrorWithURL(resp.StatusCode, string(errorBody), input.URL)
	}

	// Check content size limit from the main client config
	if c.config.MaxContentSize > 0 && len(resp.Body) > c.config.MaxContentSize {
		c.logger.Warn().
			Str("url", input.URL).
			Int("content_size", len(resp.Body)).
			Int("max_content_size", c.config.MaxContentSize).
			Msg("Content size exceeds limit, truncating")
		result.Content = resp.Body[:c.config.MaxContentSize]
	} else {
		result.Content = resp.Body
	}

	c.logger.Debug().
		Str("url", input.URL).
		Int("content_size", len(result.Content)).
		Str("content_type", result.ContentType).
		Msg("Successfully fetched content")

	return result, nil
}
