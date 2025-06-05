package monitor

import (
	"sync"

	"github.com/rs/zerolog"
)

// CycleTracker tracks changes within a monitoring cycle
type CycleTracker struct {
	logger         zerolog.Logger
	changedURLs    map[string]struct{}
	currentCycleID string
	mutex          sync.RWMutex
}

// NewCycleTracker creates a new CycleTracker
func NewCycleTracker(logger zerolog.Logger, initialCycleID string) *CycleTracker {
	return &CycleTracker{
		logger:         logger.With().Str("component", "CycleTracker").Logger(),
		changedURLs:    make(map[string]struct{}),
		currentCycleID: initialCycleID,
		mutex:          sync.RWMutex{},
	}
}

// AddChangedURL adds a URL to the changed URLs list for the current cycle
func (ct *CycleTracker) AddChangedURL(url string) {
	if url == "" {
		return
	}

	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	ct.changedURLs[url] = struct{}{}
	ct.logURLChanged(url)
}

// GetChangedURLs returns the list of URLs that changed in the current cycle
func (ct *CycleTracker) GetChangedURLs() []string {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()

	return ct.extractChangedURLs()
}

// ClearChangedURLs clears the changed URLs list for a new cycle
func (ct *CycleTracker) ClearChangedURLs() {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	ct.changedURLs = make(map[string]struct{})
	ct.logChangedURLsCleared()
}

// GetCurrentCycleID returns the current cycle ID
func (ct *CycleTracker) GetCurrentCycleID() string {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()
	return ct.currentCycleID
}

// SetCurrentCycleID sets a new cycle ID
func (ct *CycleTracker) SetCurrentCycleID(cycleID string) {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	ct.currentCycleID = cycleID
	ct.logCycleIDUpdated(cycleID)
}

// HasChanges returns true if there are changes in the current cycle
func (ct *CycleTracker) HasChanges() bool {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()

	return len(ct.changedURLs) > 0
}

// GetChangeCount returns the number of changed URLs in the current cycle
func (ct *CycleTracker) GetChangeCount() int {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()

	return len(ct.changedURLs)
}

// Private helper methods for CycleTracker

func (ct *CycleTracker) logURLChanged(url string) {
	ct.logger.Debug().
		Str("url", url).
		Str("cycle_id", ct.currentCycleID).
		Msg("URL marked as changed in current cycle")
}

func (ct *CycleTracker) extractChangedURLs() []string {
	urls := make([]string, 0, len(ct.changedURLs))
	for url := range ct.changedURLs {
		urls = append(urls, url)
	}
	return urls
}

func (ct *CycleTracker) logChangedURLsCleared() {
	ct.logger.Debug().
		Str("cycle_id", ct.currentCycleID).
		Msg("Cleared changed URLs for new cycle")
}

func (ct *CycleTracker) logCycleIDUpdated(cycleID string) {
	ct.logger.Debug().
		Str("cycle_id", cycleID).
		Msg("Updated current cycle ID")
}
