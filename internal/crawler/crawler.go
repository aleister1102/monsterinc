package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"slices"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
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

// NewCrawler initializes a new Crawler based on the provided configuration
func NewCrawler(cfg *config.CrawlerConfig, appLogger zerolog.Logger) (*Crawler, error) {
	builder := NewCrawlerBuilder(appLogger).WithConfig(cfg)
	return builder.Build()
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

	finalAllowedHostnames := cr.calculateAllowedHostnames(cfg)
	rootURLHostname := cr.extractRootHostname(cfg.SeedURLs)

	scope, err := NewScopeSettings(
		rootURLHostname,
		finalAllowedHostnames,
		cfg.Scope.DisallowedHostnames,
		cfg.Scope.AllowedSubdomains,
		cfg.Scope.DisallowedSubdomains,
		cfg.Scope.AllowedPathRegexes,
		cfg.Scope.DisallowedPathRegexes,
		cr.logger,
		cfg.IncludeSubdomains,
		cfg.SeedURLs,
	)

	if err != nil {
		return common.WrapError(err, "failed to initialize scope settings")
	}

	cr.scope = scope
	return nil
}

// calculateAllowedHostnames determines final allowed hostnames including auto-added seed hostnames
func (cr *Crawler) calculateAllowedHostnames(cfg *config.CrawlerConfig) []string {
	finalAllowedHostnames := cfg.Scope.AllowedHostnames

	if cfg.AutoAddSeedHostnames && len(cfg.SeedURLs) > 0 {
		seedHostnames := ExtractHostnamesFromSeedURLs(cfg.SeedURLs, cr.logger)
		if len(seedHostnames) > 0 {
			finalAllowedHostnames = MergeAllowedHostnames(cfg.Scope.AllowedHostnames, seedHostnames)
			cr.logger.Info().
				Strs("seed_hostnames", seedHostnames).
				Strs("original_allowed_hostnames", cfg.Scope.AllowedHostnames).
				Strs("final_allowed_hostnames", finalAllowedHostnames).
				Msg("Auto-added seed hostnames to allowed hostnames")
		}
	}

	return finalAllowedHostnames
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
	cr.logger.Info().
		Strs("seeds", cr.seedURLs).
		Str("user_agent", cr.userAgent).
		Dur("timeout", cr.requestTimeout).
		Int("threads", cr.threads).
		Int("max_depth", cr.maxDepth).
		Bool("respect_robots_txt", cr.respectRobotsTxt).
		Interface("scope", cr.scope).
		Msg("Initialized with config")
}

// handleError processes colly error callbacks
func (cr *Crawler) handleError(r *colly.Response, e error) {
	cr.incrementErrorCount()

	if cr.isContextCancelled() {
		cr.logger.Warn().Str("url", r.Request.URL.String()).Err(e).Msg("Request failed after context cancellation")
		return
	}

	cr.logger.Error().
		Str("url", r.Request.URL.String()).
		Int("status", r.StatusCode).
		Err(e).
		Msg("Request failed")
}

// handleRequest processes colly request callbacks
func (cr *Crawler) handleRequest(r *colly.Request) {
	if cr.isContextCancelled() {
		cr.logger.Info().Str("url", r.URL.String()).Msg("Context cancelled, aborting request")
		r.Abort()
		return
	}

	if cr.shouldAbortRequest(r) {
		cr.logger.Info().
			Str("url", r.URL.String()).
			Str("path", r.URL.Path).
			Msg("Abort request (path matches disallowed regex)")
		r.Abort()
	}
}

// handleResponse processes colly response callbacks
func (cr *Crawler) handleResponse(r *colly.Response) {
	cr.incrementVisitedCount()

	if cr.isHTMLContent(r) {
		cr.extractAssetsFromResponse(r)
	}
}

// shouldAbortRequest checks if request should be aborted based on path patterns
func (cr *Crawler) shouldAbortRequest(r *colly.Request) bool {
	if cr.scope == nil || len(cr.scope.disallowedPathPatterns) == 0 {
		return false
	}

	path := r.URL.Path
	for _, regex := range cr.scope.disallowedPathPatterns {
		if regex.MatchString(path) {
			return true
		}
	}

	return false
}

