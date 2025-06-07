package scanner

import (
	"sync"

	"github.com/rs/zerolog"
)

// URLStatsTracker handles statistics tracking for URL preprocessing
type URLStatsTracker struct {
	stats      URLPreprocessorStats
	statsMutex sync.RWMutex
	seenURLs   map[string]bool
	urlMutex   sync.RWMutex
	logger     zerolog.Logger
}

// URLPreprocessorStats tracks preprocessing statistics
type URLPreprocessorStats struct {
	TotalProcessed   int `json:"total_processed"`
	Normalized       int `json:"normalized"`
	SkippedByPattern int `json:"skipped_by_pattern"`
	SkippedDuplicate int `json:"skipped_duplicate"`
	FinalCount       int `json:"final_count"`
}

// NewURLStatsTracker creates a new stats tracker
func NewURLStatsTracker(logger zerolog.Logger) *URLStatsTracker {
	return &URLStatsTracker{
		stats:    URLPreprocessorStats{},
		seenURLs: make(map[string]bool),
		logger:   logger,
	}
}

// IsURLSeen checks if a URL has been seen before
func (ust *URLStatsTracker) IsURLSeen(url string) bool {
	ust.urlMutex.RLock()
	defer ust.urlMutex.RUnlock()
	return ust.seenURLs[url]
}

// MarkURLSeen marks a URL as seen
func (ust *URLStatsTracker) MarkURLSeen(url string) {
	ust.urlMutex.Lock()
	defer ust.urlMutex.Unlock()
	ust.seenURLs[url] = true
}

// ResetStats resets all statistics
func (ust *URLStatsTracker) ResetStats() {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()

	ust.stats = URLPreprocessorStats{}

	// Also reset seen URLs tracking
	ust.urlMutex.Lock()
	defer ust.urlMutex.Unlock()
	ust.seenURLs = make(map[string]bool)
}

// IncrementProcessed increments the total processed count
func (ust *URLStatsTracker) IncrementProcessed() {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()
	ust.stats.TotalProcessed++
}

// IncrementNormalized increments the normalized count
func (ust *URLStatsTracker) IncrementNormalized() {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()
	ust.stats.Normalized++
}

// IncrementSkippedByPattern increments the skipped by pattern count
func (ust *URLStatsTracker) IncrementSkippedByPattern() {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()
	ust.stats.SkippedByPattern++
}

// IncrementSkippedDuplicate increments the skipped duplicate count
func (ust *URLStatsTracker) IncrementSkippedDuplicate() {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()
	ust.stats.SkippedDuplicate++
}

// SetFinalCount sets the final processed count
func (ust *URLStatsTracker) SetFinalCount(count int) {
	ust.statsMutex.Lock()
	defer ust.statsMutex.Unlock()
	ust.stats.FinalCount = count
}

// GetStats returns a copy of current statistics
func (ust *URLStatsTracker) GetStats() URLPreprocessorStats {
	ust.statsMutex.RLock()
	defer ust.statsMutex.RUnlock()
	return ust.stats
}

// LogProcessingResults logs the final processing results
func (ust *URLStatsTracker) LogProcessingResults() {
	stats := ust.GetStats()

	ust.logger.Info().
		Int("total_processed", stats.TotalProcessed).
		Int("normalized", stats.Normalized).
		Int("skipped_by_pattern", stats.SkippedByPattern).
		Int("skipped_duplicate", stats.SkippedDuplicate).
		Int("final_count", stats.FinalCount).
		Msg("URL preprocessing completed")
}
