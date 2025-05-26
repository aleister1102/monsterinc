package differ

import (
	"fmt"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"

	"github.com/rs/zerolog"
	// For default logger if needed
	// "time" // Removed: No longer needed
	// "strings" // Add if normalization is re-introduced
	// Import "net/url" and "strings" if a normalization function like normalizeURLKey is introduced.
	// For now, sticking to the original keying logic.
)

// UrlDiffer is responsible for comparing URL lists from current and previous scans.
type UrlDiffer struct {
	parquetReader *datastore.ParquetReader
	logger        zerolog.Logger
}

// NewUrlDiffer creates a new UrlDiffer instance.
func NewUrlDiffer(pr *datastore.ParquetReader, logger zerolog.Logger) *UrlDiffer {
	return &UrlDiffer{
		parquetReader: pr,
		logger:        logger.With().Str("module", "UrlDiffer").Logger(),
	}
}

// Compare performs the diffing logic.
// It updates the URLStatus field within the passed-in currentScanProbes items directly
// for 'new', 'existing', and 'changed' statuses.
// It uses historicalProbes to identify 'old' URLs and constructs DiffedURL entries for them.
func (ud *UrlDiffer) Compare(currentScanProbes []*models.ProbeResult, rootTarget string) (*models.URLDiffResult, error) {
	ud.logger.Info().Str("root_target", rootTarget).Int("current_probes_count", len(currentScanProbes)).Msg("Starting URL comparison")
	diffResult := &models.URLDiffResult{
		RootTargetURL: rootTarget,
		Results:       make([]models.DiffedURL, 0),
	}

	// Read historical data from the single Parquet file for this rootTarget
	// The second return value (modTime) is ignored for now.
	historicalProbes, _, err := ud.parquetReader.FindAllProbeResultsForTarget(rootTarget)
	if err != nil {
		ud.logger.Error().Err(err).Str("root_target", rootTarget).Msg("Failed to read historical Parquet data for diffing")
		diffResult.Error = fmt.Sprintf("Failed to read historical data: %v", err)
		return diffResult, err // Return error as this is crucial for diffing
	}

	ud.logger.Debug().Int("historical_probes_count", len(historicalProbes)).Msg("Historical probes loaded for diffing")

	// Create a map of historical URLs for quick lookup
	historicalURLMap := make(map[string]models.ProbeResult)
	for _, p := range historicalProbes {
		historicalURLMap[p.InputURL] = p // Assuming InputURL is the primary key
	}

	// Create a map of current URLs for quick lookup
	currentURLMap := make(map[string]models.ProbeResult)
	for _, p := range currentScanProbes {
		currentURLMap[p.InputURL] = *p // Dereference pointer
	}

	// Identify new and existing URLs
	for _, currentProbe := range currentScanProbes {
		_, existsInHistory := historicalURLMap[currentProbe.InputURL]
		if existsInHistory {
			diffResult.Existing++
			currentProbe.URLStatus = string(models.StatusExisting) // Mark as existing
			// TODO: Implement content change detection if needed
			// For now, just mark as existing. Actual content diffing is more complex.
		} else {
			diffResult.New++
			currentProbe.URLStatus = string(models.StatusNew) // Mark as new
		}
		diffResult.Results = append(diffResult.Results, models.DiffedURL{ProbeResult: *currentProbe})
	}

	// Identify old URLs (in history but not in current scan)
	for historicalURL, historicalProbe := range historicalURLMap {
		_, existsInCurrent := currentURLMap[historicalURL]
		if !existsInCurrent {
			diffResult.Old++
			historicalProbe.URLStatus = string(models.StatusOld) // Mark as old
			// Preserve historical probe data for "old" URLs
			diffResult.Results = append(diffResult.Results, models.DiffedURL{ProbeResult: historicalProbe})
		}
	}

	ud.logger.Info().
		Str("root_target", rootTarget).
		Int("new_urls", diffResult.New).
		Int("old_urls", diffResult.Old).
		Int("existing_urls", diffResult.Existing).
		Msg("URL comparison completed")

	return diffResult, nil
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
