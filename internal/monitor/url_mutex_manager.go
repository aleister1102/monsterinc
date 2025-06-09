package monitor

import (
	"sync"

	"github.com/rs/zerolog"
)

// URLMutexManager manages per-URL mutexes to prevent concurrent processing
type URLMutexManager struct {
	logger   zerolog.Logger
	mutexes  map[string]*sync.Mutex
	mapMutex sync.RWMutex
}

// NewURLMutexManager creates a new URLMutexManager
func NewURLMutexManager(logger zerolog.Logger) *URLMutexManager {
	return &URLMutexManager{
		logger:   logger.With().Str("component", "URLMutexManager").Logger(),
		mutexes:  make(map[string]*sync.Mutex),
		mapMutex: sync.RWMutex{},
	}
}

// GetMutex gets or creates a mutex for a URL using double-checked locking pattern
func (umm *URLMutexManager) GetMutex(url string) *sync.Mutex {
	// Fast path: read lock to check if mutex exists
	if mutex := umm.tryGetExistingMutex(url); mutex != nil {
		return mutex
	}

	// Slow path: create mutex if it doesn't exist
	return umm.getOrCreateMutex(url)
}

// CleanupUnusedMutexes removes mutexes for URLs that are no longer monitored
func (umm *URLMutexManager) CleanupUnusedMutexes(activeURLs []string) {
	activeURLSet := umm.createActiveURLSet(activeURLs)
	removedCount := umm.removeUnusedMutexes(activeURLSet)

	umm.logCleanupResults(removedCount)
}

// GetMutexCount returns the current number of mutexes
func (umm *URLMutexManager) GetMutexCount() int {
	umm.mapMutex.RLock()
	defer umm.mapMutex.RUnlock()

	return len(umm.mutexes)
}

// Private helper methods for URLMutexManager

func (umm *URLMutexManager) tryGetExistingMutex(url string) *sync.Mutex {
	umm.mapMutex.RLock()
	defer umm.mapMutex.RUnlock()

	return umm.mutexes[url]
}

func (umm *URLMutexManager) getOrCreateMutex(url string) *sync.Mutex {
	umm.mapMutex.Lock()
	defer umm.mapMutex.Unlock()

	// Double-check pattern: another goroutine might have created it
	if mutex, exists := umm.mutexes[url]; exists {
		return mutex
	}

	umm.mutexes[url] = &sync.Mutex{}
	return umm.mutexes[url]
}

func (umm *URLMutexManager) createActiveURLSet(activeURLs []string) map[string]struct{} {
	activeURLSet := make(map[string]struct{}, len(activeURLs))
	for _, url := range activeURLs {
		activeURLSet[url] = struct{}{}
	}
	return activeURLSet
}

func (umm *URLMutexManager) removeUnusedMutexes(activeURLSet map[string]struct{}) int {
	umm.mapMutex.Lock()
	defer umm.mapMutex.Unlock()

	removedCount := 0
	for url := range umm.mutexes {
		if _, isActive := activeURLSet[url]; !isActive {
			delete(umm.mutexes, url)
			removedCount++
		}
	}
	return removedCount
}

func (umm *URLMutexManager) logCleanupResults(removedCount int) {
	if removedCount > 0 {
		umm.logger.Debug().
			Int("removed_mutexes", removedCount).
			Int("remaining_mutexes", len(umm.mutexes)).
			Msg("Cleaned up unused URL check mutexes")
	}
}

// UpdateLogger updates the logger for this component
func (umm *URLMutexManager) UpdateLogger(newLogger zerolog.Logger) {
	umm.logger = newLogger.With().Str("component", "URLMutexManager").Logger()
}
