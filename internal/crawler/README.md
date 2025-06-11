# Crawler Package

## Purpose
The `crawler` package provides comprehensive web crawling functionality for MonsterInc's security scanning pipeline. Built on the Colly framework, it discovers URLs, assets, and endpoints from target websites with intelligent scope management and headless browser support.

## Package Role in MonsterInc
As the discovery engine, this package:
- **Feeds the Scanner**: Provides discovered URLs to the scanning pipeline
- **Supports Monitoring**: Discovers new endpoints for continuous monitoring
- **Asset Discovery**: Finds JavaScript files for path extraction
- **Scope Management**: Ensures crawling stays within defined boundaries
- **Integration Ready**: Seamlessly works with HTTPx Runner and Extractor

**Key Capabilities:**
- **Context-aware crawling** with immediate interrupt response
- **Responsive cancellation** - stops URL processing and batch operations instantly
- **Graceful shutdown** with configurable timeout (2 seconds default)
- **Signal-safe operations** - all URL discovery and processing can be cancelled mid-operation

## Main Components

### 1. Crawler Core (`crawler.go`)
#### Purpose
- Main crawling engine with Colly integration
- Concurrent request handling with connection pooling
- Scope-based crawling with hostname and path filtering
- URL discovery and parent tracking
- Context-aware cancellation support

#### API Usage

```go
// Create crawler with configuration
cfg := &config.CrawlerConfig{
    MaxDepth:              3,
    MaxConcurrentRequests: 10,
    RequestTimeoutSecs:    30,
    IncludeSubdomains:     true,
    SeedURLs:             []string{"https://example.com"},
    UserAgent:            "MonsterInc-Crawler/1.0",
}

crawler, err := crawler.NewCrawler(cfg, logger)
if err != nil {
    return err
}

// Start crawling with context for interrupt handling
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

// Context cancellation will immediately stop:
// - URL batch processing
// - Seed URL processing  
// - URL discovery operations
// - Content fetching
crawler.Start(ctx)

// Interrupt handling example
go func() {
    <-interruptSignal
    cancel() // This will stop crawler immediately
}()

// Get discovered URLs
discoveredURLs := crawler.GetDiscoveredURLs()
fmt.Printf("Discovered %d URLs\n", len(discoveredURLs))

// Get parent URL for a discovered URL
parentURL := crawler.GetRootTargetForDiscoveredURL("https://example.com/page1")
```

#### Builder Pattern
```go
// Use builder for advanced configuration
builder := crawler.NewCrawlerBuilder(logger)
crawler, err := builder.
    WithConfig(cfg).
    Build()
```

### 2. Asset Extraction (`asset.go`)
#### Purpose
- Extract assets from HTML content (images, scripts, stylesheets, links)
- Support for srcset and data-* attributes
- URL resolution and validation
- Asset type detection

#### API Usage

```go
// Extract assets from HTML
htmlContent := []byte("<html><img src='image.jpg'><script src='app.js'></script></html>")
baseURL, _ := url.Parse("https://example.com")

assets := crawler.ExtractAssetsFromHTML(htmlContent, baseURL, crawlerInstance)

for _, asset := range assets {
    fmt.Printf("Found %s: %s\n", asset.Type, asset.AbsoluteURL)
}

// Custom asset extractor
extractor := crawler.NewHTMLAssetExtractor(baseURL, crawlerInstance)
assets = extractor.Extract(htmlContent)
```

#### Asset Types
- `AssetTypeImage`: Images (img, picture sources)
- `AssetTypeScript`: JavaScript files
- `AssetTypeStylesheet`: CSS files
- `AssetTypeLink`: Links and anchors
- `AssetTypeMedia`: Audio and video files
- `AssetTypeDocument`: Document links (PDF, DOC, etc.)

### 3. Headless Browser Management (`headless_browser.go`)
#### Purpose
- Browser pool management for JavaScript-heavy sites
- Dynamic content rendering
- Configurable browser options
- Resource pooling for performance

#### API Usage

