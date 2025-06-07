package crawler

import (
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// URLPatternDetector detects similar URL patterns and implements auto-calibrate logic
type URLPatternDetector struct {
	config        config.AutoCalibrateConfig
	logger        zerolog.Logger
	patternCounts map[string]int
	patternMutex  sync.RWMutex
	seenURLs      map[string]bool
	urlMutex      sync.RWMutex
}

// NewURLPatternDetector creates a new URL pattern detector
func NewURLPatternDetector(config config.AutoCalibrateConfig, logger zerolog.Logger) *URLPatternDetector {
	return &URLPatternDetector{
		config:        config,
		logger:        logger.With().Str("component", "URLPatternDetector").Logger(),
		patternCounts: make(map[string]int),
		seenURLs:      make(map[string]bool),
	}
}

// ShouldSkipURL determines if a URL should be skipped based on pattern similarity
func (upd *URLPatternDetector) ShouldSkipURL(rawURL string) bool {
	if !upd.config.Enabled {
		return false
	}

	// Check if URL was already seen
	upd.urlMutex.RLock()
	if upd.seenURLs[rawURL] {
		upd.urlMutex.RUnlock()
		return true
	}
	upd.urlMutex.RUnlock()

	// Generate pattern for the URL
	pattern, err := upd.generateURLPattern(rawURL)
	if err != nil {
		upd.logger.Debug().Err(err).Str("url", rawURL).Msg("Failed to generate URL pattern")
		return false
	}

	// Check pattern count
	upd.patternMutex.RLock()
	currentCount := upd.patternCounts[pattern]
	upd.patternMutex.RUnlock()

	// If we've exceeded the limit for this pattern, skip
	if currentCount >= upd.config.MaxSimilarURLs {
		if upd.config.EnableSkipLogging {
			upd.logger.Info().
				Str("url", rawURL).
				Str("pattern", pattern).
				Int("current_count", currentCount).
				Int("max_similar", upd.config.MaxSimilarURLs).
				Msg("Skipping URL due to similar pattern (auto-calibrate)")
		}

		// Mark URL as seen
		upd.urlMutex.Lock()
		upd.seenURLs[rawURL] = true
		upd.urlMutex.Unlock()

		return true
	}

	// Record this URL and increment pattern count
	upd.patternMutex.Lock()
	upd.patternCounts[pattern]++
	upd.patternMutex.Unlock()

	upd.urlMutex.Lock()
	upd.seenURLs[rawURL] = true
	upd.urlMutex.Unlock()

	return false
}

// generateURLPattern generates a pattern string for a URL by removing ignored parameters
func (upd *URLPatternDetector) generateURLPattern(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Create pattern from scheme, host, port, and path
	pattern := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	// Process query parameters
	if parsedURL.RawQuery != "" {
		filteredParams := upd.filterQueryParameters(parsedURL.Query())
		if len(filteredParams) > 0 {
			pattern += "?" + filteredParams
		}
	}

	// Add fragment if present (but not variable parts)
	if parsedURL.Fragment != "" && !upd.isVariableFragment(parsedURL.Fragment) {
		pattern += "#" + parsedURL.Fragment
	}

	return pattern, nil
}

// filterQueryParameters filters out ignored parameters and returns normalized query string
func (upd *URLPatternDetector) filterQueryParameters(params url.Values) string {
	var filteredPairs []string

	for key, values := range params {
		if !upd.isIgnoredParameter(key) {
			for range values {
				// For pattern matching, we don't care about the actual value
				// Just whether the parameter exists
				filteredPairs = append(filteredPairs, key+"=*")
			}
		}
	}

	// Sort for consistent pattern generation
	sort.Strings(filteredPairs)
	return strings.Join(filteredPairs, "&")
}

// isIgnoredParameter checks if a parameter should be ignored for pattern matching
func (upd *URLPatternDetector) isIgnoredParameter(paramName string) bool {
	for _, ignored := range upd.config.IgnoreParameters {
		if strings.EqualFold(paramName, ignored) {
			return true
		}
	}
	return false
}

// isVariableFragment checks if a fragment appears to be variable (like #a, #123, etc.)
func (upd *URLPatternDetector) isVariableFragment(fragment string) bool {
	// Simple heuristic: if fragment is short and alphanumeric, it's likely variable
	if len(fragment) <= 3 {
		return true
	}

	// Check if it's just a single letter or number
	if len(fragment) == 1 {
		return true
	}

	return false
}

// GetPatternStats returns statistics about detected patterns
func (upd *URLPatternDetector) GetPatternStats() map[string]int {
	upd.patternMutex.RLock()
	defer upd.patternMutex.RUnlock()

	// Return a copy to avoid race conditions
	stats := make(map[string]int)
	for pattern, count := range upd.patternCounts {
		stats[pattern] = count
	}
	return stats
}

// Reset clears all pattern tracking data
func (upd *URLPatternDetector) Reset() {
	upd.patternMutex.Lock()
	upd.patternCounts = make(map[string]int)
	upd.patternMutex.Unlock()

	upd.urlMutex.Lock()
	upd.seenURLs = make(map[string]bool)
	upd.urlMutex.Unlock()

	upd.logger.Debug().Msg("URL pattern detector reset")
}
