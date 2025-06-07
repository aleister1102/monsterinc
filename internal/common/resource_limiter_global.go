package common

import "sync"

var (
	globalResourceLimiter *ResourceLimiter
	globalLimiterMutex    sync.RWMutex
)

// StopGlobalResourceLimiter stops the global resource limiter
func StopGlobalResourceLimiter() {
	globalLimiterMutex.RLock()
	limiter := globalResourceLimiter
	globalLimiterMutex.RUnlock()

	if limiter != nil {
		limiter.Stop()
	}
}
