package datastore

// File history store constants
const (
	fileHistoryArchiveSubDir            = "archive"
	fileHistoryCurrentFile              = "current_history.parquet"
	monitorDataDir                      = "monitor"
	currentMonitorHistoryFile           = "all_monitor_history.parquet"
	archivedMonitorHistoryFormat        = "%s_%s_monitor_history.parquet"
	monitorHistoryTimestampLayout       = "2006-01-02_15-04-05"
	maxMonitorHistoryFileSize     int64 = 100 * 1024 * 1024 // 100MB
	monitorHistoryFileGlobPattern       = "*_monitor_history.parquet"
)

// ParquetFileHistoryStoreConfig holds configuration for the file history store
type ParquetFileHistoryStoreConfig struct {
	MaxFileSize       int64
	EnableCompression bool
	CompressionCodec  string
	EnableURLMutexes  bool
	CleanupInterval   int
}

// DefaultParquetFileHistoryStoreConfig returns default configuration
func DefaultParquetFileHistoryStoreConfig() ParquetFileHistoryStoreConfig {
	return ParquetFileHistoryStoreConfig{
		MaxFileSize:       maxMonitorHistoryFileSize,
		EnableCompression: true,
		CompressionCodec:  "zstd",
		EnableURLMutexes:  true,
		CleanupInterval:   3600, // 1 hour in seconds
	}
}
