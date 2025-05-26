# Crawler Package (`internal/crawler`)

The `crawler` package is responsible for discovering URLs starting from a set of seed URLs. It uses the `gocolly/colly` library for web crawling and `goquery` for HTML parsing to extract links and other assets.

## Key Features

-   **Seed-based Crawling**: Starts crawling from a list of initial URLs.
-   **Configurable Depth**: Limits how many links deep the crawler will go from the seed URLs.
-   **Concurrency**: Supports configurable concurrent requests.
-   **User-Agent Customization**: Allows setting a custom User-Agent string.
-   **Request Timeouts**: Configurable timeout for each HTTP request.
-   **Scope Management**:
    -   Allows defining allowed and disallowed hostnames and subdomains.
    -   Supports regex patterns for allowed and disallowed URL paths.
-   **Robots.txt**: Option to respect `robots.txt` directives (currently configurable, default might be to ignore for wider discovery in a security testing context).
-   **Content Length Limits**: Avoids downloading excessively large files.
-   **Asset Extraction**: Extracts various assets from HTML pages, including links (`<a>`), scripts (`<script>`), stylesheets (`<link rel="stylesheet">`), images (`<img>`), iframes (`<iframe>`), forms (`<form>`), etc. The types of extracted assets are defined in `internal/models/asset.go`.
-   **URL De-duplication**: Keeps track of discovered URLs to avoid processing the same URL multiple times.
-   **Logging**: Integrates with the application's `zerolog` logger for detailed operational logging.

## Initialization

The crawler is initialized using `crawler.NewCrawler(cfg *config.CrawlerConfig, httpClient *http.Client, logger zerolog.Logger)`.

-   `cfg`: An instance of `config.CrawlerConfig` containing all crawler-specific settings (see `internal/config/config.go` and `config.example.yaml`).
-   `httpClient`: An `*http.Client` to be used by the underlying `colly` collector. If `nil` is passed, `colly` will use its default client.
-   `logger`: A `zerolog.Logger` instance for logging.

## Usage

1.  **Initialize**: Create a new crawler instance.
    ```go
    import (
        "monsterinc/internal/config"
        "monsterinc/internal/crawler"
        "github.com/rs/zerolog/log" // Example main logger
        "net/http"
    )

    // Load or create crawlerConfig (e.g., from globalConfig.CrawlerConfig)
    crawlerCfg := config.NewDefaultCrawlerConfig() // Or from loaded config
    crawlerCfg.SeedURLs = []string{"https://example.com"}
    // ... other configurations ...

    appLogger := log.Logger // Your main application logger
    crawlerInstance, err := crawler.NewCrawler(&crawlerCfg, http.DefaultClient, appLogger)
    if err != nil {
        // Handle error
    }
    ```

2.  **Start Crawling**:
    ```go
    crawlerInstance.Start() // This is a blocking call
    ```
    Typically, you would run `Start()` in a goroutine if you need to perform other actions concurrently.

3.  **Get Discovered URLs**: After the crawl is complete (or even during, with appropriate locking if needed, though `GetDiscoveredURLs` has its own lock):
    ```go
    discoveredURLs := crawlerInstance.GetDiscoveredURLs()
    for _, u := range discoveredURLs {
        // Process URL
    }
    ```

## Asset Extraction

The `ExtractAssetsFromHTML(htmlContent []byte, basePageURL *url.URL, crawlerInstance *Crawler) []models.Asset` function is used internally by the crawler when an HTML page is processed. It can also be used independently if needed.

-   `htmlContent`: The byte slice of the HTML page.
-   `basePageURL`: The URL of the page from which the content was fetched, used to resolve relative links.
-   `crawlerInstance`: A pointer to the `Crawler` instance, primarily for logging purposes.

It returns a slice of `models.Asset` structs, where each asset includes its absolute URL, source tag, source attribute, asset type, discovery time, and the URL of the page it was discovered on.

## Scope Control

The crawler's scope is managed by `ScopeSettings` (see `internal/crawler/scope.go`). This allows for fine-grained control over:
-   Which hostnames are allowed or disallowed.
-   Which subdomains (of allowed hostnames) are allowed or disallowed.
-   Regex patterns for URL paths that are allowed or disallowed.

This ensures the crawler stays within the intended boundaries of the scan.

