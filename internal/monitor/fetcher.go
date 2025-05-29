package monitor

import (
	"fmt"
	"io"
	"monsterinc/internal/common"
	"monsterinc/internal/config"
	"net/http"

	"github.com/rs/zerolog"
)

var ErrNotModified = common.NewError("content not modified")

// Fetcher handles fetching file content from URLs.
// It might be a struct with dependencies like http.Client and config, or just a collection of functions.
type Fetcher struct {
	httpClient *http.Client
	logger     zerolog.Logger
	cfg        *config.MonitorConfig // For timeout, user-agent, etc.
}

// NewFetcher creates a new Fetcher.
func NewFetcher(client *http.Client, logger zerolog.Logger, cfg *config.MonitorConfig) *Fetcher {
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
		return nil, common.WrapError(err, fmt.Sprintf("creating request for %s", input.URL))
	}

	// Add conditional headers if previous values are available
	if input.PreviousETag != "" {
		req.Header.Set("If-None-Match", input.PreviousETag)
	}
	if input.PreviousLastModified != "" {
		req.Header.Set("If-Modified-Since", input.PreviousLastModified)
	}

	// Use httpClient with timeout from the MonitoringService (derived from config)
	resp, err := f.httpClient.Do(req)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to execute HTTP request")
		return nil, common.NewNetworkError(input.URL, "HTTP request failed", err)
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
		// Read body for non-OK for potential error messages, but limit size
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // Read up to 1KB of the body
		result.Content = bodyBytes                                  // Include partial body in error case if needed for context
		return result, common.NewHTTPErrorWithURL(resp.StatusCode, string(bodyBytes), input.URL)
	}

	if resp.ContentLength > 0 && resp.ContentLength > int64(f.cfg.MaxContentSize) {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", resp.ContentLength, f.cfg.MaxContentSize)
	}

	// TODO: Handle very large files with streaming and hash calculation
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to read response body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check actual body size
	if len(bodyBytes) > f.cfg.MaxContentSize {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", len(bodyBytes), f.cfg.MaxContentSize)
	}

	result.Content = bodyBytes

	f.logger.Debug().Str("url", input.URL).Str("content_type", result.ContentType).Int("size", len(result.Content)).Msg("File content fetched successfully")
	return result, nil
}

// TODO: Task 2.2 - Implement conditional fetching using ETag and Last-Modified headers.
