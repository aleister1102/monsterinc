package differ

import (
	"log"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"os"
)

// UrlDiffer is responsible for comparing URL lists from current and previous scans.
type UrlDiffer struct {
	parquetReader *datastore.ParquetReader
	logger        *log.Logger
}

// NewUrlDiffer creates a new UrlDiffer instance.
func NewUrlDiffer(pr *datastore.ParquetReader, logger *log.Logger) *UrlDiffer {
	if logger == nil {
		logger = log.New(os.Stdout, "URL_DIFFER: ", log.LstdFlags|log.Lshortfile)
	}
	return &UrlDiffer{
		parquetReader: pr,
		logger:        logger,
	}
}

// Compare compares the current scan results with historical data to identify new and old URLs.
// It also updates the URLStatus field in the input currentScanResults slice.
func (ud *UrlDiffer) Compare(currentScanResults []models.ProbeResult, rootTargetURL string) (*models.URLDiffResult, error) {
	ud.logger.Printf("Starting URL comparison for target: %s. Received %d current scan results.", rootTargetURL, len(currentScanResults))

	historicalURLs, err := ud.parquetReader.FindHistoricalURLsForTarget(rootTargetURL)
	if err != nil {
		ud.logger.Printf("Error reading historical data for %s: %v. Treating as first scan.", rootTargetURL, err)
		historicalURLs = []string{} // Treat as empty historical data, equivalent to a first scan
	} else {
		ud.logger.Printf("Retrieved %d historical URLs for target: %s.", len(historicalURLs), rootTargetURL)
		if len(historicalURLs) == 0 {
			ud.logger.Printf("No historical URLs found for %s, treating as a first scan scenario.", rootTargetURL)
		}
	}

	currentURLsSet := make(map[string]models.ProbeResult)
	for i := range currentScanResults { // Iterate by index to modify original slice elements
		// Use FinalURL if available, otherwise InputURL for the set key
		keyURL := currentScanResults[i].FinalURL
		if keyURL == "" {
			keyURL = currentScanResults[i].InputURL
		}
		currentURLsSet[keyURL] = currentScanResults[i]
	}
	ud.logger.Printf("Built set of %d unique current URLs for %s.", len(currentURLsSet), rootTargetURL)

	historicalURLSet := make(map[string]bool)
	for _, hURL := range historicalURLs {
		historicalURLSet[hURL] = true
	}
	ud.logger.Printf("Built set of %d unique historical URLs for %s.", len(historicalURLSet), rootTargetURL)

	diffResult := &models.URLDiffResult{
		RootTargetURL: rootTargetURL,
		Results:       []models.DiffedURL{},
	}

	urlStatuses := make(map[string]models.URLStatus)

	// Find New URLs: in current, not in historical
	for urlStr := range currentURLsSet {
		if !historicalURLSet[urlStr] {
			diffResult.Results = append(diffResult.Results, models.DiffedURL{
				NormalizedURL: urlStr,
				Status:        models.StatusNew,
			})
			urlStatuses[urlStr] = models.StatusNew
		} else {
			// If in current and in historical, it's existing
			urlStatuses[urlStr] = models.StatusExisting
		}
	}

	// Find Old URLs: in historical, not in current
	for histURL := range historicalURLSet {
		if _, existsInCurrent := currentURLsSet[histURL]; !existsInCurrent {
			diffResult.Results = append(diffResult.Results, models.DiffedURL{
				NormalizedURL: histURL,
				Status:        models.StatusOld,
				LastSeenData:  models.ProbeResult{InputURL: histURL, Error: "Present in historical data, not in current scan"},
			})
			// urlStatuses[histURL] = models.StatusOld // No need to add old to this map, as it's for currentScanResults
		}
	}

	// Update URLStatus in the original currentScanResults slice
	for i := range currentScanResults {
		keyURL := currentScanResults[i].FinalURL
		if keyURL == "" {
			keyURL = currentScanResults[i].InputURL
		}
		if status, ok := urlStatuses[keyURL]; ok {
			currentScanResults[i].URLStatus = string(status)
		} else {
			// This case should ideally not be hit if logic is correct,
			// but as a fallback, or if a URL somehow didn't get processed.
			ud.logger.Printf("Warning: URL %s from current scan results not found in status map. Defaulting status.", keyURL)
			currentScanResults[i].URLStatus = string(models.StatusExisting) // Or some other default/unknown status
		}
	}

	ud.logger.Printf("URL comparison for %s complete. New: %d, Old: %d, Existing: %d (in current scan).",
		rootTargetURL,
		countStatus(diffResult, models.StatusNew),
		countStatus(diffResult, models.StatusOld),
		countExisting(currentScanResults))

	return diffResult, nil
}

// Helper to count statuses in DiffResult
func countStatus(result *models.URLDiffResult, status models.URLStatus) int {
	if result == nil {
		return 0
	}
	count := 0
	for _, r := range result.Results {
		if r.Status == status {
			count++
		}
	}
	return count
}

// Helper to count existing statuses in currentScanResults
func countExisting(currentScanResults []models.ProbeResult) int {
	count := 0
	for _, pr := range currentScanResults {
		if pr.URLStatus == string(models.StatusExisting) {
			count++
		}
	}
	return count
}
