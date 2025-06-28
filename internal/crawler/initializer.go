package crawler

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"slices"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/gocolly/colly/v2"
)

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

	cr.initializeURLBatcher()
	cr.initializeExtensionMap()
	cr.initializePatternDetector()
	cr.logInitialization()
	return nil
}

// validateAndSetDefaults validates configuration and sets default values
func (cr *Crawler) validateAndSetDefaults() error {
	cfg := cr.config

	if len(cfg.SeedURLs) == 0 {
		cr.logger.Warn().Msg("Crawler initialized with no seed URLs in config")
	}

	requestTimeoutSecs := getIntValueOrDefault(cfg.RequestTimeoutSecs, config.DefaultCrawlerRequestTimeoutSecs)
	cr.requestTimeout = time.Duration(requestTimeoutSecs) * time.Second

	cr.maxDepth = getIntValueOrDefault(cfg.MaxDepth, config.DefaultCrawlerMaxDepth)
	cr.threads = getIntValueOrDefault(cfg.MaxConcurrentRequests, config.DefaultCrawlerMaxConcurrentRequests)

	cr.seedURLs = slices.Clone(cfg.SeedURLs)

	// Update config to reflect used defaults
	cfg.MaxDepth = cr.maxDepth

	return nil
}

// getIntValueOrDefault returns value if greater than 0, otherwise returns default
func getIntValueOrDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}

// initializeURLBatcher sets up URL batching for improved performance
func (cr *Crawler) initializeURLBatcher() {
	// Size channel buffer based on expected concurrent requests
	bufferSize := cr.config.MaxConcurrentRequests * 2
	if bufferSize < 50 {
		bufferSize = 50
	} else if bufferSize > 500 {
		bufferSize = 500
	}

	cr.urlQueue = make(chan string, bufferSize)
	cr.urlBatchSize = 10
	cr.batchShutdown = make(chan struct{})
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
		colly.MaxDepth(cr.maxDepth),
		colly.IgnoreRobotsTxt(),
	}

	collector := colly.NewCollector(collectorOptions...)
	collector.SetRequestTimeout(cr.requestTimeout)

	// Create base HTTP transport
	baseTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     90 * time.Second,
	}

	// Wrap with retry transport if retries are enabled
	var transport http.RoundTripper = baseTransport
	if cr.config.RetryConfig.MaxRetries > 0 {
		transport = NewRetryTransport(baseTransport, cr.config.RetryConfig, cr.config.URLNormalization, cr.logger)
		cr.logger.Info().
			Int("max_retries", cr.config.RetryConfig.MaxRetries).
			Int("base_delay_secs", cr.config.RetryConfig.BaseDelaySecs).
			Int("max_delay_secs", cr.config.RetryConfig.MaxDelaySecs).
			Bool("enable_jitter", cr.config.RetryConfig.EnableJitter).
			Ints("retry_status_codes", cr.config.RetryConfig.RetryStatusCodes).
			Msg("Colly configured with retry transport for rate limiting")
	}

	collector.WithTransport(transport)

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

// initializeExtensionMap caches disallowed extensions for fast lookup
func (cr *Crawler) initializeExtensionMap() {
	cr.disallowedExtMap = make(map[string]bool)
	if cr.scope != nil {
		for _, ext := range cr.scope.disallowedFileExtensions {
			cr.disallowedExtMap[ext] = true
		}
	}
}

// initializePatternDetector sets up URL pattern detector for auto-calibrate
func (cr *Crawler) initializePatternDetector() {
	cr.patternDetector = NewURLPatternDetector(cr.config.AutoCalibrate, cr.logger)
}

// logInitialization logs the initialization summary
func (cr *Crawler) logInitialization() {
	logEvent := cr.logger.Info().
		// Strs("seeds", cr.seedURLs).
		Dur("timeout", cr.requestTimeout).
		Int("threads", cr.threads).
		Int("max_depth", cr.maxDepth)

	// Log scope settings details if available
	if cr.scope != nil {
		logEvent = logEvent.Str("scope", cr.scope.String())
	} else {
		logEvent = logEvent.Str("scope", "nil")
	}

	logEvent.Msg("Initialized with config")
}
