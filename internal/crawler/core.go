package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
)

// StatsCallback interface for notifying about crawler statistics
type StatsCallback interface {
	OnAssetsExtracted(count int64)
	OnURLProcessed(count int64)
	OnError(count int64)
}

// Crawler represents the web crawler instance with thread-safe operations
type Crawler struct {
	collector      *colly.Collector
	discoveredURLs map[string]bool
	// Track parent URL for each discovered URL
	urlParentMap   map[string]string // child URL -> parent URL
	mutex          sync.RWMutex
	requestTimeout time.Duration
	threads        int
	maxDepth       int
	seedURLs       []string
	totalVisited   int
	totalErrors    int
	crawlStartTime time.Time
	scope          *ScopeSettings

	logger zerolog.Logger
	config *config.CrawlerConfig
	ctx    context.Context
	// URL batching for improved performance
	urlQueue      chan string
	urlBatchSize  int
	batchWG       sync.WaitGroup
	batchShutdown chan struct{}
	// Extension map cache for fast string operations
	disallowedExtMap map[string]bool
	// URL pattern detector for auto-calibrate
	patternDetector *URLPatternDetector
	// Stats callback for monitoring
	statsCallback StatsCallback
}

// NewCrawler initializes a new Crawler based on the provided configuration
func NewCrawler(cfg *config.CrawlerConfig, notifier notifier.Notifier, appLogger zerolog.Logger) (*Crawler, error) {
	builder := NewCrawlerBuilder(appLogger).WithConfig(cfg).WithNotifier(notifier)
	return builder.Build()
}

// GetDiscoveredURLs returns a slice of all unique URLs discovered
func (cr *Crawler) GetDiscoveredURLs() []string {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	urls := make([]string, 0, len(cr.discoveredURLs))
	for url := range cr.discoveredURLs {
		urls = append(urls, url)
	}
	return urls
}

// TrackURLParent tracks the parent URL for a discovered URL
func (cr *Crawler) TrackURLParent(childURL, parentURL string) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()
	cr.urlParentMap[childURL] = parentURL
}

// GetRootTargetForDiscoveredURL returns the root target URL for a discovered URL
// by tracing back through the parent chain to find the original seed URL
func (cr *Crawler) GetRootTargetForDiscoveredURL(discoveredURL string) string {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	// Check if this URL is a seed URL
	for _, seed := range cr.seedURLs {
		if discoveredURL == seed {
			return seed
		}
	}

	// Trace back through parent chain
	currentURL := discoveredURL
	visited := make(map[string]bool) // Prevent infinite loops

	for !visited[currentURL] {
		visited[currentURL] = true

		parentURL, exists := cr.urlParentMap[currentURL]
		if !exists {
			break // No parent found
		}

		// Check if parent is a seed URL
		for _, seed := range cr.seedURLs {
			if parentURL == seed {
				return seed
			}
		}

		currentURL = parentURL
	}

	// Fallback to urlhandler logic
	return urlhandler.GetRootTargetForURL(discoveredURL, cr.seedURLs)
}

// DisableAutoCalibrate disables auto-calibrate pattern detection in crawler
// This is useful when URLs have already been preprocessed at Scanner level
func (cr *Crawler) DisableAutoCalibrate() {
	cr.logger.Debug().Msg("Auto-calibrate disabled - URLs preprocessed at Scanner level")
	// Set auto-calibrate to disabled in runtime config
	cr.config.AutoCalibrate.Enabled = false
}
