package differ

import "github.com/aleister1102/monsterinc/internal/models"

// URLStatusAnalyzer analyzes URL status changes
type URLStatusAnalyzer struct{}

// NewURLStatusAnalyzer creates a new URL status analyzer
func NewURLStatusAnalyzer() *URLStatusAnalyzer {
	return &URLStatusAnalyzer{}
}

// AnalyzeCurrentURLs analyzes current URLs against historical data
func (usa *URLStatusAnalyzer) AnalyzeCurrentURLs(currentProbes []*models.ProbeResult, urlMaps URLMaps) ([]models.DiffedURL, URLStatusCounts) {
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
