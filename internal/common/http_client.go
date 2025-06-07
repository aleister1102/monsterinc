package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

// HTTPClient wraps net/http.Client to provide compatibility with the existing interface
type HTTPClient struct {
	client       *http.Client
	config       HTTPClientConfig
	logger       zerolog.Logger
	retryHandler *RetryHandler
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
	}, nil
}

// Do performs an HTTP request using net/http
func (c *HTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
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

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, WrapError(err, "failed to read response body")
	}

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

// DoWithRetry performs an HTTP request with retry logic if retry handler is configured
func (c *HTTPClient) DoWithRetry(req *HTTPRequest) (*HTTPResponse, error) {
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
