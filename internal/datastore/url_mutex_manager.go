package datastore

import (
	"sync"

	"github.com/rs/zerolog"
)

// URLMutexManager handles URL-specific mutex management
type URLMutexManager struct {
	mutexes map[string]*sync.Mutex
	mapLock sync.RWMutex
	enabled bool
	logger  zerolog.Logger
}

// NewURLMutexManager creates a new URL mutex manager
func NewURLMutexManager(enabled bool, logger zerolog.Logger) *URLMutexManager {
	return &URLMutexManager{
		mutexes: make(map[string]*sync.Mutex),
		enabled: enabled,
		logger:  logger.With().Str("component", "URLMutexManager").Logger(),
	}
}

// GetMutex returns a mutex for the specific URL to ensure thread-safety
func (umm *URLMutexManager) GetMutex(url string) *sync.Mutex {
	if !umm.enabled {
		// Return a dummy mutex that's safe to use but doesn't provide locking
		return &sync.Mutex{}
	}

	umm.mapLock.RLock()
	mutex, exists := umm.mutexes[url]
	umm.mapLock.RUnlock()

	if exists {
		return mutex
	}

	umm.mapLock.Lock()
	defer umm.mapLock.Unlock()

	// Double-check after acquiring write lock
	if mutex, exists := umm.mutexes[url]; exists {
		return mutex
	}

	mutex = &sync.Mutex{}
	umm.mutexes[url] = mutex
	return mutex
}

// CleanupUnusedMutexes removes mutexes for URLs that are no longer needed
func (umm *URLMutexManager) CleanupUnusedMutexes(activeURLs []string) {
	if !umm.enabled {
		return
	}

	activeSet := make(map[string]struct{})
	for _, url := range activeURLs {
		activeSet[url] = struct{}{}
	}

	umm.mapLock.Lock()
	defer umm.mapLock.Unlock()

	for url := range umm.mutexes {
		if _, active := activeSet[url]; !active {
			delete(umm.mutexes, url)
		}
	}

	umm.logger.Debug().
		Int("active_mutexes", len(umm.mutexes)).
		Msg("Cleaned up unused URL mutexes")
}
