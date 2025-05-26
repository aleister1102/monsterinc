package crawler

import (
	"errors" // Added for resolveURL
	// Added for more detailed logging
	"fmt"
	"log" // Placeholder for proper logging integration later
	"net/http"
	"net/url" // For URL parsing and manipulation
	"strconv"
	"strings"
	"sync" // For thread-safe access to discoveredURLs
	"time"

	"monsterinc/internal/config" // Import the config package
	"monsterinc/internal/urlhandler"

	"github.com/gocolly/colly/v2"
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
	Scope            *ScopeSettings // Task 2.1: Add scope settings
	RespectRobotsTxt bool           // Task 2.3: Store robots.txt preference
	maxContentLength int64
	headTimeout      time.Duration
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
	log.Printf("[ERROR] Crawler: Request to %s failed. Status: %d. Error: %v", r.Request.URL, r.StatusCode, e)
}

// handleRequest is a Colly callback executed before a request is made.
// It checks if the request URL path is disallowed by regex patterns.
func (cr *Crawler) handleRequest(r *colly.Request) {
	if cr.Scope != nil && len(cr.Scope.DisallowedPathPatterns) > 0 {
		path := r.URL.Path
		for _, re := range cr.Scope.DisallowedPathPatterns {
			if re.MatchString(path) {
				log.Printf("[INFO] Crawler: Abort request to %s (path matches disallowed regex: %s)", r.URL.String(), re.String())
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
		assets, err := ExtractAssetsFromHTML(strings.NewReader(string(r.Body)), r.Request.URL, cr)
		if err != nil {
			log.Printf("[WARN] Crawler: Error extracting assets from %s: %v", r.Request.URL.String(), err)
		} else {
			// Log an info message, but avoid overly verbose logging for every page if many assets are typically found.
			// Consider logging only if a significant number of assets are found, or if specific types of assets are found.
			if len(assets) > 0 {
				log.Printf("[INFO] Crawler: Extracted %d assets from %s", len(assets), r.Request.URL.String())
			}
		}
	}
}

// NewCrawler initializes a new Crawler based on the provided configuration.
// Task 1.1: Implement crawler initialization.
// Task 1.2: Initialize structures for URL de-duplication.
// Task 1.3: Enhance initialization logging & basic operational logging.
// Task 2.2: Update NewCrawler to accept path regexes for ScopeSettings.
// Task 2.3: Add RespectRobotsTxt parameter to NewCrawler
// Task 4.x: Modify NewCrawler to accept CrawlerConfig
func NewCrawler(cfg *config.CrawlerConfig) (*Crawler, error) {
	if cfg == nil {
		return nil, errors.New("crawler configuration cannot be nil")
	}

	// Validate essential configurations
	if len(cfg.SeedURLs) == 0 {
		return nil, errors.New("crawler initialization requires at least one seed URL in the configuration")
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

	// Create ScopeSettings internally using scope parameters from CrawlerConfig
	currentScopeSettings := NewScopeSettings(
		cfg.Scope.AllowedHostnames, cfg.Scope.AllowedSubdomains,
		cfg.Scope.DisallowedHostnames, cfg.Scope.DisallowedSubdomains,
		cfg.Scope.AllowedPathRegexes, cfg.Scope.DisallowedPathRegexes,
	)

	collector, err := configureCollyCollector(cfg, crawlerTimeoutDuration, userAgent)
	if err != nil {
		log.Printf("[ERROR] Crawler: Failed to configure Colly collector: %v", err)
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
		Scope:            currentScopeSettings,
		RespectRobotsTxt: cfg.RespectRobotsTxt, // Store the actual setting used
		maxContentLength: maxContentLength,
		headTimeout:      crawlerTimeoutDuration,
	}

	// Setup Colly Callbacks using the new methods
	cr.Collector.OnError(cr.handleError)
	cr.Collector.OnRequest(cr.handleRequest)
	cr.Collector.OnResponse(cr.handleResponse)

	log.Printf("[INFO] Crawler: Initialized with config. Seeds: %v, UserAgent: '%s', Timeout: %s, Threads: %d, MaxDepth: %d, RespectRobotsTxt: %t. Scope: %+v",
		cr.seedURLs, cr.UserAgent, cr.RequestTimeout, cr.Threads, cr.MaxDepth, cr.RespectRobotsTxt, cr.Scope)

	return cr, nil
}

// DiscoverURL attempts to add a new URL to the crawl queue if it hasn't been discovered and processed by us yet.
func (cr *Crawler) DiscoverURL(rawURL string, base *url.URL) {
	absURL, err := urlhandler.ResolveURL(rawURL, base)
	if err != nil {
		log.Printf("[WARN] Crawler: Could not resolve URL '%s' relative to '%s': %v", rawURL, base, err)
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

	// Task 2.1: Check scope before proceeding
	if cr.Scope != nil {
		isAllowed, scopeErr := cr.Scope.IsURLAllowed(normalizedAbsURL)
		if scopeErr != nil {
			log.Printf("[WARN] Crawler: Scope check for URL '%s' encountered an issue: %v. URL will not be visited.", normalizedAbsURL, scopeErr)
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
		// HEAD check before queueing
		headReq, err := http.NewRequest("HEAD", normalizedAbsURL, nil)
		if err == nil {
			client := &http.Client{Timeout: cr.RequestTimeout}
			resp, err := client.Do(headReq)
			if err != nil {
				log.Printf("[WARN] Crawler: HEAD request failed for %s: %v. Still adding to discoveredURLs for httpx.", normalizedAbsURL, err)
				cr.mutex.Lock()
				cr.discoveredURLs[normalizedAbsURL] = true
				cr.mutex.Unlock()
				return
			}
			resp.Body.Close()
			if cl := resp.Header.Get("Content-Length"); cl != "" {
				if size, err := strconv.ParseInt(cl, 10, 64); err == nil && size > cr.maxContentLength {
					log.Printf("[INFO] Crawler: Skip queue %s (Content-Length %d > %d bytes)", normalizedAbsURL, size, cr.maxContentLength)
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
				log.Printf("[WARN] Crawler: Error queueing visit for %s: %v", normalizedAbsURL, visitErr)
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
func (cr *Crawler) Start() {
	cr.crawlStartTime = time.Now()
	// Reset counters for multiple Start calls on the same crawler instance, if that's a use case.
	// cr.totalVisited = 0
	// cr.totalErrors = 0
	// cr.discoveredURLs = make(map[string]bool) // Or clear if re-using

	log.Printf("[INFO] Crawler: Starting crawl for %d seed(s): %v", len(cr.seedURLs), cr.seedURLs)

	for _, seed := range cr.seedURLs {
		// Resolve the seed URL against nil base to ensure it's absolute and valid
		// DiscoverURL will then handle adding it to the collector
		parsedSeed, err := urlhandler.ResolveURL(seed, nil) // base is nil for seed
		if err != nil {
			log.Printf("[ERROR] Crawler: Invalid or non-absolute seed URL '%s': %v. Skipping.", seed, err)
			continue
		}
		// Use parsedSeed as its own base for the DiscoverURL call, or nil if DiscoverURL handles it
		baseForSeed, _ := url.Parse(parsedSeed)
		cr.DiscoverURL(parsedSeed, baseForSeed) // Effectively adds to collector queue via Visit if new
	}

	// Wait for crawling to complete
	log.Printf("[INFO] Crawler: Waiting for %d active threads to complete...", cr.Threads)
	cr.Collector.Wait()

	cr.logSummary()
}

// logSummary logs the crawling summary statistics.
func (cr *Crawler) logSummary() {
	cr.mutex.RLock() // Protect access to counters and discoveredURLs map
	defer cr.mutex.RUnlock()

	duration := time.Since(cr.crawlStartTime)
	// Clarify what "URLs Visited" means in this context.
	// totalVisited is incremented on OnResponse, which might include redirects or non-HTML pages.
	// len(cr.discoveredURLs) is the count of unique URLs our DiscoverURL method decided to queue.
	log.Printf("[INFO] Crawler: Crawl finished for seeds: %v", cr.seedURLs)
	log.Printf("[INFO] Crawler: Summary - Duration: %s, URLs processed (OnResponse): %d, Unique URLs discovered/queued: %d, HTTP errors (OnError): %d",
		duration, cr.totalVisited, len(cr.discoveredURLs), cr.totalErrors)
}

// TODO:
// - Asset discovery (OnHTML callbacks to call DiscoverURL) - Task 3.x
// - Scope control integration (check scope in DiscoverURL before c.Visit) - Task 2.x
// - More refined error handling and specific logging levels (integrate project logger) - Task 5.x
// - Configuration loading for crawler settings (MaxDepth, UserAgent, etc. from file) - Task 4.x
