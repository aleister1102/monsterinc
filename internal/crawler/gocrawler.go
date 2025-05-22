package crawler

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	// "github.com/gocolly/colly/v2/debug"
)

// CrawlerService wraps gocolly functionalities.
// It allows setting common options for crawling.
type CrawlerService struct {
	MaxDepth       int           // Maximum crawl depth
	AllowedDomains []string      // List of allowed domains to crawl
	UserAgent      string        // Custom User-Agent string
	Delay          time.Duration // Delay between requests
	Timeout        time.Duration // Request timeout
	Threads        int           // Number of concurrent threads
	IncludeSubs    bool          // Whether to include subdomains of AllowedDomains
	// TODO: Add more options as needed, e.g., Proxy, Headers
}

// NewCrawlerService creates a new CrawlerService with default settings.
func NewCrawlerService() *CrawlerService {
	return &CrawlerService{
		MaxDepth:    2, // Default depth similar to hakrawler
		UserAgent:   "MonsterIncCrawler/1.0",
		Delay:       0, // No delay by default
		Timeout:     10 * time.Second,
		Threads:     8, // Default threads similar to hakrawler
		IncludeSubs: false,
	}
}

// CrawlURL starts crawling from a given seedURL and returns a slice of found URLs.
func (cs *CrawlerService) CrawlURL(seedURL string) ([]string, error) {
	foundURLs := make(map[string]struct{}) // Use a map to store unique URLs
	var mu sync.Mutex                      // Mutex to protect access to foundURLs

	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is the Măximum depth to crawl
		colly.MaxDepth(cs.MaxDepth),
		// Async allows running multiple scrapers in parallel
		colly.Async(true),
		// UserAgent specifies the user agent string
		colly.UserAgent(cs.UserAgent),
		// TODO: Add more options from CrawlerService struct
	)

	// Set request timeout
	c.SetRequestTimeout(cs.Timeout)

	// Set parallelism
	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*", // Apply to all domains
		Parallelism: cs.Threads,
		Delay:       cs.Delay,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set limit rule: %w", err)
	}

	// Set allowed domains
	if len(cs.AllowedDomains) > 0 {
		c.AllowedDomains = cs.AllowedDomains
	} else {
		// If no specific domains are allowed, allow the domain of the seedURL
		parsedSeedURL, err := url.Parse(seedURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse seed URL for domain restriction: %w", err)
		}
		c.AllowedDomains = []string{parsedSeedURL.Hostname()}
		if cs.IncludeSubs {
			c.AllowedDomains = append(c.AllowedDomains, "www."+parsedSeedURL.Hostname()) // Basic subdomain inclusion
			// More robust subdomain handling might be needed for complex cases
		}
	}

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)
		if absoluteURL != "" {
			mu.Lock()
			if _, exists := foundURLs[absoluteURL]; !exists {
				foundURLs[absoluteURL] = struct{}{}
				// Only visit if it's within allowed domains and depth
				e.Request.Visit(absoluteURL) // This will respect MaxDepth and AllowedDomains
			}
			mu.Unlock()
		}
	})

	// Find and visit all script sources
	c.OnHTML("script[src]", func(e *colly.HTMLElement) {
		link := e.Attr("src")
		absoluteURL := e.Request.AbsoluteURL(link)
		if absoluteURL != "" {
			mu.Lock()
			if _, exists := foundURLs[absoluteURL]; !exists {
				foundURLs[absoluteURL] = struct{}{}
				// Script URLs are typically not visited for further crawling in this context
				// but are collected as assets.
			}
			mu.Unlock()
		}
	})

	// Could add more handlers for forms, iframes, etc. similar to hakrawler's logic if needed

	c.OnError(func(r *colly.Response, err error) {
		// Log or handle errors
		// fmt.Fprintf(os.Stderr, "Request URL: %s failed with response: %v Error: %s\n", r.Request.URL, r, err)
	})

	// Start scraping on seedURL
	if err := c.Visit(seedURL); err != nil {
		// Handle error if the initial visit fails, unless it's a specific "no-host" error which might be expected
		if !strings.Contains(err.Error(), "no Host in request URL") {
			return nil, fmt.Errorf("failed to start crawl from %s: %w", seedURL, err)
		}
	}

	// Wait until threads are finished
	c.Wait()

	// Convert map keys to slice
	resultURLs := make([]string, 0, len(foundURLs))
	for u := range foundURLs {
		resultURLs = append(resultURLs, u)
	}

	return resultURLs, nil
}
