package differ

import "github.com/aleister1102/monsterinc/internal/httpxrunner"

// URLStatusAnalyzer analyzes URL status changes
type URLStatusAnalyzer struct {
	urlMapper *URLMapper
}

// NewURLStatusAnalyzer creates a new URL status analyzer
func NewURLStatusAnalyzer(urlMapper *URLMapper) *URLStatusAnalyzer {
	return &URLStatusAnalyzer{
		urlMapper: urlMapper,
	}
}

// AnalyzeCurrentURLs analyzes current URLs against historical data
func (usa *URLStatusAnalyzer) AnalyzeCurrentURLs(currentProbes []*httpxrunner.ProbeResult, urlMaps URLMaps) ([]DiffedURL, URLStatusCounts) {
	var results []DiffedURL
	counts := URLStatusCounts{}

	for _, currentProbe := range currentProbes {
		key := usa.urlMapper.GetURLKey(currentProbe.GetEffectiveURL()) // Using consistent key generation
		_, existsInHistory := urlMaps.HistoricalURLMap[key]

		if existsInHistory {
			counts.Existing++
			currentProbe.URLStatus = string(StatusExisting)
		} else {
			counts.New++
			currentProbe.URLStatus = string(StatusNew)
		}

		results = append(results, DiffedURL{ProbeResult: *currentProbe})
	}

	return results, counts
}

// AnalyzeOldURLs analyzes historical URLs to find old ones
func (usa *URLStatusAnalyzer) AnalyzeOldURLs(urlMaps URLMaps) ([]DiffedURL, int) {
	var oldResults []DiffedURL
	oldCount := 0

	for historicalURL, historicalProbe := range urlMaps.HistoricalURLMap {
		_, existsInCurrent := urlMaps.CurrentURLMap[historicalURL]
		if !existsInCurrent {
			oldCount++
			historicalProbe.URLStatus = string(StatusOld)
			oldResults = append(oldResults, DiffedURL{ProbeResult: historicalProbe})
		}
	}

	return oldResults, oldCount
}
