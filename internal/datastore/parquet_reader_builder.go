package datastore

import (
	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ParquetReaderBuilder provides a fluent interface for creating ParquetReader
type ParquetReaderBuilder struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	config        ParquetReaderConfig
}

// NewParquetReaderBuilder creates a new ParquetReaderBuilder
func NewParquetReaderBuilder(logger zerolog.Logger) *ParquetReaderBuilder {
	return &ParquetReaderBuilder{
		logger: logger.With().Str("component", "ParquetReader").Logger(),
		config: DefaultParquetReaderConfig(),
	}
}

// WithStorageConfig sets the storage configuration
func (b *ParquetReaderBuilder) WithStorageConfig(cfg *config.StorageConfig) *ParquetReaderBuilder {
	b.storageConfig = cfg
	return b
}

// WithReaderConfig sets the reader configuration
func (b *ParquetReaderBuilder) WithReaderConfig(cfg ParquetReaderConfig) *ParquetReaderBuilder {
	b.config = cfg
	return b
}

// Build creates a new ParquetReader instance
func (b *ParquetReaderBuilder) Build() (*ParquetReader, error) {
	if b.storageConfig == nil || b.storageConfig.ParquetBasePath == "" {
		b.logger.Warn().Msg("StorageConfig or ParquetBasePath is not properly configured")
	}

	fileManager := common.NewFileManager(b.logger)

	return &ParquetReader{
		storageConfig: b.storageConfig,
		logger:        b.logger,
		fileManager:   fileManager,
		config:        b.config,
	}, nil
}
