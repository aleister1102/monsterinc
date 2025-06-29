package datastore

import (
	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/common/filemanager"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ParquetWriterBuilder provides a fluent interface for creating ParquetWriter
type ParquetWriterBuilder struct {
	config       *config.StorageConfig
	logger       zerolog.Logger
	writerConfig ParquetWriterConfig
}

// NewParquetWriterBuilder creates a new ParquetWriterBuilder
func NewParquetWriterBuilder(logger zerolog.Logger) *ParquetWriterBuilder {
	return &ParquetWriterBuilder{
		logger:       logger.With().Str("component", "ParquetWriter").Logger(),
		writerConfig: DefaultParquetWriterConfig(),
	}
}

// WithStorageConfig sets the storage configuration
func (b *ParquetWriterBuilder) WithStorageConfig(cfg *config.StorageConfig) *ParquetWriterBuilder {
	b.config = cfg
	return b
}

// WithWriterConfig sets the writer configuration
func (b *ParquetWriterBuilder) WithWriterConfig(cfg ParquetWriterConfig) *ParquetWriterBuilder {
	b.writerConfig = cfg
	return b
}

// Build creates a new ParquetWriter instance
func (b *ParquetWriterBuilder) Build() (*ParquetWriter, error) {
	if b.config == nil {
		return nil, errorwrapper.NewValidationError("config", b.config, "storage config cannot be nil")
	}

	if b.config.ParquetBasePath == "" {
		b.logger.Warn().Msg("ParquetBasePath is empty in config")
	}

	fileManager := filemanager.NewFileManager(b.logger)

	return &ParquetWriter{
		config:       b.config,
		logger:       b.logger,
		fileManager:  fileManager,
		writerConfig: b.writerConfig,
	}, nil
}
