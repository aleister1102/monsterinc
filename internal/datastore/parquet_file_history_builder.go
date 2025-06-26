package datastore

import (
	"os"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ParquetFileHistoryStoreBuilder provides a fluent interface for creating ParquetFileHistoryStore
type ParquetFileHistoryStoreBuilder struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	config        ParquetFileHistoryStoreConfig
}

// NewParquetFileHistoryStoreBuilder creates a new builder
func NewParquetFileHistoryStoreBuilder(logger zerolog.Logger) *ParquetFileHistoryStoreBuilder {
	return &ParquetFileHistoryStoreBuilder{
		logger: logger.With().Str("component", "ParquetFileHistoryStore").Logger(),
		config: DefaultParquetFileHistoryStoreConfig(),
	}
}

// WithStorageConfig sets the storage configuration
func (b *ParquetFileHistoryStoreBuilder) WithStorageConfig(cfg *config.StorageConfig) *ParquetFileHistoryStoreBuilder {
	b.storageConfig = cfg
	return b
}

// WithConfig sets the store configuration
func (b *ParquetFileHistoryStoreBuilder) WithConfig(cfg ParquetFileHistoryStoreConfig) *ParquetFileHistoryStoreBuilder {
	b.config = cfg
	return b
}

// Build creates a new ParquetFileHistoryStore instance
func (b *ParquetFileHistoryStoreBuilder) Build() (*ParquetFileHistory, error) {
	if b.storageConfig == nil {
		return nil, NewValidationError("storage_config", b.storageConfig, "storage config cannot be nil")
	}

	basePath := b.storageConfig.ParquetBasePath
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, WrapError(err, "failed to ensure monitor history base directory: "+basePath)
	}

	mutexManager := NewURLMutexManager(b.config.EnableURLMutexes, b.logger)

	store := &ParquetFileHistory{
		storageConfig: b.storageConfig,
		logger:        b.logger,
		fileManager:   common.NewFileManager(b.logger),
		urlHashGen:    NewURLHashGenerator(16),
		config:        b.config,
		mutexManager:  mutexManager,
		urlMutexes:    make(map[string]*sync.Mutex),
	}

	b.logger.Info().Str("path", basePath).Msg("Monitor history base directory ensured")
	return store, nil
}
