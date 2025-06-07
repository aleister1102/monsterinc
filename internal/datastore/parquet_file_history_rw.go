package datastore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// * Operations

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistory) StoreFileRecord(record models.FileHistoryRecord) error {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(record.URL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	historyFilePath, err := pfs.getHistoryFilePath(record.URL)
	if err != nil {
		return err // Error already logged in getHistoryFilePath
	}

	// pfs.logger.Info().Str("url", record.URL).Str("path", historyFilePath).Msg("Storing file record")

	// Load existing records
	existingRecords, err := pfs.loadExistingRecords(historyFilePath)
	if err != nil {
		return err
	}

	// Check if this exact record already exists (prevent duplicates)
	for _, existingRecord := range existingRecords {
		if existingRecord.URL == record.URL &&
			existingRecord.Hash == record.Hash &&
			existingRecord.Timestamp == record.Timestamp {
			pfs.logger.Debug().Str("url", record.URL).Str("hash", record.Hash).Int64("timestamp", record.Timestamp).Msg("Record already exists, skipping duplicate")
			return nil
		}
	}

	// Always append the new record
	allRecords := append(existingRecords, record)

	// Create parquet file for writing
	file, compressionOption, err := pfs.createParquetFile(historyFilePath)
	if err != nil {
		return err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to close parquet file")
		}
	}()

	// Write all records to the file
	if err := pfs.writeParquetData(file, compressionOption, allRecords); err != nil {
		return fmt.Errorf("writing parquet data to '%s': %w", historyFilePath, err)
	}

	pfs.logger.Info().Str("url", record.URL).Int("total_records", len(allRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
func (pfs *ParquetFileHistory) GetLastKnownRecord(recordURL string) (*models.FileHistoryRecord, error) {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(recordURL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	record := &records[0] // Since records are sorted newest first
	return record, nil
}

// GetLatestRecord retrieves the most recent FileHistoryRecord for a given URL.
// This is functionally an alias for GetLastKnownRecord in the current implementation.
func (pfs *ParquetFileHistory) GetLatestRecord(recordURL string) (*models.FileHistoryRecord, error) {
	return pfs.GetLastKnownRecord(recordURL)
}

// GetRecordsForURL retrieves a limited number of records for a URL, for potential future use.
func (pfs *ParquetFileHistory) GetRecordsForURL(recordURL string, limit int) ([]*models.FileHistoryRecord, error) {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(recordURL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		return nil, err
	}

	// Apply limit if specified
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	// Convert to slice of pointers
	var resultRecords []*models.FileHistoryRecord
	for i := range records {
		resultRecords = append(resultRecords, &records[i])
	}

	return resultRecords, nil
}

// GetLastKnownHash retrieves the most recent hash for a given URL.
func (pfs *ParquetFileHistory) GetLastKnownHash(url string) (string, error) {
	record, err := pfs.GetLastKnownRecord(url)
	if err != nil {
		return "", err // Propagate error
	}
	if record == nil {
		return "", nil // No record found, so no hash
	}
	return record.Hash, nil
}

// GetHostnamesWithHistory retrieves a list of unique hostname:port combinations that have history records.
func (pfs *ParquetFileHistory) GetHostnamesWithHistory() ([]string, error) {
	hostnamesPorts := make([]string, 0)
	seenHostsPorts := make(map[string]bool)

	monitorBaseDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir)

	entries, err := os.ReadDir(monitorBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return hostnamesPorts, nil
		}
		return nil, fmt.Errorf("failed to read monitor base directory '%s': %w", monitorBaseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			hostPortDirName := entry.Name()
			hostPortDir := filepath.Join(monitorBaseDir, hostPortDirName)

			// Check if this directory contains any *_history.parquet files
			dirEntries, err := os.ReadDir(hostPortDir)
			if err != nil {
				pfs.logger.Warn().Err(err).Str("dir", hostPortDir).Msg("Error reading host directory, skipping.")
				continue
			}

			hasHistoryFiles := false
			for _, dirEntry := range dirEntries {
				if !dirEntry.IsDir() && strings.HasSuffix(dirEntry.Name(), "_history.parquet") {
					hasHistoryFiles = true
					break
				}
			}

			if hasHistoryFiles {
				// Convert sanitized directory name back to hostname:port format
				hostPortRestored := urlhandler.RestoreHostnamePort(hostPortDirName)
				if !seenHostsPorts[hostPortRestored] {
					hostnamesPorts = append(hostnamesPorts, hostPortRestored)
					seenHostsPorts[hostPortRestored] = true
				}
			}
		}
	}

	pfs.logger.Info().Int("count", len(hostnamesPorts)).Msg("Successfully retrieved hostname:port combinations with history.")
	return hostnamesPorts, nil
}

// * Private methods for reading from parquet files

// getAndSortRecordsForURL is an internal helper to get the file path for a URL,
// read its records using readFileHistoryRecords, which also sorts them (newest first).
func (pfs *ParquetFileHistory) getAndSortRecordsForURL(recordURL string) ([]models.FileHistoryRecord, error) {
	historyFilePath, err := pfs.getHistoryFilePath(recordURL)
	if err != nil {
		// Error already logged by getHistoryFilePath if it's critical for path formation
		return nil, fmt.Errorf("failed to get history file path for '%s': %w", recordURL, err)
	}

	// readFileHistoryRecords reads, sorts (newest first), and handles os.IsNotExist by returning ([]models.FileHistoryRecord{}, nil)
	records, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil {
		// This 'err' would be a more severe error than file not found, as that case is handled in readFileHistoryRecords.
		pfs.logger.Error().Err(err).Str("url", recordURL).Str("path", historyFilePath).Msg("Error reading and sorting records from history file")
		return nil, fmt.Errorf("error reading history from '%s': %w", historyFilePath, err)
	}
	return records, nil
}

// readFileHistoryRecords reads all records from the specified Parquet file.
func readFileHistoryRecords(filePath string, logger zerolog.Logger) ([]models.FileHistoryRecord, error) {
	osFile, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// logger.Info().Str("file", filePath).Msg("History file does not exist, returning empty records.")
			return []models.FileHistoryRecord{}, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open history file '%s': %w", filePath, err)
	}
	defer func() {
		err := osFile.Close()
		if err != nil {
			logger.Error().Err(err).Str("file", filePath).Msg("Failed to close history file")
		}
	}()

	stat, err := osFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat history file '%s': %w", filePath, err)
	}
	if stat.Size() == 0 {
		logger.Info().Str("file", filePath).Msg("History file is empty, returning empty records.")
		return []models.FileHistoryRecord{}, nil // Return empty slice if file is empty
	}

	pqFile, err := parquet.OpenFile(osFile, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file '%s': %w", filePath, err)
	}

	// According to parquet-go/parquet-go README examples:
	// reader := parquet.NewReader(f) // where f is *parquet.File
	// Then loop reader.Read(&row)
	reader := parquet.NewReader(pqFile)

	var records []models.FileHistoryRecord
	for {
		var record models.FileHistoryRecord
		if err := reader.Read(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break // End of file
			}
			logger.Error().Err(err).Str("file", filePath).Msg("Error reading record from Parquet file")
			return nil, fmt.Errorf("error reading record from parquet file '%s': %w", filePath, err)
		}
		records = append(records, record)
	}

	// Sort records by Timestamp descending (newest first)
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp // Compare int64 directly
	})

	// The osFile is closed by defer. The parquet.File (pqFile) does not seem to have/need an explicit Close method
	// in the examples when created from an os.File that is managed separately.
	// reader also does not show an explicit Close method in the simple Read loop example.

	logger.Debug().Int("count", len(records)).Str("file", filePath).Msg("Successfully read and sorted records from history file.")
	return records, nil
}

