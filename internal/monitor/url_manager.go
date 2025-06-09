package monitor

import (
	"sync"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// URLManager manages the URLs being monitored using target management patterns
type URLManager struct {
	logger        zerolog.Logger
	targetManager *urlhandler.TargetManager
	monitorUrls   map[string]struct{}
	urlsMutex     sync.RWMutex
}

// NewURLManager creates a new URLManager
func NewURLManager(logger zerolog.Logger) *URLManager {
	return &URLManager{
		logger:        logger.With().Str("component", "URLManager").Logger(),
		targetManager: urlhandler.NewTargetManager(logger),
		monitorUrls:   make(map[string]struct{}),
		urlsMutex:     sync.RWMutex{},
	}
}

// AddURL adds a URL to the list of monitored URLs
func (um *URLManager) AddURL(url string) {
	if !um.isValidURL(url) {
		return
	}

	um.urlsMutex.Lock()
	defer um.urlsMutex.Unlock()

	um.monitorUrls[url] = struct{}{}
	um.logURLAdded(url)
}

// GetCurrentURLs returns a copy of currently monitored URLs
func (um *URLManager) GetCurrentURLs() []string {
	um.urlsMutex.RLock()
	defer um.urlsMutex.RUnlock()

	return um.extractURLsFromMap()
}

// PreloadURLs adds multiple URLs to the monitored list using target management
func (um *URLManager) PreloadURLs(initialURLs []string) {
	if len(initialURLs) == 0 {
		return
	}

	targets := um.convertURLsToTargets(initialURLs)
	validURLs := um.extractValidURLs(targets)

	um.addURLsToMonitorList(validURLs)
	um.logPreloadedURLs(len(validURLs), len(initialURLs))
}

// LoadAndMonitorFromSources loads targets from best available source and adds them to monitoring
func (um *URLManager) LoadAndMonitorFromSources(inputFileOption string) error {
	targets, source, err := um.targetManager.LoadAndSelectTargets(inputFileOption)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		um.logger.Warn().Str("source", source).Msg("No targets loaded from source")
		return nil
	}

	urls := um.targetManager.GetTargetStrings(targets)
	um.PreloadURLs(urls)

	um.logger.Info().
		Int("count", len(urls)).
		Str("source", source).
		Msg("Loaded and added targets to monitoring")

	return nil
}

// IsMonitored checks if a URL is being monitored
func (um *URLManager) IsMonitored(url string) bool {
	um.urlsMutex.RLock()
	defer um.urlsMutex.RUnlock()

	_, exists := um.monitorUrls[url]
	return exists
}

// Count returns the number of monitored URLs
func (um *URLManager) Count() int {
	um.urlsMutex.RLock()
	defer um.urlsMutex.RUnlock()

	return len(um.monitorUrls)
}

// RemoveURL removes a URL from monitoring
func (um *URLManager) RemoveURL(url string) bool {
	um.urlsMutex.Lock()
	defer um.urlsMutex.Unlock()

	if _, exists := um.monitorUrls[url]; exists {
		delete(um.monitorUrls, url)
		um.logger.Debug().Str("url", url).Msg("URL removed from monitor list")
		return true
	}
	return false
}

// UpdateLogger updates the logger for this component
func (um *URLManager) UpdateLogger(newLogger zerolog.Logger) {
	um.logger = newLogger.With().Str("component", "URLManager").Logger()
}

// Private helper methods

func (um *URLManager) isValidURL(url string) bool {
	return url != ""
}

func (um *URLManager) logURLAdded(url string) {
	um.logger.Debug().Str("url", url).Msg("URL added to monitor list")
}

func (um *URLManager) extractURLsFromMap() []string {
	urls := make([]string, 0, len(um.monitorUrls))
	for url := range um.monitorUrls {
		urls = append(urls, url)
	}
	return urls
}

func (um *URLManager) convertURLsToTargets(urls []string) []models.Target {
	targets := make([]models.Target, 0, len(urls))
	for _, url := range urls {
		if um.isValidURL(url) {
			// Reuse target manager's URL normalization logic
			target := models.Target{
				OriginalURL:   url,
				NormalizedURL: url, // Will be normalized by target manager if needed
			}
			targets = append(targets, target)
		}
	}
	return targets
}

func (um *URLManager) extractValidURLs(targets []models.Target) []string {
	urls := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.NormalizedURL != "" {
			urls = append(urls, target.NormalizedURL)
		}
	}
	return urls
}

func (um *URLManager) addURLsToMonitorList(urls []string) {
	um.urlsMutex.Lock()
	defer um.urlsMutex.Unlock()

	for _, url := range urls {
		um.monitorUrls[url] = struct{}{}
	}
}

func (um *URLManager) logPreloadedURLs(validCount, totalCount int) {
	um.logger.Info().
		Int("valid_count", validCount).
		Int("total_count", totalCount).
		Msg("Preloaded URLs for monitoring")
}
