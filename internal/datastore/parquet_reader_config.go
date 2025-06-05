package datastore

// ParquetReaderConfig holds configuration for ParquetReader
type ParquetReaderConfig struct {
	BufferSize int
	ReadAll    bool
}

// DefaultParquetReaderConfig returns default configuration
func DefaultParquetReaderConfig() ParquetReaderConfig {
	return ParquetReaderConfig{
		BufferSize: 64 * 1024, // 64KB
		ReadAll:    true,
	}
}
