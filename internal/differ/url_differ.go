package differ

import (
	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// UrlDiffer is responsible for comparing URL lists from current and previous scans
type UrlDiffer struct {
	parquetReader  *datastore.ParquetReader
	logger         zerolog.Logger
	config         URLComparerConfig
	dataLoader     *HistoricalDataLoader
	urlMapper      *URLMapper
	statusAnalyzer *URLStatusAnalyzer
}

// UrlDifferBuilder provides a fluent interface for creating UrlDiffer
type UrlDifferBuilder struct {
	parquetReader *datastore.ParquetReader
	logger        zerolog.Logger
	config        URLComparerConfig
}

// NewUrlDifferBuilder creates a new builder
func NewUrlDifferBuilder(logger zerolog.Logger) *UrlDifferBuilder {
	return &UrlDifferBuilder{
		logger: logger.With().Str("component", "UrlDiffer").Logger(),
		config: DefaultURLComparerConfig(),
	}
}

// WithParquetReader sets the parquet reader
func (b *UrlDifferBuilder) WithParquetReader(pr *datastore.ParquetReader) *UrlDifferBuilder {
	b.parquetReader = pr
	return b
}

// WithConfig sets the URL comparer configuration
func (b *UrlDifferBuilder) WithConfig(config URLComparerConfig) *UrlDifferBuilder {
	b.config = config
	return b
}

// Build creates a new UrlDiffer instance
func (b *UrlDifferBuilder) Build() (*UrlDiffer, error) {
	if b.parquetReader == nil {
		return nil, common.NewValidationError("parquet_reader", b.parquetReader, "parquet reader cannot be nil")
	}

	dataLoader := NewHistoricalDataLoader(b.parquetReader)
	urlMapper := NewURLMapper(b.config)
	statusAnalyzer := NewURLStatusAnalyzer()

	return &UrlDiffer{
		parquetReader:  b.parquetReader,
		logger:         b.logger,
		config:         b.config,
		dataLoader:     dataLoader,
		urlMapper:      urlMapper,
		statusAnalyzer: statusAnalyzer,
	}, nil
}

// NewUrlDiffer creates a new UrlDiffer instance using builder pattern
func NewUrlDiffer(pr *datastore.ParquetReader, logger zerolog.Logger) (*UrlDiffer, error) {
	return NewUrlDifferBuilder(logger).
		WithParquetReader(pr).
		Build()
}

// validateInputs validates the input parameters for URL comparison
func (ud *UrlDiffer) validateInputs(currentScanProbes []*models.ProbeResult, rootTarget string) error {
	if rootTarget == "" {
		return common.NewValidationError("root_target", rootTarget, "root target cannot be empty")
	}

	if currentScanProbes == nil {
		return common.NewValidationError("current_scan_probes", currentScanProbes, "current scan probes cannot be nil")
	}

	return nil
}

// Compare performs the diffing logic
func (ud *UrlDiffer) Compare(currentScanProbes []*models.ProbeResult, rootTarget string) (*models.URLDiffResult, error) {
	// Validate inputs
	if err := ud.validateInputs(currentScanProbes, rootTarget); err != nil {
		return nil, common.WrapError(err, "failed to validate URL differ inputs")
	}

	resultBuilder := NewURLDiffResultBuilder(rootTarget)

	// Load historical data
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