// Helper to read all records from a single parquet file.
func (pfs *ParquetFileHistory) readRecordsFromFile(filePath string) ([]*models.FileHistoryRecord, error) {
	records, err := readFileHistoryRecords(filePath, pfs.logger)
	if err != nil {
		return nil, err
	}

	recordPtrs := make([]*models.FileHistoryRecord, len(records))
	for i, rec := range records {
		recordPtrs[i] = &rec
	}
	return recordPtrs, nil
}

// * Private methods for writing to parquet files

// createParquetFile creates a new parquet file for writing with proper compression settings
func (pfs *ParquetFileHistory) createParquetFile(filePath string) (*os.File, parquet.WriterOption, error) {
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
func (pfs *ParquetFileHistory) writeParquetData(file *os.File, compressionOption parquet.WriterOption, allRecords []models.FileHistoryRecord) error {
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
func (pfs *ParquetFileHistory) loadExistingRecords(historyFilePath string) ([]models.FileHistoryRecord, error) {
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

// * Helpers

// getURLMutex returns a mutex for the specific URL to ensure thread-safety
func (pfs *ParquetFileHistory) getURLMutex(url string) *sync.Mutex {
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

// getHistoryFilePath returns the path to the Parquet file for a specific URL
func (pfs *ParquetFileHistory) getHistoryFilePath(recordURL string) (string, error) {
	fpg := NewFilePathGenerator(pfs.storageConfig.ParquetBasePath, pfs.urlHashGen, pfs.logger)
	return fpg.GenerateHistoryFilePath(recordURL)
}