## Dependencies

-   `gocolly/colly`: For the core crawling mechanism.
-   `PuerkitoBio/goquery`: For HTML parsing.
-   `monsterinc/internal/config`: For crawler configuration.
-   `monsterinc/internal/models`: For `Asset` and `AssetType` definitions.
-   `monsterinc/internal/urlhandler`: For URL resolution and normalization.
-   `github.com/rs/zerolog`: For logging.

## Overview

The `crawler` package implements the web crawling functionality for MonsterInc. It uses the `gocolly/colly` library as its underlying crawling engine and integrates with other local packages for configuration (`config`), URL handling (`urlhandler`), and data models (`models`).

Its main responsibilities include:
- Initializing and configuring the crawler based on `config.CrawlerConfig`.
- Managing the crawling scope using `ScopeSettings` (hostnames, subdomains, path regexes).
- Discovering new URLs from HTML assets (links, scripts, etc.).
- Handling de-duplication of discovered URLs.
- Adhering to `robots.txt` rules (configurable).
- Collecting basic statistics about the crawl (visited pages, errors).

## Core Components

### 1. `Crawler` Struct

This is the main struct representing the web crawler instance.

```go
package crawler

import (
    "net/url"
    "sync"
    "time"
    "monsterinc/internal/config"
    "github.com/gocolly/colly/v2"
)

type Crawler struct {
    Collector        *colly.Collector // The underlying gocolly collector
    discoveredURLs   map[string]bool  // For de-duplication of URLs discovered by this crawler logic
    mutex            sync.RWMutex     // To protect discoveredURLs
    UserAgent        string
    RequestTimeout   time.Duration
    Threads          int
    MaxDepth         int
    seedURLs         []string
    totalVisited     int              // Count of pages gocolly responded to (OnResponse)
    totalErrors      int              // Count of errors from gocolly (OnError)
    crawlStartTime   time.Time
    Scope            *ScopeSettings   // Defines what URLs are in scope for crawling
    RespectRobotsTxt bool             // Whether to obey robots.txt rules
}
```

### 2. `ScopeSettings` Struct

Defines the rules for what URLs the crawler is allowed to visit.

```go
package crawler

import "regexp"

type ScopeSettings struct {
    AllowedHostnames     []string
    AllowedSubdomains    []string
    DisallowedHostnames  []string
    DisallowedSubdomains []string
    AllowedPathPatterns    []*regexp.Regexp
    DisallowedPathPatterns []*regexp.Regexp
}
```

## Key Functions and Methods

### Initialization

1.  **`NewCrawler(cfg *config.CrawlerConfig) (*Crawler, error)`**
    *   **Purpose:** Initializes a new `Crawler` instance.
    *   **Operation:**
        1.  Validates the input `cfg` (e.g., checks for nil config, presence of seed URLs).
        2.  Sets up crawler parameters (UserAgent, Timeout, Threads, MaxDepth) from `cfg`, using defaults from `config` package if necessary.
        3.  Initializes `ScopeSettings` using `NewScopeSettings` with rules from `cfg.Scope`.
        4.  Configures and creates a `colly.Collector` with options like `Async(true)`, `UserAgent`, `MaxDepth`, and `IgnoreRobotsTxt` (if `cfg.RespectRobotsTxt` is false).
        5.  Sets the request timeout and limit rule (parallelism) on the Colly collector.
        6.  Sets up Colly callbacks:
            *   `OnError`: Increments `totalErrors` and logs the error.
            *   `OnRequest`: Logs the URL being visited.
            *   `OnResponse`: Increments `totalVisited`. If the content type is HTML, it calls `ExtractAssetsFromHTML` to find more links.
        7.  Logs the successful initialization with key configuration parameters.
    *   **Returns:** A pointer to the new `Crawler` and an error if initialization fails.
    *   **Usage:**
        ```go
        crawlerConfig := &config.CrawlerConfig{ /* ... populate ... */ }
        cr, err := crawler.NewCrawler(crawlerConfig)
        if err != nil {
            // Handle error
        }
        ```

### Crawling Lifecycle

