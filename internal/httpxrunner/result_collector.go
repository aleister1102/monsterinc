package telescope

import (
	"sync"

	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

// ResultCollector is a thread-safe collector for probe results.
type ResultCollector struct {
	logger  zerolog.Logger
	results []ProbeResult
	mutex   sync.Mutex
	mapper  *ProbeResultMapper
	rootURL string
}

// NewResultCollector creates a new result collector.
func NewResultCollector(logger zerolog.Logger, mapper *ProbeResultMapper, rootURL string) *ResultCollector {
	return &ResultCollector{
		logger:  logger.With().Str("component", "ResultCollector").Logger(),
		results: make([]ProbeResult, 0),
		mapper:  mapper,
		rootURL: rootURL,
	}
}

// Collect is the callback function passed to the httpx engine.
// It receives a result, maps it, and adds it to the internal slice.
func (rc *ResultCollector) Collect(result runner.Result) {
	rc.logger.Info().Str("url", result.Input).Msg("Collected result")
	if rc.mapper == nil {
		rc.logger.Error().Msg("ResultCollector's mapper is nil. Cannot process result.")
		return
	}
	mappedResult := rc.mapper.MapResult(result, rc.rootURL)
	if mappedResult != nil {
		rc.mutex.Lock()
		rc.results = append(rc.results, *mappedResult)
		rc.mutex.Unlock()
	}
}

// GetResults returns all collected results.
func (rc *ResultCollector) GetResults() []ProbeResult {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	return rc.results
}
