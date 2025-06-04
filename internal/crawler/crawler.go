package crawler

import (
	"context"
	"errors" // Added for resolveURL

	// Added for more detailed logging
	"fmt" // Placeholder for proper logging integration later
	"net/http"
	"net/url" // For URL parsing and manipulation
	"strconv"
	"strings"
	"sync" // For thread-safe access to discoveredURLs
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config" // Import the config package
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
)

// Crawler represents the web crawler instance.
type Crawler struct {
	Collector        *colly.Collector
	discoveredURLs   map[string]bool
	mutex            sync.RWMutex
	UserAgent        string
	RequestTimeout   time.Duration
	Threads          int
	MaxDepth         int
	seedURLs         []string
	totalVisited     int // Approximate count of pages gocolly responded to via OnResponse
	totalErrors      int // Count of errors from OnError
	crawlStartTime   time.Time
	Scope            *ScopeSettings // Scope settings for URL filtering
	RespectRobotsTxt bool           // robots.txt preference
	maxContentLength int64
	headTimeout      time.Duration
	logger           zerolog.Logger
	config           *config.CrawlerConfig
	ctx              context.Context // Added context
	httpClient       *http.Client    // HTTP client for HEAD requests
}

// configureCollyCollector sets up and configures a new colly.Collector instance based on CrawlerConfig.
func configureCollyCollector(cfg *config.CrawlerConfig, crawlerTimeoutDuration time.Duration, userAgent string) (*colly.Collector, error) {
	collectorOptions := []colly.CollectorOption{
		colly.Async(true),
		colly.UserAgent(userAgent),
		colly.MaxDepth(cfg.MaxDepth), // MaxDepth is now directly from cfg
	}

	if !cfg.RespectRobotsTxt {
		collectorOptions = append(collectorOptions, colly.IgnoreRobotsTxt())
	}

	c := colly.NewCollector(collectorOptions...)
	c.SetRequestTimeout(crawlerTimeoutDuration)

	threads := cfg.MaxConcurrentRequests
	if threads <= 0 {
		threads = config.DefaultCrawlerMaxConcurrentRequests
	}

	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: threads,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting up colly limit rule: %w", err)
	}
	return c, nil
}

// handleError is a Colly callback executed when an error occurs during a request.
func (cr *Crawler) handleError(r *colly.Response, e error) {
	cr.mutex.Lock()
	cr.totalErrors++
	cr.mutex.Unlock()
	// Check if context was cancelled, which might cause some errors
	if cr.ctx != nil {
		select {
		case <-cr.ctx.Done():
			cr.logger.Warn().Str("url", r.Request.URL.String()).Err(e).Msg("Request failed after context cancellation")
			return
		default:
		}
	}
	cr.logger.Error().Str("url", r.Request.URL.String()).Int("status", r.StatusCode).Err(e).Msg("Request failed")
}

// handleRequest is a Colly callback executed before a request is made.
// It checks if the request URL path is disallowed by regex patterns.
func (cr *Crawler) handleRequest(r *colly.Request) {
	// Check context cancellation first
	if cr.ctx != nil {
		select {
		case <-cr.ctx.Done():
			cr.logger.Info().Str("url", r.URL.String()).Msg("Context cancelled, aborting request")
			r.Abort()
			return
		default:
		}
	}

	if cr.Scope != nil && len(cr.Scope.DisallowedPathPatterns) > 0 {
		path := r.URL.Path
		for _, re := range cr.Scope.DisallowedPathPatterns {
			if re.MatchString(path) {
				cr.logger.Info().Str("url", r.URL.String()).Str("regex", re.String()).Msg("Abort request (path matches disallowed regex)")
				r.Abort()
				return
			}
		}
	}
}