```go
// Configuration
browserConfig := config.HeadlessBrowserConfig{
    Enabled:             true,
    ChromePath:          "/usr/bin/google-chrome",
    PoolSize:            3,
    WindowWidth:         1920,
    WindowHeight:        1080,
    PageLoadTimeoutSecs: 30,
    DisableImages:       true,
    DisableCSS:          true,
}

// Create manager
manager := crawler.NewHeadlessBrowserManager(browserConfig, logger)
err := manager.Start()
defer manager.Stop()

// Crawl page with headless browser
result, err := manager.CrawlPage(ctx, "https://example.com")
if err != nil {
    return err
}

fmt.Printf("Rendered HTML: %d bytes\n", len(result.HTML))
fmt.Printf("Page title: %s\n", result.Title)
```

### 4. Scope Management (`scope.go`)
#### Purpose
- Define crawling boundaries and rules
- Hostname and subdomain filtering
- File extension restrictions
- Path-based scope validation

#### API Usage

```go
// Create scope settings
scope, err := crawler.NewScopeSettings(
    "example.com",                    // root hostname
    []string{"ads.example.com"},      // disallowed hostnames
    []string{"cdn", "static"},        // disallowed subdomains
    []string{".jpg", ".png", ".css"}, // disallowed extensions
    logger,
    true,                             // include subdomains
    true,                             // auto-add seed hostnames
    []string{"https://example.com"},  // original seed URLs
)

// Check if URL is in scope
allowed, err := scope.IsURLAllowed("https://blog.example.com/post1")
if allowed {
    fmt.Println("URL is in scope")
}

// Check hostname scope
hostnameAllowed := scope.CheckHostnameScope("api.example.com")
```

### 5. URL Discovery (`discovery.go`)
#### Purpose
- Intelligent URL queuing and deduplication
- Content-Length checking before crawling
- URL normalization and validation
- Parent-child URL relationship tracking

#### API Usage

```go
// Discover URL from HTML parsing
baseURL, _ := url.Parse("https://example.com")
crawler.DiscoverURL("/api/v1/users", baseURL)

// Manual URL discovery
crawler.DiscoverURL("https://example.com/admin", nil)

// Track URL relationships
crawler.TrackURLParent("https://example.com/child", "https://example.com/parent")

// Get root target for discovered URL
rootTarget := crawler.GetRootTargetForDiscoveredURL("https://example.com/deep/path")
```

### 6. Request/Response Handlers (`handlers.go`)
#### Purpose
- Colly callback implementations
- Error handling and retry logic
- Content type detection
- Context cancellation handling

#### API Usage

```go
// The handlers are automatically set up during crawler initialization
// Custom handlers can be added:

crawler.collector.OnRequest(func(r *colly.Request) {
    fmt.Printf("Visiting %s\n", r.URL.String())
})

crawler.collector.OnResponse(func(r *colly.Response) {
    fmt.Printf("Got response from %s: %d\n", r.Request.URL.String(), r.StatusCode)
})

crawler.collector.OnError(func(r *colly.Response, err error) {
    fmt.Printf("Error on %s: %v\n", r.Request.URL.String(), err)
})
```

## Configuration

### Crawler Configuration
```yaml
crawler_config:
  # Basic settings
  max_depth: 3
  max_concurrent_requests: 10
  request_timeout_secs: 30
  include_subdomains: true
  auto_add_seed_hostnames: true
  respect_robots_txt: false
  insecure_skip_tls_verify: false
  
  # Content settings
  enable_content_length_check: true
  max_content_length_mb: 50
  user_agent: "MonsterInc-Crawler/1.0"
  
  # Seed URLs
  seed_urls:
    - "https://example.com"
    - "https://test.example.com"
  
  # Scope configuration
  scope:
    disallowed_hostnames:
      - "ads.example.com"
      - "analytics.example.com"
    disallowed_subdomains:
      - "cdn"
      - "static"
    disallowed_file_extensions:
      - ".jpg"
      - ".png"
      - ".gif"
      - ".css"
      - ".ico"
      - ".pdf"
      - ".zip"
  
  # Headless browser
  headless_browser:
    enabled: true
    chrome_path: "/usr/bin/google-chrome"
    user_data_dir: "/tmp/chrome-data"
    window_width: 1920
    window_height: 1080
    page_load_timeout_secs: 30
    wait_after_load_ms: 2000
    disable_images: true
    disable_css: true
    disable_javascript: false
    ignore_https_errors: true
    pool_size: 3
    browser_args:
      - "--no-sandbox"
      - "--disable-dev-shm-usage"
      - "--disable-extensions"
```

