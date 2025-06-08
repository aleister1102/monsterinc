package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

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

// SendDiscordNotification sends a notification to Discord webhook
func (c *HTTPClient) SendDiscordNotification(ctx context.Context, webhookURL string, payload interface{}, filePath string) error {
	if filePath == "" {
		// Send JSON payload only
		return c.sendDiscordJSON(ctx, webhookURL, payload)
	}

	// Send multipart form-data with file attachment
	return c.sendDiscordMultipart(ctx, webhookURL, payload, filePath)
}

// sendDiscordJSON sends JSON payload to Discord webhook
func (c *HTTPClient) sendDiscordJSON(ctx context.Context, webhookURL string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return WrapError(err, "failed to marshal Discord payload")
	}

	req := &HTTPRequest{
		URL:    webhookURL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:    bytes.NewReader(jsonData),
		Context: ctx,
	}

	resp, err := c.Do(req)
	if err != nil {
		return WrapError(err, "failed to send Discord notification")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	c.logger.Debug().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
	return nil
}

// sendDiscordMultipart sends multipart form-data to Discord webhook with file attachment
func (c *HTTPClient) sendDiscordMultipart(ctx context.Context, webhookURL string, payload interface{}, filePath string) error {
	// Check file size before attempting to send
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return WrapError(err, "failed to get file info for Discord attachment")
	}

	const maxDiscordFileSize = 10 * 1024 * 1024 // 10MB in bytes
	if fileInfo.Size() > maxDiscordFileSize {
		return fmt.Errorf("file size %d bytes exceeds Discord limit of %d bytes (%.2f MB > 10 MB)",
			fileInfo.Size(), maxDiscordFileSize, float64(fileInfo.Size())/(1024*1024))
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add JSON payload as form field
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return WrapError(err, "failed to marshal Discord payload")
	}

	if err := writer.WriteField("payload_json", string(jsonData)); err != nil {
		return WrapError(err, "failed to write payload_json field")
	}

	// Add file attachment
	file, err := os.Open(filePath)
	if err != nil {
		return WrapError(err, "failed to open file for Discord attachment")
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return WrapError(err, "failed to create form file")
	}

	if _, err := io.Copy(part, file); err != nil {
		return WrapError(err, "failed to copy file content")
	}

	if err := writer.Close(); err != nil {
		return WrapError(err, "failed to close multipart writer")
	}

	req := &HTTPRequest{
		URL:    webhookURL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": writer.FormDataContentType(),
		},
		Body:    &buf,
		Context: ctx,
	}

	resp, err := c.Do(req)
	if err != nil {
		return WrapError(err, "failed to send Discord notification with attachment")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	c.logger.Debug().Int("status_code", resp.StatusCode).Str("file", filePath).Float64("file_size_mb", float64(fileInfo.Size())/(1024*1024)).Msg("Discord notification with attachment sent successfully")
	return nil
}
