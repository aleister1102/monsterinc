package monitor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/monsterinc/httpclient"
	"github.com/monsterinc/limiter"

	"github.com/rs/zerolog"
)

// ContentProcessor handles processing of fetched file content, like hashing.
type ContentProcessor struct {
	config           *config.MonitorConfig
	logger           zerolog.Logger
	httpClient       *httpclient.HTTPClient
	resourceLimiter  *limiter.ResourceLimiter
	discoveredAssets []models.Asset
	mutex            sync.Mutex
	fetcher          *httpclient.Fetcher
}

// NewContentProcessor creates a new ContentProcessor.
func NewContentProcessor(
	cfg *config.MonitorConfig,
	appLogger zerolog.Logger,
	httpClient *httpclient.HTTPClient,
	resourceLimiter *limiter.ResourceLimiter,
) *ContentProcessor {
	fetcher := httpclient.NewFetcher(httpClient, appLogger, &httpclient.HTTPClientFetcherConfig{
		MaxContentSize: cfg.MaxContentSize,
	})
	return &ContentProcessor{
		config:           cfg,
		logger:           appLogger.With().Str("component", "ContentProcessor").Logger(),
		httpClient:       httpClient,
		resourceLimiter:  resourceLimiter,
		discoveredAssets: make([]models.Asset, 0),
		fetcher:          fetcher,
	}
}

// ProcessBatch processes a batch of URLs.
func (cp *ContentProcessor) ProcessBatch(ctx context.Context, batch []string, cycleID string) {
	// TODO: Implement the full logic for processing a batch of URLs.
	// This should involve fetching, content analysis, path extraction, and secret scanning.
	cp.logger.Info().
		Int("batch_size", len(batch)).
		Str("cycle_id", cycleID).
		Msg("Processing batch of URLs")
	// Placeholder implementation.
}

// GetDiscoveredAssets returns the assets discovered during processing.
func (cp *ContentProcessor) GetDiscoveredAssets() []models.Asset {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()
	assets := make([]models.Asset, len(cp.discoveredAssets))
	copy(assets, cp.discoveredAssets)
	return assets
}

// ProcessContent takes the file content and processes it.
// Currently, it calculates a SHA256 hash of the content.
// It returns a MonitoredFileUpdate struct with the new hash and other relevant info.
func (cp *ContentProcessor) ProcessContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	return &models.MonitoredFileUpdate{
		URL:         url,
		NewHash:     hashStr,
		ContentType: contentType,
		FetchedAt:   time.Now(),
		Content:     content, // Pass content through; service layer decides if it's stored
	}, nil
}