// handleResponse is a Colly callback executed when a response is received.
// It increments the visited count and extracts assets if the content is HTML.
func (cr *Crawler) handleResponse(r *colly.Response) {
	cr.mutex.Lock()
	cr.totalVisited++
	cr.mutex.Unlock()
	if r.Headers.Get("Content-Type") != "" && strings.Contains(strings.ToLower(r.Headers.Get("Content-Type")), "text/html") {
		// Pass r.Body ([]byte) directly
		assets := ExtractAssetsFromHTML(r.Body, r.Request.URL, cr)
		// ExtractAssetsFromHTML now logs its own errors and doesn't return one.
		// if err != nil { // Removed error handling here as function signature changed
		// 	cr.logger.Warn().Str("url", r.Request.URL.String()).Err(err).Msg("Error extracting assets")
		// } else {
		if len(assets) > 0 {
			cr.logger.Info().Str("url", r.Request.URL.String()).Int("assets", len(assets)).Msg("Extracted assets")
		}
		// }
	}
}

// NewCrawler initializes a new Crawler based on the provided configuration.
func NewCrawler(cfg *config.CrawlerConfig, appLogger zerolog.Logger) (*Crawler, error) {
	moduleLogger := appLogger.With().Str("module", "Crawler").Logger()

	if cfg == nil {
		moduleLogger.Error().Msg("Crawler configuration cannot be nil.")
		return nil, fmt.Errorf("crawler config cannot be nil")
	}

	// Validate essential configurations
	if len(cfg.SeedURLs) == 0 {
		// Allow crawler to be initialized without seed URLs, as Orchestrator might not have them
		// if the input source is empty. Crawler.Start will handle empty seeds.
		moduleLogger.Warn().Msg("Crawler initialized with no seed URLs in config. Orchestrator will provide seeds at Start, or crawler will do nothing.")
	}

	// Use defaults from CrawlerConfig if specific values are not set or are zero-values
	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = config.DefaultCrawlerUserAgent // Use constant from config package
	}

	requestTimeoutSecs := cfg.RequestTimeoutSecs
	if requestTimeoutSecs <= 0 {
		requestTimeoutSecs = config.DefaultCrawlerRequestTimeoutSecs // Use constant from config package
	}
	crawlerTimeoutDuration := time.Duration(requestTimeoutSecs) * time.Second

	// MaxDepth handling (ensure it uses default if cfg.MaxDepth is not valid)
	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = config.DefaultCrawlerMaxDepth
		cfg.MaxDepth = maxDepth // Update cfg to reflect the used default
	}

	// Determine threads, consistent with collector config and for Crawler struct
	crawlerThreads := cfg.MaxConcurrentRequests
	if crawlerThreads <= 0 {
		crawlerThreads = config.DefaultCrawlerMaxConcurrentRequests
	}

	maxContentLengthMB := cfg.MaxContentLengthMB
	if maxContentLengthMB <= 0 {
		maxContentLengthMB = 2 // fallback nếu config lỗi
	}
	maxContentLength := int64(maxContentLengthMB) * 1024 * 1024

	// Auto-add seed hostnames to allowed hostnames if enabled
	finalAllowedHostnames := cfg.Scope.AllowedHostnames
	if cfg.AutoAddSeedHostnames && len(cfg.SeedURLs) > 0 {
		seedHostnames := ExtractHostnamesFromSeedURLs(cfg.SeedURLs, moduleLogger)
		if len(seedHostnames) > 0 {
			finalAllowedHostnames = MergeAllowedHostnames(cfg.Scope.AllowedHostnames, seedHostnames)
			moduleLogger.Info().
				Strs("seed_hostnames", seedHostnames).
				Strs("original_allowed_hostnames", cfg.Scope.AllowedHostnames).
				Strs("final_allowed_hostnames", finalAllowedHostnames).
				Msg("Auto-added seed hostnames to allowed hostnames")
		}
	}

	// Initialize ScopeSettings with the logger
	var rootURLHostname string
	if len(cfg.SeedURLs) > 0 {
		if parsed, err := url.Parse(cfg.SeedURLs[0]); err == nil {
			rootURLHostname = parsed.Hostname()
		}
	}

	scopeSettings, err := NewScopeSettings(
		rootURLHostname,
		finalAllowedHostnames,
		cfg.Scope.DisallowedHostnames,
		cfg.Scope.AllowedSubdomains,
		cfg.Scope.DisallowedSubdomains,
		cfg.Scope.AllowedPathRegexes,    // TODO
		cfg.Scope.DisallowedPathRegexes, // TODO
		moduleLogger,                    // Pass the moduleLogger to ScopeSettings
		cfg.IncludeSubdomains,           // Pass IncludeSubdomains setting
		cfg.SeedURLs,                    // Pass original seed URLs for base domain extraction
	)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to initialize scope settings")
		return nil, err
	}

	collector, err := configureCollyCollector(cfg, crawlerTimeoutDuration, userAgent)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to configure Colly collector")
		return nil, err
	}

	// Create HTTP client for HEAD requests using common package
	httpClientFactory := common.NewHTTPClientFactory(moduleLogger)
	headClient, err := httpClientFactory.CreateCrawlerClient(crawlerTimeoutDuration, "", nil)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to create HTTP client for HEAD requests")
		return nil, err
	}

	cr := &Crawler{
		Collector:        collector,
		discoveredURLs:   make(map[string]bool),
		UserAgent:        userAgent,
		RequestTimeout:   crawlerTimeoutDuration, // Assign the converted time.Duration
		Threads:          crawlerThreads,         // Initialize cr.Threads
		MaxDepth:         maxDepth,               // Initialize cr.MaxDepth
		seedURLs:         append([]string(nil), cfg.SeedURLs...),
		Scope:            scopeSettings,
		RespectRobotsTxt: cfg.RespectRobotsTxt, // Store the actual setting used
		maxContentLength: maxContentLength,
		headTimeout:      crawlerTimeoutDuration,
		logger:           moduleLogger,
		config:           cfg,
		httpClient:       headClient, // Use the created HTTP client
	}

	// Setup Colly Callbacks using the new methods
	cr.Collector.OnError(cr.handleError)
	cr.Collector.OnRequest(cr.handleRequest)
	cr.Collector.OnResponse(cr.handleResponse)

	moduleLogger.Info().Strs("seeds", cr.seedURLs).Str("user_agent", cr.UserAgent).Dur("timeout", cr.RequestTimeout).Int("threads", cr.Threads).Int("max_depth", cr.MaxDepth).Bool("respect_robots_txt", cr.RespectRobotsTxt).Interface("scope", cr.Scope).Msg("Initialized with config")

	return cr, nil
}

