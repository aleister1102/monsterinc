package common

import (
	"context"

	"github.com/rs/zerolog"
)

// ErrNotModified is returned when content has not been modified (HTTP 304).
var ErrNotModified = NewError("content not modified")

// Fetcher handles fetching file content from URLs using net/http.
type Fetcher struct {
	httpClient *HTTPClient
	logger     zerolog.Logger
	cfg        *HTTPClientFetcherConfig
}

// HTTPClientFetcherConfig holds configuration specific to the Fetcher's needs
type HTTPClientFetcherConfig struct {
	MaxContentSize int
}

// NewFetcher creates a new Fetcher using HTTPClient.
func NewFetcher(client *HTTPClient, logger zerolog.Logger, cfg *HTTPClientFetcherConfig) *Fetcher {
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
		ETag:           resp.Headers["Etag"],
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

	// Check content size limit
	if f.cfg.MaxContentSize > 0 && len(resp.Body) > f.cfg.MaxContentSize {
		f.logger.Warn().
			Str("url", input.URL).
			Int("content_size", len(resp.Body)).
			Int("max_content_size", f.cfg.MaxContentSize).
			Msg("Content size exceeds limit, truncating")
		result.Content = resp.Body[:f.cfg.MaxContentSize]
	} else {
		result.Content = resp.Body
	}

	f.logger.Debug().
		Str("url", input.URL).
		Int("content_size", len(result.Content)).
		Str("content_type", result.ContentType).
		Msg("Successfully fetched content")

	return result, nil
}
