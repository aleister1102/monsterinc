package differ

import (
	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/rs/zerolog"
)

// UrlDifferBuilder provides a fluent interface for creating UrlDiffer
type UrlDifferBuilder struct {
	parquetReader *datastore.ParquetReader
	logger        zerolog.Logger
	config        URLDifferConfig
}

// NewUrlDifferBuilder creates a new builder
func NewUrlDifferBuilder(logger zerolog.Logger) *UrlDifferBuilder {
	return &UrlDifferBuilder{
		logger: logger.With().Str("component", "UrlDiffer").Logger(),
		config: DefaultURLDifferConfig(),
	}
}

// WithParquetReader sets the parquet reader
func (b *UrlDifferBuilder) WithParquetReader(pr *datastore.ParquetReader) *UrlDifferBuilder {
	b.parquetReader = pr
	return b
}

// WithConfig sets the URL comparer configuration
func (b *UrlDifferBuilder) WithConfig(config URLDifferConfig) *UrlDifferBuilder {
	b.config = config
	return b
}

// Build creates a new UrlDiffer instance
func (b *UrlDifferBuilder) Build() (*UrlDiffer, error) {
	if b.parquetReader == nil {
		return nil, errors.NewValidationError("parquet_reader", b.parquetReader, "parquet reader cannot be nil")
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
