package differ

import (
	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
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
func (hdl *HistoricalDataLoader) LoadHistoricalProbes(rootTarget string) ([]httpxrunner.ProbeResult, error) {
	if rootTarget == "" {
		return nil, errors.NewValidationError("root_target", rootTarget, "root target cannot be empty")
	}

	allProbes, _, err := hdl.parquetReader.FindAllProbeResultsForTarget(rootTarget)
	if err != nil {
		return nil, errors.WrapError(err, "failed to read historical data for target: "+rootTarget)
	}

	return allProbes, nil
}
