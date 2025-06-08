package crawler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/rs/zerolog"
)

// HeadlessBrowserManager manages a shared browser instance for crawling
type HeadlessBrowserManager struct {
	config        config.HeadlessBrowserConfig
	logger        zerolog.Logger
	sharedBrowser *rod.Browser
	pagePool      chan *rod.Page
	launcher      *launcher.Launcher
	launcherURL   string // Store launcher URL for reuse
	mutex         sync.Mutex
	isRunning     bool
	maxConcurrent int // Max concurrent pages
}

// NewHeadlessBrowserManager creates a new headless browser manager
func NewHeadlessBrowserManager(cfg config.HeadlessBrowserConfig, logger zerolog.Logger) *HeadlessBrowserManager {
	maxConcurrent := cfg.PoolSize // Reuse pool size as max concurrent pages
	if maxConcurrent < 1 {
		maxConcurrent = 3
	}

	return &HeadlessBrowserManager{
		config:        cfg,
		logger:        logger.With().Str("component", "HeadlessBrowserManager").Logger(),
		pagePool:      make(chan *rod.Page, maxConcurrent),
		maxConcurrent: maxConcurrent,
		isRunning:     false,
	}
}

// Start initializes the browser pool
func (hbm *HeadlessBrowserManager) Start() error {
	hbm.mutex.Lock()
	defer hbm.mutex.Unlock()

	if hbm.isRunning {
		return nil
	}

	if !hbm.config.Enabled {
		hbm.logger.Info().Msg("Headless browser is disabled in config")
		return nil
	}

	// Setup launcher
	launcher := launcher.New()

	if hbm.config.ChromePath != "" {
		launcher = launcher.Bin(hbm.config.ChromePath)
	}

	if hbm.config.UserDataDir != "" {
		launcher = launcher.UserDataDir(hbm.config.UserDataDir)
	}

	// Apply browser arguments
	launcher = launcher.
		Set("no-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-gpu").
		Set("disable-web-security").
		Set("disable-features", "VizDisplayCompositor").
		Set("no-first-run").
		Set("disable-default-apps").
		Set("disable-sync")

	// Add user-defined args (skip this for now due to API complexity)
	// User can configure via ChromePath and other specific configs

	if hbm.config.DisableImages {
		launcher = launcher.Set("blink-settings", "imagesEnabled=false")
	}

	// Launch browser
	launcherURL, err := launcher.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	hbm.launcher = launcher
	hbm.launcherURL = launcherURL // Store launcher URL for reuse

	// Create shared browser instance
	sharedBrowser := rod.New().ControlURL(launcherURL)
	if err := sharedBrowser.Connect(); err != nil {
		return fmt.Errorf("failed to connect shared browser: %w", err)
	}
	hbm.sharedBrowser = sharedBrowser

	// Pre-create page pool for better performance
	for i := 0; i < hbm.maxConcurrent; i++ {
		page, err := hbm.sharedBrowser.Page(proto.TargetCreateTarget{})
		if err != nil {
			hbm.logger.Error().Err(err).Int("page_index", i).Msg("Failed to create page")
			continue
		}

		hbm.pagePool <- page
		hbm.logger.Debug().Int("page_index", i).Msg("Page instance created and added to pool")
	}

	hbm.isRunning = true
	hbm.logger.Info().Int("max_concurrent", hbm.maxConcurrent).Msg("Shared headless browser manager started")
	return nil
}

