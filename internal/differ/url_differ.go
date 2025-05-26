package differ

import (
	"fmt"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"time"

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
	ud.logger.Info().Str("root_target", rootTarget).Int("current_probe_count", len(currentScanProbes)).Msg("Starting URL diff")

	historicalProbes, err := ud.parquetReader.FindHistoricalDataForTarget(rootTarget)
	if err != nil {
		ud.logger.Error().Err(err).Str("root_target", rootTarget).Msg("Failed to get historical data")
		return nil, fmt.Errorf("failed to get historical data for %s: %w", rootTarget, err)
	}
	ud.logger.Info().Int("historical_probe_count", len(historicalProbes)).Str("root_target", rootTarget).Msg("Retrieved historical probes")

	result := &models.URLDiffResult{
		RootTargetURL: rootTarget,
		Results:       make([]models.DiffedURL, 0, len(currentScanProbes)+len(historicalProbes)), // Pre-allocate slice
		New:           0,
		Old:           0,
		Existing:      0,
	}

	currentProbesMap := make(map[string]*models.ProbeResult)
	for _, p := range currentScanProbes {
		if p == nil || p.InputURL == "" {
			ud.logger.Warn().Msg("Skipping current probe with nil or empty InputURL.")
			continue
		}
		currentProbesMap[p.InputURL] = p
	}

	historicalProbesMap := make(map[string]models.ProbeResult)
	for _, hp := range historicalProbes {
		if hp.InputURL == "" {
			ud.logger.Warn().Interface("historical_probe_details", hp).Msg("Skipping historical probe with empty InputURL.")
			continue
		}
		historicalProbesMap[hp.InputURL] = hp
	}

	// Identify New, Existing, or Changed URLs
	for keyURL, currentProbe := range currentProbesMap {
		currentProbe.URLStatus = string(models.StatusNew) // Default to New
		currentProbe.RootTargetURL = rootTarget           // Ensure RootTargetURL is set

		if historicalProbe, found := historicalProbesMap[keyURL]; found {
			currentProbe.URLStatus = string(models.StatusExisting)
			if !historicalProbe.OldestScanTimestamp.IsZero() {
				currentProbe.OldestScanTimestamp = historicalProbe.OldestScanTimestamp
			} else if !currentProbe.Timestamp.IsZero() { // currentProbe.Timestamp is from current scan
				currentProbe.OldestScanTimestamp = currentProbe.Timestamp
			} else {
				// Fallback if both are zero, though currentProbe.Timestamp should ideally always be set
				currentProbe.OldestScanTimestamp = time.Now()
				ud.logger.Warn().Str("url", keyURL).Msg("Current probe timestamp is zero, using time.Now() for OldestScanTimestamp.")
			}
			result.Existing++
			delete(historicalProbesMap, keyURL) // Remove from historical map to find "Old" URLs later
		} else {
			// URL is New
			if !currentProbe.Timestamp.IsZero() {
				currentProbe.OldestScanTimestamp = currentProbe.Timestamp // First time seeing it
			} else {
				currentProbe.OldestScanTimestamp = time.Now()
				ud.logger.Warn().Str("url", keyURL).Msg("Current probe timestamp is zero for NEW URL, using time.Now() for OldestScanTimestamp.")
			}
			result.New++
		}
		result.Results = append(result.Results, models.DiffedURL{ProbeResult: *currentProbe})
	}

	// Identify Old URLs (those remaining in historicalProbesMap)
	for _, oldProbe := range historicalProbesMap {
		oldProbe.URLStatus = string(models.StatusOld)
		result.Results = append(result.Results, models.DiffedURL{ProbeResult: oldProbe})
		result.Old++
	}

	ud.logger.Info().Str("root_target", rootTarget).Int("new", result.New).Int("existing", result.Existing).Int("old", result.Old).Int("total_diff_entries", len(result.Results)).Msg("Diff complete")

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