// DiscoverURL attempts to add a new URL to the crawl queue if it hasn't been discovered and processed by us yet.
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	// Check context cancellation
	if cr.ctx != nil {
		select {
		case <-cr.ctx.Done():
			cr.logger.Debug().Str("raw_url", rawURL).Msg("Context cancelled, skipping URL discovery")
			return
		default:
		}
	}

	absURL, err := urlhandler.ResolveURL(rawURL, base)
	if err != nil {
		cr.logger.Warn().Str("raw_url", rawURL).Str("base", base.String()).Err(err).Msg("Could not resolve URL")
		// Vẫn ghi nhận URL để httpx probe thử
		cr.mutex.Lock()
		cr.discoveredURLs[rawURL] = true
		cr.mutex.Unlock()
		return
	}
	normalizedAbsURL := strings.TrimSpace(absURL) // Basic normalization
	if normalizedAbsURL == "" {
		return
	}

	if cr.Scope != nil {
		isAllowed, scopeErr := cr.Scope.IsURLAllowed(normalizedAbsURL)
		if scopeErr != nil {
			cr.logger.Warn().Str("url", normalizedAbsURL).Err(scopeErr).Msg("Scope check encountered an issue")
			return
		}
		if !isAllowed {
			// log.Printf("[INFO] Crawler: URL '%s' is out of scope. Skipping.", normalizedAbsURL)
			return
		}
	}

	cr.mutex.RLock()
	exists := cr.discoveredURLs[normalizedAbsURL]
	cr.mutex.RUnlock()

	if !exists {
		// HEAD check before queueing using common HTTP client
		headReq, err := http.NewRequest("HEAD", normalizedAbsURL, nil)
		if err == nil {
			resp, err := cr.httpClient.Do(headReq)
			if err != nil {
				cr.logger.Warn().Str("url", normalizedAbsURL).Err(err).Msg("HEAD request failed")
				cr.mutex.Lock()
				cr.discoveredURLs[normalizedAbsURL] = true
				cr.mutex.Unlock()
				return
			}
			resp.Body.Close()
			if cl := resp.Header.Get("Content-Length"); cl != "" {
				if size, err := strconv.ParseInt(cl, 10, 64); err == nil && size > cr.maxContentLength {
					cr.logger.Info().Str("url", normalizedAbsURL).Int64("size", size).Int64("max_size", cr.maxContentLength).Msg("Skip queue (Content-Length exceeded)")
					// Vẫn ghi nhận URL vào discoveredURLs để httpx runner xử lý
					cr.mutex.Lock()
					cr.discoveredURLs[normalizedAbsURL] = true
					cr.mutex.Unlock()
					return
				}
			}
		}
		cr.mutex.Lock()
		// Double-check after acquiring write lock
		if !cr.discoveredURLs[normalizedAbsURL] {
			cr.discoveredURLs[normalizedAbsURL] = true
			cr.mutex.Unlock()
			visitErr := cr.Collector.Visit(normalizedAbsURL)
			if visitErr != nil && !strings.Contains(visitErr.Error(), "already visited") && !errors.Is(visitErr, colly.ErrRobotsTxtBlocked) {
				cr.logger.Warn().Str("url", normalizedAbsURL).Err(visitErr).Msg("Error queueing visit")
			}
		} else {
			cr.mutex.Unlock() // Already added by another goroutine
		}
	}
}

