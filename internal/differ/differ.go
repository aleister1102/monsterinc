package differ

import (
	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/rs/zerolog"
)

// UrlDiffer is responsible for comparing URL lists from current and previous scans
type UrlDiffer struct {
	parquetReader  *datastore.ParquetReader
	logger         zerolog.Logger
	config         URLDifferConfig
	dataLoader     *HistoricalDataLoader
	urlMapper      *URLMapper
	statusAnalyzer *URLStatusAnalyzer
}

// NewUrlDiffer creates a new UrlDiffer instance using builder pattern
func NewUrlDiffer(pr *datastore.ParquetReader, logger zerolog.Logger) (*UrlDiffer, error) {
	return NewUrlDifferBuilder(logger).
		WithParquetReader(pr).
		Build()
}

// Differentiate performs the diffing logic
func (ud *UrlDiffer) Differentiate(currentScanProbes []*httpxrunner.ProbeResult, rootTarget string, scanSessionID string) (*URLDiffResult, error) {
	// Validate inputs
	if err := ud.validateInputs(currentScanProbes, rootTarget); err != nil {
		return nil, errors.WrapError(err, "failed to validate URL differ inputs")
	}

	resultBuilder := NewURLDiffResultBuilder(rootTarget)

	// Load historical data, excluding current scan session
	historicalProbes, err := ud.dataLoader.LoadHistoricalProbes(rootTarget)
	if err != nil {
		resultBuilder.WithError(err)
		return resultBuilder.Build(), err
	}

	// Create lookup maps
	urlMaps := ud.urlMapper.CreateMaps(historicalProbes, currentScanProbes)

	// Analyze current URLs
	currentResults, counts := ud.statusAnalyzer.AnalyzeCurrentURLs(currentScanProbes, urlMaps)
	resultBuilder.WithResults(currentResults, counts)

	// Analyze old URLs
	oldResults, oldCount := ud.statusAnalyzer.AnalyzeOldURLs(urlMaps)
	resultBuilder.AddResults(oldResults, oldCount)

	return resultBuilder.Build(), nil
}

// validateInputs validates the input parameters for URL comparison
func (ud *UrlDiffer) validateInputs(currentScanProbes []*httpxrunner.ProbeResult, rootTarget string) error {
	if rootTarget == "" {
		return errors.NewValidationError("root_target", rootTarget, "root target cannot be empty")
	}

	if currentScanProbes == nil {
		return errors.NewValidationError("current_scan_probes", currentScanProbes, "current scan probes cannot be nil")
	}

	return nil
}