## Advanced Usage

### Custom Asset Extractors
```go
// Define custom asset extractor
type CustomAssetExtractor struct {
    baseURL *url.URL
    crawler *Crawler
}

func (cae *CustomAssetExtractor) Extract(htmlContent []byte) []models.Asset {
    var assets []models.Asset
    
    // Custom extraction logic
    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
    if err != nil {
        return assets
    }
    
    // Extract custom elements
    doc.Find("[data-src]").Each(func(i int, s *goquery.Selection) {
        if dataSrc, exists := s.Attr("data-src"); exists {
            asset := models.Asset{
                AbsoluteURL:    cae.resolveURL(dataSrc),
                SourceTag:      s.Get(0).Data,
                SourceAttr:     "data-src",
                Type:           models.AssetTypeImage,
                DiscoveredAt:   time.Now(),
                DiscoveredFrom: cae.baseURL.String(),
            }
            assets = append(assets, asset)
        }
    })
    
    return assets
}
```

### Custom Scope Rules
```go
// Custom scope validator
type CustomScopeValidator struct {
    allowedPatterns []*regexp.Regexp
    deniedPatterns  []*regexp.Regexp
}

func (csv *CustomScopeValidator) IsAllowed(url string) bool {
    // Check against allowed patterns
    for _, pattern := range csv.allowedPatterns {
        if pattern.MatchString(url) {
            // Check against denied patterns
            for _, deniedPattern := range csv.deniedPatterns {
                if deniedPattern.MatchString(url) {
                    return false
                }
            }
            return true
        }
    }
    return false
}
```

### Crawler Middleware
```go
// Request middleware
func RequestLoggingMiddleware(next colly.RequestCallback) colly.RequestCallback {
    return func(r *colly.Request) {
        start := time.Now()
        r.Ctx.Put("start_time", start)
        
        if next != nil {
            next(r)
        }
    }
}

// Response middleware
func ResponseTimingMiddleware(next colly.ResponseCallback) colly.ResponseCallback {
    return func(r *colly.Response) {
        if startTime, exists := r.Ctx.GetAny("start_time"); exists {
            if start, ok := startTime.(time.Time); ok {
                duration := time.Since(start)
                fmt.Printf("Request to %s took %v\n", r.Request.URL.String(), duration)
            }
        }
        
        if next != nil {
            next(r)
        }
    }
}

// Apply middleware
crawler.collector.OnRequest(RequestLoggingMiddleware(crawler.handleRequest))
crawler.collector.OnResponse(ResponseTimingMiddleware(crawler.handleResponse))
```

## Performance Optimization

### 1. Connection Pooling
```go
// Configure HTTP client for optimal performance
config := &config.CrawlerConfig{
    MaxConcurrentRequests: 50,
    RequestTimeoutSecs:    10,
}

// The crawler automatically configures connection pooling
```

### 2. Memory Management
```go
// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

// Monitor crawler progress
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            urls := crawler.GetDiscoveredURLs()
            fmt.Printf("Discovered %d URLs so far\n", len(urls))
        case <-ctx.Done():
            return
        }
    }
}()
```

### 3. Resource Limiting
```yaml
crawler_config:
  max_concurrent_requests: 20    # Limit concurrent requests
  max_content_length_mb: 10      # Skip large files
  request_timeout_secs: 15       # Timeout slow requests
  enable_content_length_check: true  # Check content size before download
```

## Extension Patterns