// GetDiscoveredURLs returns a slice of all unique URLs discovered by our logic.
func (cr *Crawler) GetDiscoveredURLs() []string {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	urls := make([]string, 0, len(cr.discoveredURLs))
	for u := range cr.discoveredURLs {
		urls = append(urls, u)
	}
	return urls
}

// Start initiates the crawling process with the configured seed URLs.
func (cr *Crawler) Start(ctx context.Context) {
	cr.ctx = ctx // Store context
	cr.crawlStartTime = time.Now()
	// Reset counters for multiple Start calls on the same crawler instance, if that's a use case.
	// cr.totalVisited = 0
	// cr.totalErrors = 0
	// cr.discoveredURLs = make(map[string]bool) // Or clear if re-using

	cr.logger.Info().Int("seed_count", len(cr.seedURLs)).Strs("seeds", cr.seedURLs).Msg("Starting crawl")

	for _, seed := range cr.seedURLs {
		// Check context before processing each seed
		select {
		case <-cr.ctx.Done():
			cr.logger.Info().Msg("Context cancelled during seed processing, stopping crawler start.")
			return
		default:
		}
		// Resolve the seed URL against nil base to ensure it's absolute and valid
		// DiscoverURL will then handle adding it to the collector
		parsedSeed, err := urlhandler.ResolveURL(seed, nil) // base is nil for seed
		if err != nil {
			cr.logger.Error().Str("seed", seed).Err(err).Msg("Invalid or non-absolute seed URL")
			continue
		}
		// Use parsedSeed as its own base for the DiscoverURL call, or nil if DiscoverURL handles it
		baseForSeed, _ := url.Parse(parsedSeed)
		cr.DiscoverURL(parsedSeed, baseForSeed) // Effectively adds to collector queue via Visit if new
	}

	// Wait for crawling to complete
	cr.logger.Info().Int("active_threads", cr.Threads).Msg("Waiting for threads to complete")
	cr.Collector.Wait()

	// Check context after Wait as well, in case it was cancelled while waiting.
	select {
	case <-cr.ctx.Done():
		cr.logger.Info().Msg("Context cancelled while waiting for collector to finish.")
	default:
	}

	cr.logSummary()
}

// logSummary logs the crawling summary statistics.
func (cr *Crawler) logSummary() {
	duration := time.Since(cr.crawlStartTime)
	cr.mutex.RLock() // Protect access to counters and discoveredURLs map
	defer cr.mutex.RUnlock()

	// Clarify what "URLs Visited" means in this context.
	// totalVisited is incremented on OnResponse, which might include redirects or non-HTML pages.
	// len(cr.discoveredURLs) is the count of unique URLs our DiscoverURL method decided to queue.
	cr.logger.Info().Strs("seeds", cr.seedURLs).Msg("Crawl finished")
	cr.logger.Info().Dur("duration", duration).Int("visited", cr.totalVisited).Int("discovered", len(cr.discoveredURLs)).Int("errors", cr.totalErrors).Msg("Summary")
}