// Stop closes shared browser instance and the launcher
func (hbm *HeadlessBrowserManager) Stop() {
	hbm.mutex.Lock()
	defer hbm.mutex.Unlock()

	if !hbm.isRunning {
		return
	}

	// Close all pages in pool
	close(hbm.pagePool)
	for page := range hbm.pagePool {
		if page != nil {
			err := page.Close()
			if err != nil {
				hbm.logger.Error().Err(err).Msg("Failed to close page")
			}
		}
	}

	// Close shared browser with timeout
	if hbm.sharedBrowser != nil {
		done := make(chan error, 1)
		go func() {
			done <- hbm.sharedBrowser.Close()
		}()

		select {
		case err := <-done:
			if err != nil {
				hbm.logger.Error().Err(err).Msg("Failed to close shared browser")
			}
		case <-time.After(5 * time.Second):
			hbm.logger.Warn().Msg("Timeout closing shared browser, forcing cleanup")
		}
	}

	// Close launcher with timeout
	if hbm.launcher != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			hbm.launcher.Cleanup()
		}()

		select {
		case <-done:
			hbm.logger.Debug().Msg("Launcher cleanup completed")
		case <-time.After(3 * time.Second):
			hbm.logger.Warn().Msg("Timeout cleaning up launcher, forcing exit")
		}
	}

	hbm.isRunning = false
	hbm.logger.Info().Msg("Shared headless browser manager stopped")
}

// GetPage gets a page from the pool or creates a new one
func (hbm *HeadlessBrowserManager) GetPage() (*rod.Page, error) {
	if !hbm.config.Enabled || !hbm.isRunning {
		return nil, fmt.Errorf("headless browser manager not running or disabled")
	}

	// Log pool status for debugging
	poolLen := len(hbm.pagePool)
	hbm.logger.Debug().Int("pool_available", poolLen).Int("max_concurrent", hbm.maxConcurrent).Msg("Getting page from pool")

	select {
	case page := <-hbm.pagePool:
		// Check if page is still alive
		if page == nil {
			hbm.logger.Warn().Msg("Received nil page from pool, creating new one")
			// Try to create a new page as replacement
			if newPage, createErr := hbm.createNewPage(); createErr == nil {
				return newPage, nil
			}
			return nil, fmt.Errorf("received nil page and couldn't create replacement")
		}

		// Check if page is still healthy
		if !hbm.isPageHealthy(page) {
			hbm.logger.Warn().Msg("Page from pool is unhealthy, creating new one")
			// Close the unhealthy page
			if closeErr := page.Close(); closeErr != nil {
				hbm.logger.Error().Err(closeErr).Msg("Failed to close unhealthy page")
			}
			// Try to create a new page as replacement
			if newPage, createErr := hbm.createNewPage(); createErr == nil {
				return newPage, nil
			}
			return nil, fmt.Errorf("received unhealthy page and couldn't create replacement")
		}

		return page, nil
	case <-time.After(15 * time.Second): // Increased timeout
		hbm.logger.Warn().Int("pool_available", poolLen).Int("max_concurrent", hbm.maxConcurrent).Msg("Page pool timeout, attempting to create new page")
		// Try to create a new page as fallback
		if newPage, createErr := hbm.createNewPage(); createErr == nil {
			return newPage, nil
		}
		return nil, fmt.Errorf("timeout waiting for page from pool (available: %d/%d pages)", poolLen, hbm.maxConcurrent)
	}
}

// ReturnPage returns a page to the pool
func (hbm *HeadlessBrowserManager) ReturnPage(page *rod.Page) {
	if !hbm.isRunning || page == nil {
		return
	}

	// Check page health before returning to pool
	if !hbm.isPageHealthy(page) {
		hbm.logger.Warn().Msg("Unhealthy page detected during return, closing instead of returning to pool")
		if err := page.Close(); err != nil {
			hbm.logger.Error().Err(err).Msg("Failed to close unhealthy page")
		}
		return
	}

	// Always try to return to pool, with timeout to avoid blocking
	select {
	case hbm.pagePool <- page:
		hbm.logger.Debug().Msg("Page returned to pool successfully")
	case <-time.After(1 * time.Second):
		// Pool might be full or blocked, close the page to free resources
		hbm.logger.Warn().Msg("Failed to return page to pool (timeout), closing page")
		if err := page.Close(); err != nil {
			hbm.logger.Error().Err(err).Msg("Failed to close page")
		}
	}
}

// IsEnabled returns whether headless browser is enabled
func (hbm *HeadlessBrowserManager) IsEnabled() bool {
	return hbm.config.Enabled
}

// HeadlessCrawlResult contains the result of headless crawling
type HeadlessCrawlResult struct {
	HTML  string
	Title string
	URL   string
	Error error
}

