package monitor

import (
	"errors"
	"fmt"
	"io"
	"github.com/aleister1102/monsterinc/internal/config"
	"net/http"

	"github.com/rs/zerolog"
)

var ErrNotModified = errors.New("content not modified")

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
		return nil, fmt.Errorf("creating request for %s: %w", input.URL, err)
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
		return nil, fmt.Errorf("fetching %s: %w", input.URL, err)
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
		return result, fmt.Errorf("HTTP status %d for %s: %s", resp.StatusCode, input.URL, string(bodyBytes))
	}

	// TODO: FR10 - Handle very large files. Consider streaming and hash calculation on the fly.
	// For now, read full body. This needs to be robust for large files.
	// Consider max content length from config if available and applicable here.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		f.logger.Error().Err(err).Str("url", input.URL).Msg("Failed to read response body")
		return nil, fmt.Errorf("reading body for %s: %w", input.URL, err)
	}
	result.Content = bodyBytes

	f.logger.Debug().Str("url", input.URL).Str("content_type", result.ContentType).Int("size", len(result.Content)).Msg("File content fetched successfully")
	return result, nil
}

// TODO: Task 2.2 - Implement conditional fetching using ETag and Last-Modified headers.
