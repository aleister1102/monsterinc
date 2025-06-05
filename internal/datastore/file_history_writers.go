package datastore

import (
	"fmt"
	"os"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/parquet-go/parquet-go"
)

// createParquetFile creates a new parquet file for writing with proper compression settings
func (pfs *ParquetFileHistoryStore) createParquetFile(filePath string) (*os.File, parquet.WriterOption, error) {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", filePath).Msg("Failed to open/create history file for writing")
		return nil, nil, fmt.Errorf("opening/creating history file '%s': %w", filePath, err)
	}

	// Get the compression codec from config string
	compressionOption := parquet.Compression(&parquet.Uncompressed) // Default to Uncompressed

	switch strings.ToLower(pfs.storageConfig.CompressionCodec) {
	case "snappy":
		compressionOption = parquet.Compression(&parquet.Snappy)
	case "gzip":
		// For Gzip, you might want to configure the compression level if the library supports it.
		// Defaulting to the library's default Gzip compression.
		compressionOption = parquet.Compression(&parquet.Gzip)
	case "zstd":
		compressionOption = parquet.Compression(&parquet.Zstd)
	case "none", "uncompressed", "":
		// Already default
	default:
		pfs.logger.Warn().Str("codec", pfs.storageConfig.CompressionCodec).Msg("Unsupported compression codec string, defaulting to Uncompressed")
	}

	return file, compressionOption, nil
}

// writeParquetData writes all records to the parquet file
func (pfs *ParquetFileHistoryStore) writeParquetData(file *os.File, compressionOption parquet.WriterOption, allRecords []models.FileHistoryRecord) error {
	writer := parquet.NewWriter(file, parquet.SchemaOf(models.FileHistoryRecord{}), compressionOption)

	for _, rec := range allRecords {
		if err := writer.Write(rec); err != nil {
			pfs.logger.Error().Err(err).Str("url", rec.URL).Msg("Failed to write record to Parquet file")
			// Decide if we should continue or return an error. For now, log and continue.
		}
	}

	if err := writer.Close(); err != nil {
		pfs.logger.Error().Err(err).Msg("Failed to close Parquet writer")
		return fmt.Errorf("closing Parquet writer: %w", err)
	}

	return nil
}

// loadExistingRecords loads existing records from the history file
func (pfs *ParquetFileHistoryStore) loadExistingRecords(historyFilePath string) ([]models.FileHistoryRecord, error) {
	existingRecords, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil && !os.IsNotExist(err) {
		// Log error but attempt to overwrite if it's not a simple "not found" error
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Error reading existing history file, will attempt to overwrite")
		return []models.FileHistoryRecord{}, nil // Reset to ensure we write a new file
	} else if os.IsNotExist(err) {
		pfs.logger.Info().Str("path", historyFilePath).Msg("History file does not exist, creating new one.")
		return []models.FileHistoryRecord{}, nil
	}
	return existingRecords, nil
}
