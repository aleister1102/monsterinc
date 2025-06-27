package differ

import (
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	httpx "github.com/aleister1102/go-telescope"
)

// URLMapper creates lookup maps for URL comparison
type URLMapper struct {
	config URLComparerConfig
}

// NewURLMapper creates a new URL mapper
func NewURLMapper(config URLComparerConfig) *URLMapper {
	return &URLMapper{
		config: config,
	}
}

// CreateMaps creates lookup maps from probe result slices
func (um *URLMapper) CreateMaps(historicalProbes []httpx.ProbeResult, currentProbes []*httpx.ProbeResult) URLMaps {
	historicalURLMap := make(map[string]httpx.ProbeResult)
	for _, p := range historicalProbes {
		key := um.getURLKey(p.InputURL)
		historicalURLMap[key] = p
	}

	currentURLMap := make(map[string]httpx.ProbeResult)
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

// URLStatusAnalyzer analyzes URL status changes
type URLStatusAnalyzer struct{}

// NewURLStatusAnalyzer creates a new URL status analyzer
func NewURLStatusAnalyzer() *URLStatusAnalyzer {
	return &URLStatusAnalyzer{}
}

// AnalyzeCurrentURLs analyzes current URLs against historical data
func (usa *URLStatusAnalyzer) AnalyzeCurrentURLs(currentProbes []*httpx.ProbeResult, urlMaps URLMaps) ([]models.DiffedURL, URLStatusCounts) {
	var results []models.DiffedURL
	counts := URLStatusCounts{}

	for _, currentProbe := range currentProbes {
		key := currentProbe.InputURL // Using InputURL directly for now
		_, existsInHistory := urlMaps.HistoricalURLMap[key]

		if existsInHistory {
			counts.Existing++
			currentProbe.URLStatus = string(models.StatusExisting)
		} else {
			counts.New++
			currentProbe.URLStatus = string(models.StatusNew)
		}

		results = append(results, models.DiffedURL{ProbeResult: *currentProbe})
	}

	return results, counts
}

// AnalyzeOldURLs analyzes historical URLs to find old ones
func (usa *URLStatusAnalyzer) AnalyzeOldURLs(urlMaps URLMaps) ([]models.DiffedURL, int) {
	var oldResults []models.DiffedURL
	oldCount := 0

	for historicalURL, historicalProbe := range urlMaps.HistoricalURLMap {
		_, existsInCurrent := urlMaps.CurrentURLMap[historicalURL]
		if !existsInCurrent {
			oldCount++
			historicalProbe.URLStatus = string(models.StatusOld)
			oldResults = append(oldResults, models.DiffedURL{ProbeResult: historicalProbe})
		}
	}

	return oldResults, oldCount
}