// CrawlPage crawls a page using headless browser
func (hbm *HeadlessBrowserManager) CrawlPage(ctx context.Context, url string) (result *HeadlessCrawlResult, err error) {
	var page *rod.Page
	var pageReturned bool

	// Recover from any panic to prevent crashing the entire crawler
	defer func() {
		if r := recover(); r != nil {
			hbm.logger.Error().Interface("panic", r).Str("url", url).Msg("Recovered from panic in CrawlPage")

			// Ensure page is properly handled even during panic
			if page != nil && !pageReturned {
				// Close page instead of returning to pool due to unknown state
				if closeErr := page.Close(); closeErr != nil {
					hbm.logger.Error().Err(closeErr).Msg("Failed to close page after panic")
				}
			}

			result = &HeadlessCrawlResult{
				URL:   url,
				Error: fmt.Errorf("panic occurred while crawling %s: %v", url, r),
			}
			err = nil
		}
	}()

	if !hbm.config.Enabled {
		return nil, fmt.Errorf("headless browser is disabled")
	}

	page, err = hbm.GetPage()
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	// Ensure page is always returned to pool, even in case of panic
	defer func() {
		if !pageReturned {
			hbm.ReturnPage(page)
		}
	}()

	// Set timeout for page operations
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(hbm.config.PageLoadTimeoutSecs)*time.Second)
	defer cancel()

	// Use context with timeout for page operations
	page = page.Context(timeoutCtx)

	// Set viewport
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  hbm.config.WindowWidth,
		Height: hbm.config.WindowHeight,
	}); err != nil {
		hbm.logger.Warn().Err(err).Msg("Failed to set viewport")
	}

	// Set user agent if configured
	if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (compatible; MonsterInc-Crawler/1.0)",
	}); err != nil {
		hbm.logger.Warn().Err(err).Msg("Failed to set user agent")
	}

	// Navigate to URL
	if err := page.Navigate(url); err != nil {
		return &HeadlessCrawlResult{
			URL:   url,
			Error: fmt.Errorf("failed to navigate to %s: %w", url, err),
		}, nil
	}

	// Wait for page load
	if err := page.WaitLoad(); err != nil {
		return &HeadlessCrawlResult{
			URL:   url,
			Error: fmt.Errorf("page load timeout for %s: %w", url, err),
		}, nil
	}

	// Additional wait after load if configured
	if hbm.config.WaitAfterLoadMs > 0 {
		time.Sleep(time.Duration(hbm.config.WaitAfterLoadMs) * time.Millisecond)
	}

	// Get page content
	html, err := page.HTML()
	if err != nil {
		return &HeadlessCrawlResult{
			URL:   url,
			Error: fmt.Errorf("failed to get HTML for %s: %w", url, err),
		}, nil
	}

	// Get page title
	title := ""
	if titleElement, err := page.Element("title"); err == nil {
		if titleText, err := titleElement.Text(); err == nil {
			title = titleText
		}
	}

	// Get final URL (in case of redirects)
	finalURL := url // Default to original URL
	if info, err := page.Info(); err == nil {
		finalURL = info.URL
	}

	// Mark page as returned before returning result
	pageReturned = true
	hbm.ReturnPage(page)

	return &HeadlessCrawlResult{
		HTML:  html,
		Title: title,
		URL:   finalURL,
		Error: nil,
	}, nil
}

// isPageHealthy checks if a page instance is still healthy and connected
func (hbm *HeadlessBrowserManager) isPageHealthy(page *rod.Page) bool {
	if page == nil {
		return false
	}

	// Try to get page info to check if page is still alive
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to get page info - this will fail if page is disconnected
	_, err := page.Context(ctx).Info()
	return err == nil
}

// createNewPage creates a new page instance for pool replacement
func (hbm *HeadlessBrowserManager) createNewPage() (*rod.Page, error) {
	if hbm.sharedBrowser == nil {
		return nil, fmt.Errorf("shared browser not available")
	}

	// Create new page from shared browser
	page, err := hbm.sharedBrowser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("failed to create new page: %w", err)
	}

	hbm.logger.Debug().Msg("Created new page instance as replacement")
	return page, nil
}