// isHTMLContent checks if response contains HTML content
func (cr *Crawler) isHTMLContent(r *colly.Response) bool {
	contentType := r.Headers.Get("Content-Type")
	return contentType != "" && strings.Contains(strings.ToLower(contentType), "text/html")
}

// extractAssetsFromResponse extracts assets from HTML response
func (cr *Crawler) extractAssetsFromResponse(r *colly.Response) {
	assets := ExtractAssetsFromHTML(r.Body, r.Request.URL, cr)
	if len(assets) > 0 {
		cr.logger.Info().
			Str("url", r.Request.URL.String()).
			Int("assets", len(assets)).
			Msg("Extracted assets")
	}
}

// isContextCancelled checks if context is cancelled
func (cr *Crawler) isContextCancelled() bool {
	if cr.ctx == nil {
		return false
	}

	select {
	case <-cr.ctx.Done():
		return true
	default:
		return false
	}
}

// incrementErrorCount safely increments error counter
func (cr *Crawler) incrementErrorCount() {
	cr.mutex.Lock()
	cr.totalErrors++
	cr.mutex.Unlock()
}

// incrementVisitedCount safely increments visited counter
func (cr *Crawler) incrementVisitedCount() {
	cr.mutex.Lock()
	cr.totalVisited++
	cr.mutex.Unlock()
}

// DiscoverURL attempts to add a new URL to the crawl queue
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	if cr.isContextCancelled() {
		cr.logger.Debug().Str("raw_url", rawURL).Msg("Context cancelled, skipping URL discovery")
		return
	}

	normalizedURL, shouldSkip := cr.processRawURL(rawURL, base)
	if shouldSkip {
		return
	}

	if cr.isURLAlreadyDiscovered(normalizedURL) {
		return
	}

	if cr.shouldSkipURLByContentLength(normalizedURL) {
		cr.addDiscoveredURL(normalizedURL)
		return
	}

	cr.queueURLForVisit(normalizedURL)
}

// processRawURL resolves and validates the raw URL
func (cr *Crawler) processRawURL(rawURL string, base *url.URL) (string, bool) {
	absURL, err := urlhandler.ResolveURL(rawURL, base)
	if err != nil {
		cr.logger.Warn().
			Str("raw_url", rawURL).
			Str("base", base.String()).
			Err(err).
			Msg("Could not resolve URL")

		cr.addDiscoveredURL(rawURL)
		return "", true
	}

	normalizedURL := strings.TrimSpace(absURL)
	if normalizedURL == "" {
		return "", true
	}

	if !cr.isURLInScope(normalizedURL) {
		return "", true
	}

	return normalizedURL, false
}

// isURLInScope checks if URL is within crawler scope
func (cr *Crawler) isURLInScope(normalizedURL string) bool {
	if cr.scope == nil {
		return true
	}

	isAllowed, err := cr.scope.IsURLAllowed(normalizedURL)
	if err != nil {
		cr.logger.Warn().Str("url", normalizedURL).Err(err).Msg("Scope check encountered an issue")
		return false
	}

	return isAllowed
}

// isURLAlreadyDiscovered checks if URL was already discovered
func (cr *Crawler) isURLAlreadyDiscovered(normalizedURL string) bool {
	cr.mutex.RLock()
	exists := cr.discoveredURLs[normalizedURL]
	cr.mutex.RUnlock()
	return exists
}

// shouldSkipURLByContentLength performs HEAD request to check content length
func (cr *Crawler) shouldSkipURLByContentLength(normalizedURL string) bool {
	headReq, err := http.NewRequest("HEAD", normalizedURL, nil)
	if err != nil {
		return false
	}

	resp, err := cr.httpClient.Do(headReq)
	if err != nil {
		cr.logger.Warn().Str("url", normalizedURL).Err(err).Msg("HEAD request failed")
		return false
	}
	defer resp.Body.Close()

	return cr.checkContentLength(resp, normalizedURL)
}

