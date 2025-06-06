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

// HeadlessBrowserManager manages a pool of browser instances for crawling
type HeadlessBrowserManager struct {
	config      config.HeadlessBrowserConfig
	logger      zerolog.Logger
	browserPool chan *rod.Browser
	launcher    *launcher.Launcher
	mutex       sync.Mutex
	isRunning   bool
}

// NewHeadlessBrowserManager creates a new headless browser manager
func NewHeadlessBrowserManager(cfg config.HeadlessBrowserConfig, logger zerolog.Logger) *HeadlessBrowserManager {
	return &HeadlessBrowserManager{
		config:      cfg,
		logger:      logger.With().Str("component", "HeadlessBrowserManager").Logger(),
		browserPool: make(chan *rod.Browser, cfg.PoolSize),
		isRunning:   false,
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

	// Create browser pool
	for i := 0; i < hbm.config.PoolSize; i++ {
		browser := rod.New().ControlURL(launcherURL)
		if err := browser.Connect(); err != nil {
			hbm.logger.Error().Err(err).Int("browser_index", i).Msg("Failed to connect browser")
			continue
		}

		hbm.browserPool <- browser
		hbm.logger.Debug().Int("browser_index", i).Msg("Browser instance created and added to pool")
	}

	hbm.isRunning = true
	hbm.logger.Info().Int("pool_size", hbm.config.PoolSize).Msg("Headless browser manager started")
	return nil
}

// Stop closes all browser instances and the launcher
func (hbm *HeadlessBrowserManager) Stop() {
	hbm.mutex.Lock()
	defer hbm.mutex.Unlock()

	if !hbm.isRunning {
		return
	}

	// Close all browsers in pool
	close(hbm.browserPool)
	for browser := range hbm.browserPool {
		if browser != nil {
			browser.Close()
		}
	}

	// Close launcher
	if hbm.launcher != nil {
		hbm.launcher.Cleanup()
	}

	hbm.isRunning = false
	hbm.logger.Info().Msg("Headless browser manager stopped")
}

// GetBrowser gets a browser from the pool
func (hbm *HeadlessBrowserManager) GetBrowser() (*rod.Browser, error) {
	if !hbm.config.Enabled || !hbm.isRunning {
		return nil, fmt.Errorf("headless browser manager not running or disabled")
	}

	select {
	case browser := <-hbm.browserPool:
		return browser, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for browser from pool")
	}
}

// ReturnBrowser returns a browser to the pool
func (hbm *HeadlessBrowserManager) ReturnBrowser(browser *rod.Browser) {
	if !hbm.isRunning || browser == nil {
		return
	}

	select {
	case hbm.browserPool <- browser:
		// Successfully returned to pool
	default:
		// Pool is full, close the browser
		browser.Close()
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
func (hbm *HeadlessBrowserManager) CrawlPage(ctx context.Context, url string) (*HeadlessCrawlResult, error) {
	if !hbm.config.Enabled {
		return nil, fmt.Errorf("headless browser is disabled")
	}

	browser, err := hbm.GetBrowser()
	if err != nil {
		return nil, fmt.Errorf("failed to get browser: %w", err)
	}
	defer hbm.ReturnBrowser(browser)

	// Create page with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(hbm.config.PageLoadTimeoutSecs)*time.Second)
	defer cancel()

	page, err := browser.Context(timeoutCtx).Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

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
	finalURL := page.MustInfo().URL

	return &HeadlessCrawlResult{
		HTML:  html,
		Title: title,
		URL:   finalURL,
		Error: nil,
	}, nil
}
