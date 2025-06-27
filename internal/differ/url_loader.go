package differ

import (
	"github.com/aleister1102/monsterinc/internal/datastore"
	httpx "github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// HistoricalDataLoader handles loading historical probe results
type HistoricalDataLoader struct {
	parquetReader *datastore.ParquetReader
}

// NewHistoricalDataLoader creates a new historical data loader
func NewHistoricalDataLoader(parquetReader *datastore.ParquetReader) *HistoricalDataLoader {
	return &HistoricalDataLoader{
		parquetReader: parquetReader,
	}
}

// LoadHistoricalProbes loads historical probe results for a root target, excluding current scan session
func (hdl *HistoricalDataLoader) LoadHistoricalProbes(rootTarget string, currentScanSessionID string) ([]httpx.ProbeResult, error) {
	if rootTarget == "" {
		return nil, NewValidationError("root_target", rootTarget, "root target cannot be empty")
	}

	allProbes, _, err := hdl.parquetReader.FindAllProbeResultsForTarget(rootTarget)
	if err != nil {
		return nil, WrapError(err, "failed to read historical data for target: "+rootTarget)
	}

	// Filter out probes from current scan session to avoid marking URLs as "existing"
	// when they are discovered multiple times within the same scan
	var historicalProbes []httpx.ProbeResult
	for _, probe := range allProbes {
		// Only include probes that are NOT from the current scan session
		// This ensures URLs discovered multiple times in current scan are marked as "new"
		if !hdl.isFromCurrentScanSession(probe, currentScanSessionID) {
			historicalProbes = append(historicalProbes, probe)
		}
	}

	return historicalProbes, nil
}

// isFromCurrentScanSession checks if a probe result is from the current scan session
func (hdl *HistoricalDataLoader) isFromCurrentScanSession(probe httpx.ProbeResult, currentScanSessionID string) bool {
	// For now, we can use a simple approach: if the probe doesn't have scan session info
	// or if we can't determine it reliably, we include it in historical data
	// This is a conservative approach that ensures we don't accidentally exclude valid historical data

	// TODO: Once ParquetReader is updated to read ScanSessionID from parquet files,
	// we can implement proper session-based filtering here
	// For now, return false to include all probes as historical
	return false
}