// checkContentLength validates response content length
func (cr *Crawler) checkContentLength(resp *http.Response, normalizedURL string) bool {
	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return false
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return false
	}

	if size > cr.maxContentLength {
		cr.logger.Info().
			Str("url", normalizedURL).
			Int64("size", size).
			Int64("max_size", cr.maxContentLength).
			Msg("Skip queue (Content-Length exceeded)")
		return true
	}

	return false
}

// queueURLForVisit adds URL to colly queue for crawling
func (cr *Crawler) queueURLForVisit(normalizedURL string) {
	cr.mutex.Lock()

	// Double-check after acquiring write lock
	if cr.discoveredURLs[normalizedURL] {
		cr.mutex.Unlock()
		return
	}

	cr.discoveredURLs[normalizedURL] = true
	cr.mutex.Unlock()

	if err := cr.collector.Visit(normalizedURL); err != nil {
		cr.handleVisitError(normalizedURL, err)
	}
}

// addDiscoveredURL safely adds URL to discovered list
func (cr *Crawler) addDiscoveredURL(url string) {
	cr.mutex.Lock()
	cr.discoveredURLs[url] = true
	cr.mutex.Unlock()
}

// handleVisitError handles errors from colly Visit calls
func (cr *Crawler) handleVisitError(normalizedURL string, err error) {
	if strings.Contains(err.Error(), "already visited") || errors.Is(err, colly.ErrRobotsTxtBlocked) {
		return
	}

	cr.logger.Warn().
		Str("url", normalizedURL).
		Err(err).
		Msg("Error queueing visit")
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

// Start initiates the crawling process with configured seed URLs
func (cr *Crawler) Start(ctx context.Context) {
	cr.ctx = ctx
	cr.crawlStartTime = time.Now()

	cr.logger.Info().
		Int("seed_count", len(cr.seedURLs)).
		Strs("seeds", cr.seedURLs).
		Msg("Starting crawl")

	cr.processSeedURLs()
	cr.waitForCompletion()
	cr.logSummary()
}

// processSeedURLs processes all seed URLs for crawling
func (cr *Crawler) processSeedURLs() {
	for _, seed := range cr.seedURLs {
		if cr.isContextCancelled() {
			cr.logger.Info().Msg("Context cancelled during seed processing, stopping crawler start")
			return
		}

		cr.processSeedURL(seed)
	}
}

// processSeedURL processes a single seed URL
func (cr *Crawler) processSeedURL(seed string) {
	parsedSeed, err := urlhandler.ResolveURL(seed, nil)
	if err != nil {
		cr.logger.Error().Str("seed", seed).Err(err).Msg("Invalid or non-absolute seed URL")
		return
	}

	baseForSeed, _ := url.Parse(parsedSeed)
	cr.DiscoverURL(parsedSeed, baseForSeed)
}

// waitForCompletion waits for all crawling threads to complete
func (cr *Crawler) waitForCompletion() {
	cr.logger.Info().Int("active_threads", cr.threads).Msg("Waiting for threads to complete")
	cr.collector.Wait()

	if cr.isContextCancelled() {
		cr.logger.Info().Msg("Context cancelled while waiting for collector to finish")
	}
}

// logSummary logs the crawling summary statistics
func (cr *Crawler) logSummary() {
	duration := time.Since(cr.crawlStartTime)

	cr.mutex.RLock()
	visited := cr.totalVisited
	discovered := len(cr.discoveredURLs)
	errors := cr.totalErrors
	cr.mutex.RUnlock()

	cr.logger.Info().Strs("seeds", cr.seedURLs).Msg("Crawl finished")
	cr.logger.Info().
		Dur("duration", duration).
		Int("visited", visited).
		Int("discovered", discovered).
		Int("errors", errors).
		Msg("Summary")
}

// getValueOrDefault returns value if not empty, otherwise returns default
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// getIntValueOrDefault returns value if greater than 0, otherwise returns default
func getIntValueOrDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}
