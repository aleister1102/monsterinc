package httpx

import (
	"sync"

	"github.com/rs/zerolog"
)

// ResultCollector handles collection of probe results
type ResultCollector struct {
	results []ProbeResult
	mutex   sync.RWMutex
	logger  zerolog.Logger
}

// NewResultCollector creates a new result collector
func NewResultCollector(logger zerolog.Logger) *ResultCollector {
	return &ResultCollector{
		results: make([]ProbeResult, 0),
		logger:  logger.With().Str("component", "ResultCollector").Logger(),
	}
}

// AddResult adds a result to the collection
func (rc *ResultCollector) AddResult(result *ProbeResult) {
	if result == nil {
		rc.logger.Warn().Msg("Attempted to add nil result")
		return
	}

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.results = append(rc.results, *result)
	rc.logger.Debug().
		Str("input_url", result.InputURL).
		Int("status_code", result.StatusCode).
		Msg("Result added to collection")
}

// GetResults returns all collected results
func (rc *ResultCollector) GetResults() []ProbeResult {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	// Return copy to prevent external modifications
	resultsCopy := make([]ProbeResult, len(rc.results))
	copy(resultsCopy, rc.results)

	return resultsCopy
}

// GetResultsCount returns the number of collected results
func (rc *ResultCollector) GetResultsCount() int {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	return len(rc.results)
}
