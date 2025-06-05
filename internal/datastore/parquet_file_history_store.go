package datastore

import (
	"os"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"

	// Needed for GetLastKnownRecord sorting

	"github.com/rs/zerolog"
)

// Config and constants are now in file_history_config.go

// URLHashGenerator is now in url_hash_generator.go

// ParquetFileHistoryStore implements the models.FileHistoryStore interface using Parquet files.
// Each monitored URL will have its history stored in a separate Parquet file.
type ParquetFileHistoryStore struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	fileManager   *common.FileManager
	urlHashGen    *URLHashGenerator
	config        ParquetFileHistoryStoreConfig

	// Thread-safety components
	mutexManager *URLMutexManager
	urlMutexes   map[string]*sync.Mutex
	mutexMapLock sync.RWMutex
}

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
func (b *ParquetFileHistoryStoreBuilder) Build() (*ParquetFileHistoryStore, error) {
	if b.storageConfig == nil {
		return nil, common.NewValidationError("storage_config", b.storageConfig, "storage config cannot be nil")
	}

	basePath := b.storageConfig.ParquetBasePath
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, common.WrapError(err, "failed to ensure monitor history base directory: "+basePath)
	}

	mutexManager := NewURLMutexManager(b.config.EnableURLMutexes, b.logger)

	store := &ParquetFileHistoryStore{
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

// NewParquetFileHistoryStore creates a new ParquetFileHistoryStore using builder pattern
func NewParquetFileHistoryStore(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetFileHistoryStore, error) {
	return NewParquetFileHistoryStoreBuilder(logger).
		WithStorageConfig(cfg).
		Build()
}

// URLMutexManager is now in url_mutex_manager.go

// getURLMutex returns a mutex for the specific URL to ensure thread-safety
func (pfs *ParquetFileHistoryStore) getURLMutex(url string) *sync.Mutex {
	if pfs.mutexManager == nil {
		// Fallback to original implementation
		pfs.mutexMapLock.RLock()
		mutex, exists := pfs.urlMutexes[url]
		pfs.mutexMapLock.RUnlock()

		if exists {
			return mutex
		}

		pfs.mutexMapLock.Lock()
		defer pfs.mutexMapLock.Unlock()

		if mutex, exists := pfs.urlMutexes[url]; exists {
			return mutex
		}

		mutex = &sync.Mutex{}
		pfs.urlMutexes[url] = mutex
		return mutex
	}

	return pfs.mutexManager.GetMutex(url)
}

// FilePathGenerator is now in file_path_generator.go

// getHistoryFilePath returns the path to the Parquet file for a specific URL
func (pfs *ParquetFileHistoryStore) getHistoryFilePath(recordURL string) (string, error) {
	fpg := NewFilePathGenerator(pfs.storageConfig.ParquetBasePath, pfs.urlHashGen, pfs.logger)
	return fpg.GenerateHistoryFilePath(recordURL)
}

// getAndSortRecordsForURL is now in file_history_readers.go

// readFileHistoryRecords is now in file_history_readers.go

// Writer operations are now in file_history_writers.go

// Core operations are now in file_history_operations.go

// Helper functions are now in file_history_helpers.go

// Diff operations are now in file_history_diff_operations.go
