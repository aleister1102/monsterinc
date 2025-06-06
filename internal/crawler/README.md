# Crawler Module

## Overview

The Crawler module provides web crawling capabilities with support for both traditional HTTP crawling and headless browser crawling for JavaScript-heavy pages.

## Features

### Traditional Crawling
- Fast HTTP-based crawling using Colly
- Configurable scope and depth limits
- Asset extraction from HTML pages
- URL batching for improved performance

### Headless Browser Crawling
- Automatic detection of JavaScript-heavy pages
- Browser pool management for efficient resource usage
- Support for dynamic content rendering
- Configurable browser settings

## Configuration

### Basic Crawler Configuration

```yaml
crawler_config:
  seed_urls:
    - https://example.com
  max_depth: 3
  max_concurrent_requests: 10
  request_timeout_secs: 30
```

### Headless Browser Configuration

```yaml
crawler_config:
  headless_browser:
    enabled: true                    # Enable headless browser crawling
    chrome_path: ""                  # Path to Chrome executable (optional)
    user_data_dir: ""               # Browser user data directory (optional)
    window_width: 1920              # Browser window width
    window_height: 1080             # Browser window height
    page_load_timeout_secs: 30      # Page load timeout
    wait_after_load_ms: 1000        # Additional wait after page load
    disable_images: true            # Disable image loading for speed
    disable_css: false              # Disable CSS loading
    disable_javascript: false       # Disable JavaScript execution
    ignore_https_errors: true       # Ignore HTTPS certificate errors
    pool_size: 3                    # Number of browser instances in pool
    browser_args:                   # Additional browser arguments
      - "--no-sandbox"
      - "--disable-dev-shm-usage"
      - "--disable-gpu"
```

## How It Works

### Automatic Detection
The crawler automatically detects when to use headless browser based on page content:
- Presence of `<script>` tags
- JavaScript frameworks (Angular, React, Vue, jQuery)
- AJAX patterns (fetch, XMLHttpRequest)
- DOM manipulation methods

### Browser Pool Management
- Maintains a pool of browser instances for efficiency
- Automatic browser instance recycling
- Graceful shutdown and cleanup

### Content Processing
1. Traditional crawler fetches the page
2. If JavaScript indicators are detected, headless browser is used
3. Rendered HTML is compared with original
4. Additional assets are extracted from rendered content

## Usage Example

```go
// Create crawler with headless browser enabled
config := &config.CrawlerConfig{
    SeedURLs: []string{"https://spa-example.com"},
    HeadlessBrowser: config.HeadlessBrowserConfig{
        Enabled: true,
        PoolSize: 3,
    },
}

crawler, err := NewCrawler(config, logger)
if err != nil {
    log.Fatal(err)
}

// Start crawling
ctx := context.Background()
crawler.Start(ctx)

// Get discovered URLs
urls := crawler.GetDiscoveredURLs()
```

## Performance Considerations

### When to Enable Headless Browser
- Single Page Applications (SPAs)
- JavaScript-heavy websites
- Sites with dynamic content loading
- AJAX-based navigation

### When to Disable Headless Browser
- Static websites
- Simple HTML pages
- Performance-critical scenarios
- Resource-constrained environments

### Resource Usage
- Each browser instance uses ~50-100MB RAM
- CPU usage increases with JavaScript execution
- Network usage may increase due to additional requests

## Troubleshooting

### Common Issues

1. **Browser Launch Failures**
   - Ensure Chrome/Chromium is installed
   - Check `chrome_path` configuration
   - Verify browser arguments compatibility

2. **Timeout Issues**
   - Increase `page_load_timeout_secs`
   - Adjust `wait_after_load_ms`
   - Check network connectivity

3. **Resource Exhaustion**
   - Reduce `pool_size`
   - Enable `disable_images`
   - Limit concurrent requests

### Debug Logging
Enable debug logging to see headless browser activity:

```yaml
log_config:
  log_level: debug
```

## Dependencies

- [go-rod/rod](https://github.com/go-rod/rod) - Browser automation
- [gocolly/colly](https://github.com/gocolly/colly) - Web crawling framework 