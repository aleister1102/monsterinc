package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/rs/zerolog"
)

// CrawlerManager manages a singleton crawler instance for reuse across batches
type CrawlerManager struct {
	logger          zerolog.Logger
	crawlerInstance *crawler.Crawler
	mu              sync.RWMutex
}

// NewCrawlerManager creates a new crawler manager
func NewCrawlerManager(logger zerolog.Logger) *CrawlerManager {
	return &CrawlerManager{
		logger: logger.With().Str("component", "CrawlerManager").Logger(),
	}
}

// GetOrCreateCrawler returns the existing crawler instance or creates a new one if needed
// This ensures we reuse the same crawler across multiple batches
func (cm *CrawlerManager) GetOrCreateCrawler(cfg *config.CrawlerConfig) (*crawler.Crawler, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// If crawler doesn't exist, create new one
	if cm.crawlerInstance == nil {
		cm.logger.Info().Msg("Creating new singleton crawler instance")
		newCrawler, err := crawler.NewCrawler(cfg, nil, cm.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create crawler: %w", err)
		}

		cm.crawlerInstance = newCrawler
		cm.logger.Info().Msg("Singleton crawler instance created successfully")
	} else {
		cm.logger.Debug().Msg("Reusing existing crawler instance")
	}

	return cm.crawlerInstance, nil
}

// ExecuteCrawlerBatch executes a single batch using the managed crawler
func (cm *CrawlerManager) ExecuteCrawlerBatch(
	ctx context.Context,
	cfg *config.CrawlerConfig,
	seedURLs []string,
	sessionID string,
) (*CrawlerBatchResult, error) {
	// Get or create crawler instance
	crawlerInstance, err := cm.GetOrCreateCrawler(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get crawler instance: %w", err)
	}

	// Update config with current seed URLs for this batch
	batchConfig := *cfg // Copy config
	batchConfig.SeedURLs = seedURLs

	// Check if crawler needs to be updated with new seed URLs
	cm.updateCrawlerForBatch(crawlerInstance, seedURLs)

	result := &CrawlerBatchResult{
		CrawlerInstance: crawlerInstance,
	}

	// Run crawler if seed URLs provided
	if len(seedURLs) == 0 {
		cm.logger.Info().Str("session_id", sessionID).Msg("No seed URLs provided, skipping crawler")
		return result, nil
	}

	// Check for context cancellation
	if cancelled := common.CheckCancellation(ctx); cancelled.Cancelled {
		return nil, cancelled.Error
	}

	cm.logger.Info().
		Int("seed_count", len(seedURLs)).
		Str("session_id", sessionID).
		Msg("Running crawler batch")

	// Execute crawler with progress callback
	discoveredURLs, err := cm.runCrawlerBatchWithProgress(ctx, crawlerInstance, seedURLs)
	if err != nil {
		return nil, fmt.Errorf("crawler batch execution failed: %w", err)
	}

	result.DiscoveredURLs = discoveredURLs

	cm.logger.Info().
		Int("discovered_count", len(discoveredURLs)).
		Str("session_id", sessionID).
		Msg("Crawler batch completed")

	return result, nil
}

// updateCrawlerForBatch updates crawler with new seed URLs for the current batch
func (cm *CrawlerManager) updateCrawlerForBatch(crawlerInstance *crawler.Crawler, seedURLs []string) {
	// Reset discovered URLs for new batch
	crawlerInstance.ResetForNewBatch(seedURLs)
}

// DisableAutoCalibrateForPreprocessedURLs disables auto-calibrate when URLs are preprocessed
func (cm *CrawlerManager) DisableAutoCalibrateForPreprocessedURLs() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.crawlerInstance != nil {
		cm.crawlerInstance.DisableAutoCalibrate()
		cm.logger.Info().Msg("Auto-calibrate disabled for preprocessed URLs")
	}
}

// runCrawlerBatchWithProgress runs a single crawler batch with progress updates
func (cm *CrawlerManager) runCrawlerBatchWithProgress(ctx context.Context, crawlerInstance *crawler.Crawler, seedURLs []string) ([]string, error) {
	// Initial progress update

	// Start progress monitoring in background
	done := make(chan struct{})
	go cm.monitorCrawlerProgress(ctx, crawlerInstance, done)

	// Run crawler with context
	crawlerInstance.RunBatch(ctx, seedURLs)

	// Stop progress monitoring
	close(done)

	// Final progress update
	discoveredURLs := crawlerInstance.GetDiscoveredURLs()

	return discoveredURLs, nil
}

// monitorCrawlerProgress monitors crawler progress and updates display
func (cm *CrawlerManager) monitorCrawlerProgress(ctx context.Context, crawlerInstance *crawler.Crawler, done chan struct{}) {
	ticker := time.NewTicker(3 * time.Second) // Reduce frequency to avoid spam
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			// Get current stats from crawler
			discoveredCount := len(crawlerInstance.GetDiscoveredURLs())
			cm.logger.Info().Int("discovered_count", discoveredCount).Msg("Crawler progress")
		}
	}
}

// Shutdown gracefully shuts down the managed crawler
func (cm *CrawlerManager) Shutdown() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.crawlerInstance != nil {
		cm.logger.Info().Msg("Shutting down singleton crawler instance")
		cm.crawlerInstance.Stop()
		cm.crawlerInstance.EnsureFullShutdown()
		cm.crawlerInstance = nil
		cm.logger.Info().Msg("Singleton crawler instance shutdown complete")
	}
}

// CrawlerBatchResult contains the results from a crawler batch execution
type CrawlerBatchResult struct {
	DiscoveredURLs  []string
	CrawlerInstance *crawler.Crawler
	Error           error
}
