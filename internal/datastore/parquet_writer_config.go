package datastore

// ParquetWriterConfig holds configuration for ParquetWriter
type ParquetWriterConfig struct {
	CompressionType  string
	BatchSize        int
	EnableValidation bool
}

// DefaultParquetWriterConfig returns default configuration
func DefaultParquetWriterConfig() ParquetWriterConfig {
	return ParquetWriterConfig{
		CompressionType:  "zstd",
		BatchSize:        1000,
		EnableValidation: true,
	}
}
