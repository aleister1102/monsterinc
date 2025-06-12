package crawler

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/secretscanner"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
)

// StatsCallback interface for notifying about crawler statistics
type StatsCallback interface {
	OnAssetsExtracted(count int64)
	OnURLProcessed(count int64)
	OnError(count int64)
	OnSecretFound(count int64)
}

// Crawler represents the web crawler instance with thread-safe operations
type Crawler struct {
	collector      *colly.Collector
	discoveredURLs map[string]bool
	// Track parent URL for each discovered URL
	urlParentMap     map[string]string // child URL -> parent URL
	mutex            sync.RWMutex
	userAgent        string
	requestTimeout   time.Duration
	threads          int
	maxDepth         int
	seedURLs         []string
	totalVisited     int
	totalErrors      int
	crawlStartTime   time.Time
	scope            *ScopeSettings
	respectRobotsTxt bool

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
	// Headless browser manager
	headlessBrowserManager *HeadlessBrowserManager
	// URL pattern detector for auto-calibrate
	patternDetector *URLPatternDetector
	// Stats callback for monitoring
	statsCallback StatsCallback
	// Secret detector
	detector *secretscanner.Detector
}

// NewCrawler initializes a new Crawler based on the provided configuration
func NewCrawler(cfg *config.CrawlerConfig, notifier notifier.Notifier, appLogger zerolog.Logger) (*Crawler, error) {
	builder := NewCrawlerBuilder(appLogger).WithConfig(cfg).WithNotifier(notifier)
	return builder.Build()
}

// CrawlerBuilder provides a fluent interface for creating Crawler instances
type CrawlerBuilder struct {
	config   *config.CrawlerConfig
	logger   zerolog.Logger
	notifier notifier.Notifier
}

// NewCrawlerBuilder creates a new CrawlerBuilder instance
func NewCrawlerBuilder(logger zerolog.Logger) *CrawlerBuilder {
	return &CrawlerBuilder{
		logger: logger.With().Str("module", "Crawler").Logger(),
	}
}

// WithConfig sets the crawler configuration
func (cb *CrawlerBuilder) WithConfig(cfg *config.CrawlerConfig) *CrawlerBuilder {
	cb.config = cfg
	return cb
}

// WithNotifier sets the notifier for alerts
func (cb *CrawlerBuilder) WithNotifier(notifier notifier.Notifier) *CrawlerBuilder {
	cb.notifier = notifier
	return cb
}

// Build creates a new Crawler instance with the configured settings
func (cb *CrawlerBuilder) Build() (*Crawler, error) {
	if cb.config == nil {
		return nil, common.NewValidationError("config", nil, "crawler config cannot be nil")
	}

	crawler := &Crawler{
		discoveredURLs: make(map[string]bool),
		urlParentMap:   make(map[string]string),
		logger:         cb.logger,
		config:         cb.config,
	}

	if cb.config.Secrets.Enabled {
		secretsStore, err := datastore.NewSecretsStore(&cb.config.Secrets.SecretsStore, cb.logger)
		if err != nil {
			return nil, common.WrapError(err, "failed to create secrets store")
		}
		detector, err := secretscanner.NewDetector(
			&cb.config.Secrets,
			secretsStore,
			cb.notifier,
			cb.logger,
		)
		if err != nil {
			return nil, common.WrapError(err, "failed to create secret detector")
		}
		crawler.detector = detector
		crawler.logger.Info().Msg("Secret detection enabled")
	}

	if err := crawler.initialize(); err != nil {
		return nil, common.WrapError(err, "failed to initialize crawler")
	}

	return crawler, nil
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

// EnableAutoCalibrate re-enables auto-calibrate pattern detection
func (cr *Crawler) EnableAutoCalibrate() {
	cr.logger.Debug().Msg("Auto-calibrate enabled")
	cr.config.AutoCalibrate.Enabled = true
}

// SetStatsCallback sets the stats callback for monitoring
func (cr *Crawler) SetStatsCallback(callback StatsCallback) {
	cr.statsCallback = callback
	if cr.detector != nil {
		cr.detector.SetStatsCallback(cr.statsCallback)
	}
}

// extractRootHostname extracts hostname from the first seed URL
func (cr *Crawler) extractRootHostname(seedURLs []string) string {
	if len(seedURLs) == 0 {
		return ""
	}

	if parsed, err := url.Parse(seedURLs[0]); err == nil {
		return parsed.Hostname()
	}

	return ""
}
