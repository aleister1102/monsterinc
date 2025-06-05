package differ

import (
	"strings"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// URLComparerConfig holds configuration for URL comparison
type URLComparerConfig struct {
	EnableURLNormalization bool
	CaseSensitive          bool
}

// DefaultURLComparerConfig returns default configuration
func DefaultURLComparerConfig() URLComparerConfig {
	return URLComparerConfig{
		EnableURLNormalization: false,
		CaseSensitive:          true,
	}
}

// HistoricalDataLoader handles loading historical probe results
type HistoricalDataLoader struct {
	parquetReader *datastore.ParquetReader
	logger        zerolog.Logger
}

// NewHistoricalDataLoader creates a new historical data loader
func NewHistoricalDataLoader(parquetReader *datastore.ParquetReader, logger zerolog.Logger) *HistoricalDataLoader {
	return &HistoricalDataLoader{
		parquetReader: parquetReader,
		logger:        logger.With().Str("component", "HistoricalDataLoader").Logger(),
	}
}

// LoadHistoricalProbes loads historical probe results for a root target
func (hdl *HistoricalDataLoader) LoadHistoricalProbes(rootTarget string) ([]models.ProbeResult, error) {
	if rootTarget == "" {
		return nil, common.NewValidationError("root_target", rootTarget, "root target cannot be empty")
	}

	historicalProbes, _, err := hdl.parquetReader.FindAllProbeResultsForTarget(rootTarget)
	if err != nil {
		hdl.logger.Error().Err(err).Str("root_target", rootTarget).Msg("Failed to read historical Parquet data for diffing")
		return nil, common.WrapError(err, "failed to read historical data for target: "+rootTarget)
	}

	hdl.logger.Debug().
		Str("root_target", rootTarget).
		Int("historical_probes_count", len(historicalProbes)).
		Msg("Historical probes loaded successfully")

	return historicalProbes, nil
}

// URLMapper creates lookup maps for URL comparison
type URLMapper struct {
	logger zerolog.Logger
	config URLComparerConfig
}

// NewURLMapper creates a new URL mapper
func NewURLMapper(config URLComparerConfig, logger zerolog.Logger) *URLMapper {
	return &URLMapper{
		logger: logger.With().Str("component", "URLMapper").Logger(),
		config: config,
	}
}

// URLMaps holds the mapping data for URL comparison
type URLMaps struct {
	HistoricalURLMap map[string]models.ProbeResult
	CurrentURLMap    map[string]models.ProbeResult
}

// CreateMaps creates lookup maps from probe result slices
func (um *URLMapper) CreateMaps(historicalProbes []models.ProbeResult, currentProbes []*models.ProbeResult) URLMaps {
	historicalURLMap := make(map[string]models.ProbeResult)
	for _, p := range historicalProbes {
		key := um.getURLKey(p.InputURL)
		historicalURLMap[key] = p
	}

	currentURLMap := make(map[string]models.ProbeResult)
	for _, p := range currentProbes {
		key := um.getURLKey(p.InputURL)
		currentURLMap[key] = *p
	}

	um.logger.Debug().
		Int("historical_urls", len(historicalURLMap)).
		Int("current_urls", len(currentURLMap)).
		Msg("URL mapping completed")

	return URLMaps{
		HistoricalURLMap: historicalURLMap,
		CurrentURLMap:    currentURLMap,
	}
}

// getURLKey returns the key to use for URL comparison
func (um *URLMapper) getURLKey(url string) string {
	key := url

	// Apply URL normalization if enabled
	if um.config.EnableURLNormalization {
		if normalized, err := urlhandler.NormalizeURL(url); err == nil {
			key = normalized
		} else {
			um.logger.Warn().Err(err).Str("url", url).Msg("Failed to normalize URL for comparison")
		}
	}

	// Apply case sensitivity
	if !um.config.CaseSensitive {
		key = strings.ToLower(key)
	}

	return key
}

// URLStatusAnalyzer analyzes URL status changes
type URLStatusAnalyzer struct {
	logger zerolog.Logger
}

// NewURLStatusAnalyzer creates a new URL status analyzer
func NewURLStatusAnalyzer(logger zerolog.Logger) *URLStatusAnalyzer {
	return &URLStatusAnalyzer{
		logger: logger.With().Str("component", "URLStatusAnalyzer").Logger(),
	}
}

// URLStatusCounts holds the counts for different URL statuses
type URLStatusCounts struct {
	New      int
	Existing int
	Old      int
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
			usa.logger.Debug().Str("url", currentProbe.InputURL).Msg("URL marked as existing")
		} else {
			counts.New++
			currentProbe.URLStatus = string(models.StatusNew)
			usa.logger.Debug().Str("url", currentProbe.InputURL).Msg("URL marked as new")
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
			usa.logger.Debug().Str("url", historicalProbe.InputURL).Msg("URL marked as old")
		}
	}

	return oldResults, oldCount
}

// URLDiffResultBuilder builds URLDiffResult objects
type URLDiffResultBuilder struct {
	result models.URLDiffResult
	logger zerolog.Logger
}

// NewURLDiffResultBuilder creates a new result builder
func NewURLDiffResultBuilder(rootTarget string, logger zerolog.Logger) *URLDiffResultBuilder {
	return &URLDiffResultBuilder{
		result: models.URLDiffResult{
			RootTargetURL: rootTarget,
			Results:       make([]models.DiffedURL, 0),
		},
		logger: logger.With().Str("component", "URLDiffResultBuilder").Logger(),
	}
}

// WithError sets an error on the result
func (rb *URLDiffResultBuilder) WithError(err error) *URLDiffResultBuilder {
	rb.result.Error = err.Error()
	return rb
}

// WithResults sets the diff results and counts
func (rb *URLDiffResultBuilder) WithResults(results []models.DiffedURL, counts URLStatusCounts) *URLDiffResultBuilder {
	rb.result.Results = results
	rb.result.New = counts.New
	rb.result.Existing = counts.Existing
	rb.result.Old = counts.Old
	return rb
}

// AddResults adds additional results to the existing results
func (rb *URLDiffResultBuilder) AddResults(additionalResults []models.DiffedURL, additionalOldCount int) *URLDiffResultBuilder {
	rb.result.Results = append(rb.result.Results, additionalResults...)
	rb.result.Old += additionalOldCount
	return rb
}

// Build creates the final URLDiffResult
func (rb *URLDiffResultBuilder) Build() *models.URLDiffResult {
	return &rb.result
}

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

	dataLoader := NewHistoricalDataLoader(b.parquetReader, b.logger)
	urlMapper := NewURLMapper(b.config, b.logger)
	statusAnalyzer := NewURLStatusAnalyzer(b.logger)

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

	ud.logger.Info().
		Str("root_target", rootTarget).
		Int("current_probes_count", len(currentScanProbes)).
		Msg("Starting URL comparison")

	resultBuilder := NewURLDiffResultBuilder(rootTarget, ud.logger)

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

	result := resultBuilder.Build()

	ud.logger.Info().
		Str("root_target", rootTarget).
		Int("new_urls", result.New).
		Int("old_urls", result.Old).
		Int("existing_urls", result.Existing).
		Int("total_results", len(result.Results)).
		Msg("URL comparison completed successfully")

	return result, nil
}
