package differ

import (
	"fmt"
	"log"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"os"
	// "time" // Removed: No longer needed
	// "strings" // Add if normalization is re-introduced
	// Import "net/url" and "strings" if a normalization function like normalizeURLKey is introduced.
	// For now, sticking to the original keying logic.
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

// Compare performs the diffing logic.
// It updates the URLStatus field within the passed-in currentScanProbes items directly
// for 'new', 'existing', and 'changed' statuses.
// It uses historicalProbes to identify 'old' URLs and constructs DiffedURL entries for them.
func (ud *UrlDiffer) Compare(currentScanProbes []*models.ProbeResult, rootTarget string) (*models.URLDiffResult, error) {
	ud.logger.Printf("[INFO] Differ: Starting URL diff for root target: %s. Current probes: %d", rootTarget, len(currentScanProbes))

	historicalProbes, err := ud.parquetReader.FindHistoricalDataForTarget(rootTarget)
	if err != nil {
		ud.logger.Printf("[ERROR] Differ: Failed to get historical data for target %s: %v", rootTarget, err)
		return nil, fmt.Errorf("failed to get historical data for %s: %w", rootTarget, err)
	}
	ud.logger.Printf("[INFO] Differ: Retrieved %d historical probes for target %s", len(historicalProbes), rootTarget)

	result := &models.URLDiffResult{
		RootTargetURL: rootTarget,                  // Corrected field name
		Results:       make([]models.DiffedURL, 0), // Corrected: slice, not map
		New:           0,
		Old:           0,
		Existing:      0,
	}

	// Use InputURL as key for comparison. Normalization can be added later.
	currentProbesMap := make(map[string]*models.ProbeResult)
	for _, p := range currentScanProbes {
		if p.InputURL == "" {
			ud.logger.Printf("[WARN] Differ: Skipping current probe with empty InputURL.")
			continue
		}
		currentProbesMap[p.InputURL] = p
	}

	historicalProbesMap := make(map[string]models.ProbeResult)
	for _, hp := range historicalProbes {
		if hp.InputURL == "" {
			ud.logger.Printf("[WARN] Differ: Skipping historical probe with empty InputURL: %+v", hp)
			continue
		}
		historicalProbesMap[hp.InputURL] = hp
	}

	// currentScanTime := time.Now() // Removed: Not used, currentProbe.Timestamp is used

	// Identify New, Existing, or Changed URLs
	for keyURL, currentProbe := range currentProbesMap {
		currentProbe.URLStatus = string(models.StatusNew) // Default to New
		currentProbe.RootTargetURL = rootTarget           // Ensure RootTargetURL is set

		if historicalProbe, found := historicalProbesMap[keyURL]; found {
			currentProbe.URLStatus = string(models.StatusExisting)
			// Preserve FirstSeenTimestamp from historical data if it exists and is valid
			if !historicalProbe.OldestScanTimestamp.IsZero() {
				currentProbe.OldestScanTimestamp = historicalProbe.OldestScanTimestamp
			} else {
				currentProbe.OldestScanTimestamp = currentProbe.Timestamp // currentProbe.Timestamp is from current scan
			}
			// LastSeen for existing items is the current scan's timestamp
			// currentProbe.Timestamp should already be the current scan time from the prober

			result.Existing++
		} else {
			// URL is New
			currentProbe.OldestScanTimestamp = currentProbe.Timestamp // First time seeing it
			// currentProbe.Timestamp is already current scan time
			result.New++
		}
		result.Results = append(result.Results, models.DiffedURL{ProbeResult: *currentProbe})
		delete(historicalProbesMap, keyURL) // Remove from historical map to find "Old" URLs later
	}

	// Identify Old URLs (those remaining in historicalProbesMap)
	for _, oldProbe := range historicalProbesMap {
		oldProbe.URLStatus = string(models.StatusOld)
		// RootTargetURL should already be set from when it was stored
		// Timestamps (OldestScanTimestamp, Timestamp) for old probes remain as they were in historical data
		result.Results = append(result.Results, models.DiffedURL{ProbeResult: oldProbe})
		result.Old++
	}

	ud.logger.Printf("[INFO] Differ: Diff complete for %s. New: %d, Existing: %d, Old: %d. Total diff entries: %d",
		rootTarget, result.New, result.Existing, result.Old, len(result.Results))

	return result, nil
}

// Helper to count statuses in DiffResult - NO LONGER NEEDED here if counts are in URLDiffResult struct
/*
func countStatus(result *models.URLDiffResult, status models.URLStatus) int { ... }
*/

// Helper to count existing statuses in currentScanResults - NO LONGER NEEDED here
/*
func countExisting(currentScanResults []*models.ProbeResult) int { ... } // If it were to be used, signature would change
*/

// NOTE: If a URL normalization strategy (like in the other url_differ) is required,
// the normalizeURLKey function should be added here and used consistently for both
// current and historical URL keys.
