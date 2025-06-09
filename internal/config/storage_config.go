package config

// StorageConfig defines configuration for data storage
type StorageConfig struct {
	CompressionCodec string `json:"compression_codec,omitempty" yaml:"compression_codec,omitempty"`
	ParquetBasePath  string `json:"parquet_base_path,omitempty" yaml:"parquet_base_path,omitempty"`
}

// NewDefaultStorageConfig creates default storage configuration
func NewDefaultStorageConfig() StorageConfig {
	return StorageConfig{
		CompressionCodec: DefaultStorageCompressionCodec,
		ParquetBasePath:  DefaultStorageParquetBasePath,
	}
}
