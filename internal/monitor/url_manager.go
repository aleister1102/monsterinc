package monitor

import (
	"sync"

	"github.com/rs/zerolog"
)

// URLManager manages the URLs being monitored
type URLManager struct {
	logger              zerolog.Logger
	monitorUrls         map[string]struct{}
	monitoredUrlsRMutex sync.RWMutex
}

// NewURLManager creates a new URLManager
func NewURLManager(logger zerolog.Logger) *URLManager {
	return &URLManager{
		logger:              logger.With().Str("component", "URLManager").Logger(),
		monitorUrls:         make(map[string]struct{}),
		monitoredUrlsRMutex: sync.RWMutex{},
	}
}

// AddURL adds a URL to the list of monitored URLs
func (um *URLManager) AddURL(url string) {
	if url == "" {
		return
	}

	um.monitoredUrlsRMutex.Lock()
	defer um.monitoredUrlsRMutex.Unlock()

	um.monitorUrls[url] = struct{}{}
	um.logger.Debug().Str("url", url).Msg("URL added to monitor list")
}

// GetCurrentURLs returns a copy of currently monitored URLs
func (um *URLManager) GetCurrentURLs() []string {
	um.monitoredUrlsRMutex.RLock()
	defer um.monitoredUrlsRMutex.RUnlock()

	urls := make([]string, 0, len(um.monitorUrls))
	for url := range um.monitorUrls {
		urls = append(urls, url)
	}
	return urls
}

// PreloadURLs adds multiple URLs to the monitored list
func (um *URLManager) PreloadURLs(initialURLs []string) {
	for _, url := range initialURLs {
		um.AddURL(url)
	}
	um.logger.Info().Int("count", len(initialURLs)).Msg("Preloaded URLs for monitoring")
}

// IsMonitored checks if a URL is being monitored
func (um *URLManager) IsMonitored(url string) bool {
	um.monitoredUrlsRMutex.RLock()
	defer um.monitoredUrlsRMutex.RUnlock()

	_, exists := um.monitorUrls[url]
	return exists
}

// Count returns the number of monitored URLs
func (um *URLManager) Count() int {
	um.monitoredUrlsRMutex.RLock()
	defer um.monitoredUrlsRMutex.RUnlock()

	return len(um.monitorUrls)
}

// CycleTracker tracks changes within a monitoring cycle
type CycleTracker struct {
	logger                  zerolog.Logger
	changedURLsInCycle      map[string]struct{}
	changedURLsInCycleMutex sync.RWMutex
	currentCycleID          string
	cycleIDMutex            sync.RWMutex
}

// NewCycleTracker creates a new CycleTracker
func NewCycleTracker(logger zerolog.Logger, initialCycleID string) *CycleTracker {
	return &CycleTracker{
		logger:                  logger.With().Str("component", "CycleTracker").Logger(),
		changedURLsInCycle:      make(map[string]struct{}),
		changedURLsInCycleMutex: sync.RWMutex{},
		currentCycleID:          initialCycleID,
		cycleIDMutex:            sync.RWMutex{},
	}
}

// AddChangedURL adds a URL to the changed URLs list for the current cycle
func (ct *CycleTracker) AddChangedURL(url string) {
	ct.changedURLsInCycleMutex.Lock()
	defer ct.changedURLsInCycleMutex.Unlock()

	ct.changedURLsInCycle[url] = struct{}{}
	ct.logger.Debug().Str("url", url).Str("cycle_id", ct.GetCurrentCycleID()).Msg("URL marked as changed in current cycle")
}

// GetChangedURLs returns the list of URLs that changed in the current cycle
func (ct *CycleTracker) GetChangedURLs() []string {
	ct.changedURLsInCycleMutex.RLock()
	defer ct.changedURLsInCycleMutex.RUnlock()

	urls := make([]string, 0, len(ct.changedURLsInCycle))
	for url := range ct.changedURLsInCycle {
		urls = append(urls, url)
	}
	return urls
}

// ClearChangedURLs clears the changed URLs list for a new cycle
func (ct *CycleTracker) ClearChangedURLs() {
	ct.changedURLsInCycleMutex.Lock()
	defer ct.changedURLsInCycleMutex.Unlock()

	ct.changedURLsInCycle = make(map[string]struct{})
	ct.logger.Debug().Str("cycle_id", ct.GetCurrentCycleID()).Msg("Cleared changed URLs for new cycle")
}

// GetCurrentCycleID returns the current cycle ID
func (ct *CycleTracker) GetCurrentCycleID() string {
	ct.cycleIDMutex.RLock()
	defer ct.cycleIDMutex.RUnlock()
	return ct.currentCycleID
}

// SetCurrentCycleID sets a new cycle ID
func (ct *CycleTracker) SetCurrentCycleID(cycleID string) {
	ct.cycleIDMutex.Lock()
	defer ct.cycleIDMutex.Unlock()
	ct.currentCycleID = cycleID
	ct.logger.Debug().Str("cycle_id", cycleID).Msg("Updated current cycle ID")
}

// URLMutexManager manages per-URL mutexes to prevent concurrent processing
type URLMutexManager struct {
	logger               zerolog.Logger
	urlCheckMutexes      map[string]*sync.Mutex
	urlCheckMutexMapLock sync.RWMutex
}

// NewURLMutexManager creates a new URLMutexManager
func NewURLMutexManager(logger zerolog.Logger) *URLMutexManager {
	return &URLMutexManager{
		logger:               logger.With().Str("component", "URLMutexManager").Logger(),
		urlCheckMutexes:      make(map[string]*sync.Mutex),
		urlCheckMutexMapLock: sync.RWMutex{},
	}
}

// GetMutex gets or creates a mutex for a URL
func (umm *URLMutexManager) GetMutex(url string) *sync.Mutex {
	umm.urlCheckMutexMapLock.RLock()
	if mutex, exists := umm.urlCheckMutexes[url]; exists {
		umm.urlCheckMutexMapLock.RUnlock()
		return mutex
	}
	umm.urlCheckMutexMapLock.RUnlock()

	umm.urlCheckMutexMapLock.Lock()
	defer umm.urlCheckMutexMapLock.Unlock()

	// Double-check pattern
	if mutex, exists := umm.urlCheckMutexes[url]; exists {
		return mutex
	}

	umm.urlCheckMutexes[url] = &sync.Mutex{}
	return umm.urlCheckMutexes[url]
}

// CleanupUnusedMutexes removes mutexes for URLs that are no longer monitored
func (umm *URLMutexManager) CleanupUnusedMutexes(activeURLs []string) {
	activeURLSet := make(map[string]struct{})
	for _, url := range activeURLs {
		activeURLSet[url] = struct{}{}
	}

	umm.urlCheckMutexMapLock.Lock()
	defer umm.urlCheckMutexMapLock.Unlock()

	removed := 0
	for url := range umm.urlCheckMutexes {
		if _, isActive := activeURLSet[url]; !isActive {
			delete(umm.urlCheckMutexes, url)
			removed++
		}
	}

	if removed > 0 {
		umm.logger.Debug().
			Int("removed_mutexes", removed).
			Int("remaining_mutexes", len(umm.urlCheckMutexes)).
			Msg("Cleaned up unused URL check mutexes")
	}
}
