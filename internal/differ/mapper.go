package differ

import (
	"strings"

	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// URLMapper creates lookup maps for URL comparison
type URLMapper struct {
	config URLDifferConfig
}

// NewURLMapper creates a new URL mapper
func NewURLMapper(config URLDifferConfig) *URLMapper {
	return &URLMapper{
		config: config,
	}
}

// CreateMaps creates lookup maps from probe result slices
func (um *URLMapper) CreateMaps(historicalProbes []httpxrunner.ProbeResult, currentProbes []*httpxrunner.ProbeResult) URLMaps {
	historicalURLMap := make(map[string]httpxrunner.ProbeResult)
	for _, p := range historicalProbes {
		key := um.getURLKey(p.InputURL)
		historicalURLMap[key] = p
	}

	currentURLMap := make(map[string]httpxrunner.ProbeResult)
	for _, p := range currentProbes {
		key := um.getURLKey(p.InputURL)
		currentURLMap[key] = *p
	}

	return URLMaps{
		HistoricalURLMap: historicalURLMap,
		CurrentURLMap:    currentURLMap,
	}
}

// getURLKey returns the key to use for URL comparison
func (um *URLMapper) getURLKey(url string) string {
	key := url

	// Apply URL normalization if enabled
	if um.config.EnableURLNormalization {
		if normalized, err := urlhandler.NormalizeURL(url); err == nil {
			key = normalized
		}
	}

	// Apply case sensitivity
	if !um.config.CaseSensitive {
		key = strings.ToLower(key)
	}

	return key
}
