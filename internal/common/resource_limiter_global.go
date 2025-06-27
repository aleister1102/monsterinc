package common

import (
	"sync"

	limiter "github.com/aleister1102/monsterinc/internal/rslimiter"
)

var (
	globalResourceLimiter *limiter.ResourceLimiter
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
