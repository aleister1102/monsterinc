package scanner

import (
	"context"
	"fmt"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/rs/zerolog"
)

// CrawlerManager manages a singleton crawler instance for reuse across batches
type CrawlerManager struct {
	logger          zerolog.Logger
	crawlerInstance *crawler.Crawler
	mutex           sync.RWMutex
	initialized     bool
	lastConfig      *config.CrawlerConfig
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
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// If crawler doesn't exist or config has changed significantly, create new one
	if !cm.initialized || cm.needsRecreation(cfg) {
		if cm.crawlerInstance != nil {
			cm.logger.Info().Msg("Recreating crawler due to config changes")
			cm.crawlerInstance = nil
		}

		cm.logger.Info().Msg("Creating new singleton crawler instance")
		newCrawler, err := crawler.NewCrawler(cfg, cm.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create crawler: %w", err)
		}

		cm.crawlerInstance = newCrawler
		cm.lastConfig = cfg
		cm.initialized = true
		cm.logger.Info().Msg("Singleton crawler instance created successfully")
	} else {
		cm.logger.Debug().Msg("Reusing existing crawler instance")
	}

	return cm.crawlerInstance, nil
}

// needsRecreation checks if the crawler needs to be recreated due to config changes
func (cm *CrawlerManager) needsRecreation(newCfg *config.CrawlerConfig) bool {
	if cm.lastConfig == nil {
		return true
	}

	// Check critical config changes that require crawler recreation
	if cm.lastConfig.MaxConcurrentRequests != newCfg.MaxConcurrentRequests ||
		cm.lastConfig.RequestTimeoutSecs != newCfg.RequestTimeoutSecs ||
		cm.lastConfig.MaxDepth != newCfg.MaxDepth ||
		cm.lastConfig.UserAgent != newCfg.UserAgent ||
		cm.lastConfig.InsecureSkipTLSVerify != newCfg.InsecureSkipTLSVerify {
		return true
	}

	return false
}

// ExecuteCrawlerBatch executes a single batch using the managed crawler
func (cm *CrawlerManager) ExecuteCrawlerBatch(
	ctx context.Context,
	cfg *config.CrawlerConfig,
	seedURLs []string,
	sessionID string,
	progressDisplay *common.ProgressDisplayManager,
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

	// Update progress
	if progressDisplay != nil {
		progressDisplay.UpdateScanProgress(1, 5, "Crawler", fmt.Sprintf("Running crawler batch with %d seed URLs\n", len(seedURLs)))
	}

	cm.logger.Info().
		Int("seed_count", len(seedURLs)).
		Str("session_id", sessionID).
		Msg("Running crawler batch")

	// Execute crawler
	discoveredURLs, err := cm.runCrawlerBatch(ctx, crawlerInstance, seedURLs)
	if err != nil {
		return nil, fmt.Errorf("crawler batch execution failed: %w", err)
	}

	result.DiscoveredURLs = discoveredURLs

	// Update progress
	if progressDisplay != nil {
		progressDisplay.UpdateScanProgress(1, 5, "Crawler Complete", fmt.Sprintf("Crawler batch completed: %d URLs discovered", len(discoveredURLs)))
	}

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
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cm.crawlerInstance != nil {
		cm.crawlerInstance.DisableAutoCalibrate()
		cm.logger.Info().Msg("Auto-calibrate disabled for preprocessed URLs")
	}
}

// runCrawlerBatch runs a single crawler batch and returns discovered URLs
func (cm *CrawlerManager) runCrawlerBatch(ctx context.Context, crawlerInstance *crawler.Crawler, seedURLs []string) ([]string, error) {
	// Run crawler with context
	crawlerInstance.RunBatch(ctx, seedURLs)

	// Get discovered URLs
	discoveredURLs := crawlerInstance.GetDiscoveredURLs()

	return discoveredURLs, nil
}

// Shutdown gracefully shuts down the managed crawler
func (cm *CrawlerManager) Shutdown() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.crawlerInstance != nil {
		cm.logger.Info().Msg("Shutting down singleton crawler instance")
		cm.crawlerInstance.Stop()
		cm.crawlerInstance.EnsureFullShutdown()
		cm.crawlerInstance = nil
		cm.initialized = false
		cm.logger.Info().Msg("Singleton crawler instance shutdown complete")
	}
}

// CrawlerBatchResult contains the results from a crawler batch execution
type CrawlerBatchResult struct {
	DiscoveredURLs  []string
	CrawlerInstance *crawler.Crawler
	Error           error
}
