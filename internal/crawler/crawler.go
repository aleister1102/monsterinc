package crawler

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"slices"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
)

// Crawler represents the web crawler instance with thread-safe operations
type Crawler struct {
	collector        *colly.Collector
	discoveredURLs   map[string]bool
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
	maxContentLength int64
	headTimeout      time.Duration
	logger           zerolog.Logger
	config           *config.CrawlerConfig
	ctx              context.Context
	httpClient       *http.Client
}

// NewCrawler initializes a new Crawler based on the provided configuration
func NewCrawler(cfg *config.CrawlerConfig, appLogger zerolog.Logger) (*Crawler, error) {
	builder := NewCrawlerBuilder(appLogger).WithConfig(cfg)
	return builder.Build()
}

// CrawlerBuilder provides a fluent interface for creating Crawler instances
type CrawlerBuilder struct {
	config *config.CrawlerConfig
	logger zerolog.Logger
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

// Build creates a new Crawler instance with the configured settings
func (cb *CrawlerBuilder) Build() (*Crawler, error) {
	if cb.config == nil {
		return nil, common.NewValidationError("config", nil, "crawler config cannot be nil")
	}

	crawler := &Crawler{
		discoveredURLs: make(map[string]bool),
		logger:         cb.logger,
		config:         cb.config,
	}

	if err := crawler.initialize(); err != nil {
		return nil, common.WrapError(err, "failed to initialize crawler")
	}

	return crawler, nil
}

// initialize sets up the crawler with configuration and dependencies
func (cr *Crawler) initialize() error {
	if err := cr.validateAndSetDefaults(); err != nil {
		return err
	}

	if err := cr.setupScope(); err != nil {
		return err
	}

	if err := cr.setupCollector(); err != nil {
		return err
	}

	if err := cr.setupHTTPClient(); err != nil {
		return err
	}

	cr.logInitialization()
	return nil
}

// validateAndSetDefaults validates configuration and sets default values
func (cr *Crawler) validateAndSetDefaults() error {
	cfg := cr.config

	if len(cfg.SeedURLs) == 0 {
		cr.logger.Warn().Msg("Crawler initialized with no seed URLs in config")
	}

	cr.userAgent = getValueOrDefault(cfg.UserAgent, config.DefaultCrawlerUserAgent)

	requestTimeoutSecs := getIntValueOrDefault(cfg.RequestTimeoutSecs, config.DefaultCrawlerRequestTimeoutSecs)
	cr.requestTimeout = time.Duration(requestTimeoutSecs) * time.Second

	cr.maxDepth = getIntValueOrDefault(cfg.MaxDepth, config.DefaultCrawlerMaxDepth)
	cr.threads = getIntValueOrDefault(cfg.MaxConcurrentRequests, config.DefaultCrawlerMaxConcurrentRequests)

	maxContentLengthMB := getIntValueOrDefault(cfg.MaxContentLengthMB, 2)
	cr.maxContentLength = int64(maxContentLengthMB) * 1024 * 1024

	cr.headTimeout = cr.requestTimeout
	cr.respectRobotsTxt = cfg.RespectRobotsTxt
	cr.seedURLs = slices.Clone(cfg.SeedURLs)

	// Update config to reflect used defaults
	cfg.MaxDepth = cr.maxDepth

	return nil
}

// setupScope initializes scope settings for URL filtering
func (cr *Crawler) setupScope() error {
	cfg := cr.config

	scope, err := NewScopeSettings(
		cr.extractRootHostname(cfg.SeedURLs),
		cfg.Scope.DisallowedHostnames,
		cfg.Scope.DisallowedSubdomains,
		cfg.Scope.DisallowedFileExtensions,
		cr.logger,
		cfg.IncludeSubdomains,
		cfg.AutoAddSeedHostnames,
		cfg.SeedURLs,
	)

	if err != nil {
		return common.WrapError(err, "failed to initialize scope settings")
	}

	cr.scope = scope
	return nil
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

// setupCollector configures the colly collector
func (cr *Crawler) setupCollector() error {
	collector, err := cr.createCollector()
	if err != nil {
		return common.WrapError(err, "failed to configure colly collector")
	}

	cr.collector = collector
	cr.setupCallbacks()
	return nil
}

// createCollector creates and configures a new colly.Collector instance
func (cr *Crawler) createCollector() (*colly.Collector, error) {
	collectorOptions := []colly.CollectorOption{
		colly.Async(true),
		colly.UserAgent(cr.userAgent),
		colly.MaxDepth(cr.maxDepth),
	}

	if !cr.respectRobotsTxt {
		collectorOptions = append(collectorOptions, colly.IgnoreRobotsTxt())
	}

	collector := colly.NewCollector(collectorOptions...)
	collector.SetRequestTimeout(cr.requestTimeout)

	err := collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: cr.threads,
	})

	if err != nil {
		return nil, common.WrapError(err, "error setting up colly limit rule")
	}

	return collector, nil
}

// setupCallbacks configures colly event callbacks
func (cr *Crawler) setupCallbacks() {
	cr.collector.OnError(cr.handleError)
	cr.collector.OnRequest(cr.handleRequest)
	cr.collector.OnResponse(cr.handleResponse)
}

// setupHTTPClient creates HTTP client for HEAD requests
func (cr *Crawler) setupHTTPClient() error {
	httpClientFactory := common.NewHTTPClientFactory(cr.logger)
	client, err := httpClientFactory.CreateCrawlerClient(cr.requestTimeout, "", nil)
	if err != nil {
		return common.WrapError(err, "failed to create HTTP client for HEAD requests")
	}

	cr.httpClient = client
	return nil
}

// logInitialization logs the initialization summary
func (cr *Crawler) logInitialization() {
	logEvent := cr.logger.Info().
		Strs("seeds", cr.seedURLs).
		Str("user_agent", cr.userAgent).
		Dur("timeout", cr.requestTimeout).
		Int("threads", cr.threads).
		Int("max_depth", cr.maxDepth).
		Bool("respect_robots_txt", cr.respectRobotsTxt)

	// Log scope settings details if available
	if cr.scope != nil {
		logEvent = logEvent.Str("scope", cr.scope.String())
	} else {
		logEvent = logEvent.Str("scope", "nil")
	}

	logEvent.Msg("Initialized with config")
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
