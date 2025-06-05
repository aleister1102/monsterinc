package datastore

import (
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
)

// ParquetFileHistory implements the models.FileHistoryStore interface using Parquet files.
// Each monitored URL will have its history stored in a separate Parquet file.
type ParquetFileHistory struct {
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

// NewParquetFileHistoryStore creates a new ParquetFileHistoryStore using builder pattern
func NewParquetFileHistoryStore(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetFileHistory, error) {
	return NewParquetFileHistoryStoreBuilder(logger).
		WithStorageConfig(cfg).
		Build()
}