### 1. Custom Crawlers
```go
// Specialized crawler interface
type SpecializedCrawler interface {
    CrawlAPI(endpoint string) ([]models.Asset, error)
    CrawlSPA(url string) ([]models.Asset, error)
    CrawlWithAuth(url string, token string) ([]models.Asset, error)
}

// API crawler implementation
type APICrawler struct {
    *Crawler
    httpClient *common.FastHTTPClient
}

func (ac *APICrawler) CrawlAPI(endpoint string) ([]models.Asset, error) {
    // Custom API crawling logic
    resp, err := ac.httpClient.Do(&common.HTTPRequest{
        URL:    endpoint,
        Method: "GET",
        Headers: map[string]string{
            "Accept": "application/json",
        },
    })
    
    if err != nil {
        return nil, err
    }
    
    // Parse JSON and extract URLs
    var assets []models.Asset
    // ... extraction logic
    
    return assets, nil
}
```

### 2. Plugin System
```go
// Crawler plugin interface
type CrawlerPlugin interface {
    Name() string
    OnRequest(*colly.Request) error
    OnResponse(*colly.Response) error
    OnAssetDiscovered(models.Asset) error
}

// Plugin manager
type PluginManager struct {
    plugins []CrawlerPlugin
}

func (pm *PluginManager) Register(plugin CrawlerPlugin) {
    pm.plugins = append(pm.plugins, plugin)
}

func (pm *PluginManager) ExecuteOnRequest(r *colly.Request) {
    for _, plugin := range pm.plugins {
        if err := plugin.OnRequest(r); err != nil {
            log.Printf("Plugin %s error: %v", plugin.Name(), err)
        }
    }
}
```

### 3. Storage Backends
```go
// URL storage interface
type URLStorage interface {
    Store(url string) error
    Exists(url string) bool
    GetAll() ([]string, error)
    Clear() error
}

// Redis storage implementation
type RedisURLStorage struct {
    client *redis.Client
    setKey string
}

func (rus *RedisURLStorage) Store(url string) error {
    return rus.client.SAdd(context.Background(), rus.setKey, url).Err()
}

func (rus *RedisURLStorage) Exists(url string) bool {
    result := rus.client.SIsMember(context.Background(), rus.setKey, url)
    return result.Val()
}
```

## Error Handling

### Common Error Scenarios
1. **Network timeouts**: Configure appropriate timeouts
2. **Rate limiting**: Implement backoff strategies
3. **Memory issues**: Monitor and limit resource usage
4. **Invalid URLs**: Validate URLs before processing
5. **Access denied**: Handle 403/401 responses gracefully

### Error Recovery
```go
// Implement retry logic
type RetryConfig struct {
    MaxAttempts int
    BackoffBase time.Duration
    MaxBackoff  time.Duration
}

func (c *Crawler) crawlWithRetry(url string, config RetryConfig) error {
    var lastErr error
    
    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(math.Pow(2, float64(attempt))) * config.BackoffBase
            if backoff > config.MaxBackoff {
                backoff = config.MaxBackoff
            }
            time.Sleep(backoff)
        }
        
        err := c.collector.Visit(url)
        if err == nil {
            return nil
        }
        
        lastErr = err
        c.logger.Warn().Err(err).Int("attempt", attempt+1).Str("url", url).Msg("Crawl attempt failed")
    }
    
    return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
```

## Best Practices

1. **Scope Definition**: Clearly define crawling scope to avoid infinite loops
2. **Rate Limiting**: Respect target servers with appropriate delays
3. **Resource Management**: Monitor memory and CPU usage
4. **Error Handling**: Implement robust error handling and recovery
5. **Context Usage**: Always use context for cancellation support
6. **Logging**: Log important events and errors for debugging
7. **Testing**: Test crawling logic with mock servers
8. **Robots.txt**: Respect robots.txt when appropriate

## Dependencies
- `github.com/gocolly/colly/v2`: Core crawling framework
- `github.com/go-rod/rod`: Headless browser automation
- `github.com/PuerkitoBio/goquery`: HTML parsing and manipulation
- `github.com/rs/zerolog`: Logging framework