package common

import (
	"sync"

	"github.com/monsterinc/limiter"
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