1.  **`Crawler.Start()`**
    *   **Purpose:** Initiates the crawling process.
    *   **Operation:**
        1.  Records the `crawlStartTime`.
        2.  Iterates through the `seedURLs`:
            *   Resolves each seed URL using `urlhandler.ResolveURL` to ensure it's absolute.
            *   Calls `DiscoverURL` to add the (potentially) new seed to the crawl queue.
        3.  Calls `cr.Collector.Wait()` to block until all crawling operations by Colly are complete.
        4.  Calls `cr.logSummary()` to print statistics.
    *   **Usage:**
        ```go
        err := cr.Start() // Assuming cr is an initialized Crawler
        // This call will block until crawling is finished.
        ```

2.  **`Crawler.DiscoverURL(rawURL string, base *url.URL)`**
    *   **Purpose:** Attempts to add a new URL to the crawl queue if it's in scope and not yet discovered.
    *   **Operation:**
        1.  Resolves `rawURL` against `base` using `urlhandler.ResolveURL` to get an absolute URL.
        2.  Trims and checks if the resolved URL is empty; if so, returns.
        3.  **Scope Check:** If `cr.Scope` is set, calls `cr.Scope.IsURLAllowed()`.
            *   If there's an error during the scope check or the URL is not allowed, logs it and returns.
        4.  **De-duplication:** Checks if the normalized absolute URL already exists in `cr.discoveredURLs` (with RLock).
        5.  If it doesn't exist, acquires a write lock (`cr.mutex.Lock()`), double-checks again.
        6.  If still new, adds the URL to `cr.discoveredURLs`, logs it, and then calls `cr.Collector.Visit(normalizedAbsURL)` to queue it for Colly.
        7.  Logs any errors from `cr.Collector.Visit()`, excluding "already visited" and `colly.ErrRobotsTxtBlocked`.
    *   **Note:** This method is called for seed URLs and for URLs extracted from HTML assets.

### Scope Management

1.  **`NewScopeSettings(...) *ScopeSettings`**
    *   **Purpose:** Creates a `ScopeSettings` struct.
    *   **Operation:** Takes slices of allowed/disallowed hostnames, subdomains, and path regex strings. It normalizes hostnames (lowercase, trim) and compiles path regex strings into `*regexp.Regexp` objects. Errors during regex compilation are logged.

2.  **`ScopeSettings.IsURLAllowed(urlString string) (bool, error)`**
    *   **Purpose:** Determines if a given URL string is within the defined scope.
    *   **Operation:**
        1.  Parses the `urlString`. Returns an error if parsing fails or the URL is empty.
        2.  Returns an error if the URL is not absolute (as hostname scope check requires an absolute URL).
        3.  Returns an error if the parsed URL has no hostname component.
        4.  Calls `CheckHostnameScope()` with the URL's hostname.
        5.  If hostname is allowed, calls `checkPathScope()` with the URL's path (defaults to "/" if empty).
        6.  Returns `true` if both hostname and path are allowed, `false` otherwise. An error is returned if preliminary parsing/validation fails.

3.  **`ScopeSettings.CheckHostnameScope(hostname string) bool`**
    *   **Purpose:** Evaluates if a hostname is within the configured hostname/subdomain scope.
    *   **Operation:**
        1.  Checks against `DisallowedHostnames` and `DisallowedSubdomains` (using `urlhandler.IsDomainOrSubdomain`).
        2.  If not disallowed:
            *   If `AllowedHostnames` is empty, returns `true`.
            *   Otherwise, checks if the hostname matches an entry in `AllowedHostnames` or, if it's a subdomain of an allowed host, checks against `AllowedSubdomains` (if `AllowedSubdomains` is not empty, the subdomain must be explicitly listed).

4.  **`ScopeSettings.checkPathScope(path string) bool`**
    *   **Purpose:** Evaluates if a URL path matches the configured path regexes.
    *   **Operation:**
        1.  Checks against `DisallowedPathPatterns`. If any match, returns `false`.
        2.  If `AllowedPathPatterns` is defined and not empty, the path must match at least one allowed pattern to return `true`. If no allowed patterns match, returns `false`.
        3.  If `AllowedPathPatterns` is empty (and not disallowed), returns `true`.

### Asset Extraction

