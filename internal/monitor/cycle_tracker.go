package monitor

import (
	"sync"
)

// CycleTracker tracks changes within a monitoring cycle
type CycleTracker struct {
	changedURLs    map[string]struct{}
	currentCycleID string
	mutex          sync.RWMutex
}

// NewCycleTracker creates a new CycleTracker
func NewCycleTracker(initialCycleID string) *CycleTracker {
	return &CycleTracker{
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

func (ct *CycleTracker) extractChangedURLs() []string {
	// Pre-allocate with exact capacity to avoid reallocation
	urls := make([]string, 0, len(ct.changedURLs))
	for url := range ct.changedURLs {
		urls = append(urls, url)
	}
	return urls
}