1.  **`ExtractAssetsFromHTML(htmlBody io.Reader, basePageURL *url.URL, crawlerInstance *Crawler) ([]models.ExtractedAsset, error)`**
    *   **Purpose:** Parses an HTML document and extracts URLs from predefined tags and attributes.
    *   **Operation:**
        1.  Uses `goquery.NewDocumentFromReader` to parse the `htmlBody`.
        2.  Iterates over a predefined map `tagsToExtract` (e.g., `"a": "href"`, `"script": "src"`).
        3.  For each found tag and attribute, extracts the URL value(s) (handles `srcset` specially by calling `parseSrcset`).
        4.  Skips empty, data, mailto, tel, or javascript URLs.
        5.  Resolves the extracted URL against `basePageURL` (if provided) or ensures it's absolute.
        6.  Creates a `models.ExtractedAsset` struct.
        7.  If `crawlerInstance` is provided, calls `crawlerInstance.DiscoverURL()` with the extracted absolute URL and `basePageURL` to potentially add it to the crawl queue.
    *   **Returns:** A slice of `models.ExtractedAsset` and an error if `goquery` parsing fails.

2.  **`parseSrcset(srcset string) []string`**
    *   **Purpose:** Parses a `srcset` attribute string and extracts the URL parts.
    *   **Operation:** Splits the string by commas, then for each part, splits by whitespace and takes the first field (the URL).
    *   **Returns:** A slice of URL strings found in the `srcset`.

### Utility Methods

1.  **`Crawler.GetDiscoveredURLs() []string`**
    *   **Purpose:** Returns a slice of all unique URLs discovered by the crawler's own de-duplication logic.
    *   **Operation:** Reads from `cr.discoveredURLs` with a read lock.

2.  **`Crawler.logSummary()`**
    *   **Purpose:** Logs crawling summary statistics (duration, URLs processed, unique URLs discovered, HTTP errors).
    *   **Operation:** Reads counters and `discoveredURLs` map size with a read lock.

## Error Handling and Logging

- The package uses the standard `log` package for logging, with different levels like `[INFO]`, `[WARN]`, `[ERROR]`, `[DEBUG]` indicated in the log messages.
- Errors from `colly` are caught in `OnError`. Errors from URL resolution, scope checking, and queuing are logged within `DiscoverURL`.
- `NewCrawler` returns errors for invalid configuration.
- `ExtractAssetsFromHTML` returns errors from HTML parsing.
- `ScopeSettings` functions log errors during regex compilation.

## Dependencies

-   **Go Standard Library:** `errors`, `log`, `net/url`, `strings`, `sync`, `time`, `io`, `regexp`.
-   **External Libraries:**
    *   `github.com/gocolly/colly/v2`: The core crawling library.
    *   `github.com/PuerkitoBio/goquery`: For HTML parsing in asset extraction.
-   **Internal Packages:**
    *   `monsterinc/internal/config`: For crawler configuration (`CrawlerConfig`).
    *   `monsterinc/internal/urlhandler`: For URL resolution and validation.
    *   `monsterinc/internal/models`: For data structures like `ExtractedAsset`.

## How to Use

1.  **Create Configuration:** Populate a `config.CrawlerConfig` struct with seed URLs, scope rules, and other crawler parameters.
    ```go
    import "monsterinc/internal/config"

    cfg := &config.CrawlerConfig{
        SeedURLs: []string{"http://example.com"},
        MaxDepth: 2,
        Scope: config.CrawlerScopeConfig{
            AllowedHostnames: []string{"example.com"},
        },
        RespectRobotsTxt: true,
        // ... other parameters
    }
    ```

2.  **Initialize Crawler:** Create a new crawler instance.
    ```go
    import "monsterinc/internal/crawler"

    cr, err := crawler.NewCrawler(cfg)
    if err != nil {
        log.Fatalf("Failed to create crawler: %v", err)
    }
    ```

3.  **Start Crawling:** Run the crawler.
    ```go
    cr.Start() // This is a blocking call
    ```

4.  **Get Results (Optional):** After the crawl finishes, you can retrieve the list of unique URLs discovered by your crawler's logic (this is separate from Colly's internal visited list, but represents what your crawler attempted to queue based on its scope and de-duplication).
    ```go
    discovered := cr.GetDiscoveredURLs()
    log.Printf("Crawler discovered %d unique URLs in scope:", len(discovered))
    // for _, u := range discovered { log.Println(u) }
    ```

The crawler logs its progress and summary to standard output using the `log` package. 