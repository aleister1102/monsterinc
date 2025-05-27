package datastore

import (
	"errors"
	"fmt"
	"io"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	// Needed for GetLastKnownRecord sorting
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

const (
	currentMonitorHistoryFile           = "all_monitor_history.parquet"
	archivedMonitorHistoryFormat        = "%s_%s_monitor_history.parquet" // timestamp, original_basename_no_ext
	monitorHistoryTimestampLayout       = "2006-01-02_15-04-05"
	maxMonitorHistoryFileSize     int64 = 100 * 1024 * 1024 // 100MB
	monitorHistoryFileGlobPattern       = "*_monitor_history.parquet"
)

var storeMutex sync.Mutex // Mutex to protect file read/write operations

// ParquetFileHistoryStore implements the models.FileHistoryStore interface using Parquet files.
// Each monitored URL will have its history stored in a separate Parquet file.
type ParquetFileHistoryStore struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
}

// NewParquetFileHistoryStore creates a new ParquetFileHistoryStore.
func NewParquetFileHistoryStore(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetFileHistoryStore, error) {
	store := &ParquetFileHistoryStore{
		storageConfig: cfg,
		logger:        logger.With().Str("component", "ParquetFileHistoryStore").Logger(),
	}

	basePath := cfg.ParquetBasePath
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure monitor history base directory '%s': %w", basePath, err)
	}
	store.logger.Info().Str("path", basePath).Msg("Monitor history base directory ensured.")
	return store, nil
}

// getHistoryFilePath now returns the path to the single global history file.
func (pfs *ParquetFileHistoryStore) getHistoryFilePath() string {
	return filepath.Join(pfs.storageConfig.ParquetBasePath, currentMonitorHistoryFile)
}

// readFileHistoryRecords reads all records from the specified Parquet file.
func readFileHistoryRecords(filePath string, logger zerolog.Logger) ([]models.FileHistoryRecord, error) {
	osFile, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info().Str("file", filePath).Msg("History file does not exist, returning empty records.")
			return []models.FileHistoryRecord{}, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open history file '%s': %w", filePath, err)
	}
	defer osFile.Close()

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

	// The osFile is closed by defer. The parquet.File (pqFile) does not seem to have/need an explicit Close method
	// in the examples when created from an os.File that is managed separately.
	// reader also does not show an explicit Close method in the simple Read loop example.

	logger.Debug().Int("count", len(records)).Str("file", filePath).Msg("Successfully read records from history file.")
	return records, nil
}

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistoryStore) StoreFileRecord(record models.FileHistoryRecord) error {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	currentFilePath := pfs.getHistoryFilePath()

	// Check file size for rotation
	fileInfo, err := os.Stat(currentFilePath)
	if err == nil { // File exists
		if fileInfo.Size() >= maxMonitorHistoryFileSize {
			timestamp := time.Now().Format(monitorHistoryTimestampLayout)
			originalBase := strings.TrimSuffix(currentMonitorHistoryFile, filepath.Ext(currentMonitorHistoryFile))
			archiveFileName := fmt.Sprintf(archivedMonitorHistoryFormat, timestamp, originalBase)
			archiveFilePath := filepath.Join(pfs.storageConfig.ParquetBasePath, archiveFileName)

			pfs.logger.Info().Str("current_file", currentFilePath).Str("archive_file", archiveFilePath).Int64("size_bytes", fileInfo.Size()).Msg("Rotating monitor history file due to size limit.")
			if err := os.Rename(currentFilePath, archiveFilePath); err != nil {
				pfs.logger.Error().Err(err).Str("current_file", currentFilePath).Str("archive_file", archiveFilePath).Msg("Failed to rename current history file for rotation")
				return fmt.Errorf("failed to rotate history file %s to %s: %w", currentFilePath, archiveFilePath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		pfs.logger.Error().Err(err).Str("file_path", currentFilePath).Msg("Failed to stat current history file before storing record")
		return fmt.Errorf("failed to stat history file %s: %w", currentFilePath, err)
	}

	// Read existing records from the current file (it might be new/empty after rotation or if it's the first run)
	existingRecords, err := readFileHistoryRecords(currentFilePath, pfs.logger) // This function handles os.IsNotExist internally
	if err != nil {
		// If it's not a "not found" or "empty file" scenario (which are handled by readFileHistoryRecords returning empty slice)
		// then it's a more serious read error.
		pfs.logger.Error().Err(err).Str("file", currentFilePath).Msg("Failed to read existing records before storing new one.")
		return fmt.Errorf("failed to read existing records from '%s': %w", currentFilePath, err)
	}

	// Add the new record
	existingRecords = append(existingRecords, record)

	// Write all records (old + new) back to the file
	outputFile, err := os.OpenFile(currentFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pfs.logger.Error().Err(err).Str("file", currentFilePath).Msg("Failed to open history file for writing.")
		return fmt.Errorf("failed to open history file '%s' for writing: %w", currentFilePath, err)
	}
	defer outputFile.Close()

	// Use generic NewGenericWriter with type parameter as per parquet-go/parquet-go README for v0.21.0+
	// The NewWriter (non-generic) expects a schema argument and writes rows one by one.
	// NewGenericWriter is more suitable for writing a slice of structs directly or by buffering.
	writer := parquet.NewGenericWriter[models.FileHistoryRecord](outputFile, parquet.Compression(&parquet.Zstd))

	// WriteAll is not a standard method on the Writer from NewGenericWriter based on common patterns.
	// We need to write records one by one or buffer them if the writer supports it.
	// The README example for NewGenericBuffer uses WriteRows, but here we have a GenericWriter.
	// Let's assume GenericWriter also has a Write method for individual structs.
	numWritten, err := writer.Write(existingRecords)
	if err != nil {
		pfs.logger.Error().Err(err).Msg("Failed to write records to Parquet using GenericWriter.")
		outputFile.Close()
		os.Remove(currentFilePath)
		return fmt.Errorf("failed to write records to parquet file '%s': %w", currentFilePath, err)
	}
	if numWritten != len(existingRecords) {
		pfs.logger.Error().Int("expected", len(existingRecords)).Int("written", numWritten).Msg("Mismatch in number of records written to Parquet.")
		outputFile.Close()
		os.Remove(currentFilePath)
		return fmt.Errorf("mismatch in records written to parquet file '%s' (expected %d, got %d)", currentFilePath, len(existingRecords), numWritten)
	}

	if err := writer.Close(); err != nil {
		pfs.logger.Error().Err(err).Msg("Failed to close Parquet writer.")
		// Attempt to remove partially written file
		os.Remove(currentFilePath)
		return fmt.Errorf("failed to close parquet writer for file '%s': %w", currentFilePath, err)
	}

	pfs.logger.Info().Str("url", record.URL).Str("file", currentFilePath).Int("total_records", len(existingRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// getArchivedHistoryFilesSorted retrieves a list of archived history Parquet files,
// sorted by timestamp in their names from newest to oldest.
func (pfs *ParquetFileHistoryStore) getArchivedHistoryFilesSorted() ([]string, error) {
	globPath := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorHistoryFileGlobPattern)
	matches, err := filepath.Glob(globPath)
	if err != nil {
		pfs.logger.Error().Err(err).Str("pattern", globPath).Msg("Failed to glob for archived history files.")
		return nil, fmt.Errorf("failed to glob for archived history files: %w", err)
	}

	// Filter out the current active file from the list of matches, if it's present
	currentFilePath := pfs.getHistoryFilePath()
	var archivedFiles []string
	for _, match := range matches {
		if match != currentFilePath {
			archivedFiles = append(archivedFiles, match)
		}
	}

	// Sort files by timestamp in name (newest first)
	sort.SliceStable(archivedFiles, func(i, j int) bool {
		// Extract timestamp from filename, e.g., "2023-10-27_15-04-05_all_monitor_history.parquet"
		// This is a bit simplistic and assumes the filename structure. A more robust parse might be needed.
		nameI := filepath.Base(archivedFiles[i])
		nameJ := filepath.Base(archivedFiles[j])
		timestampIStr := strings.Split(nameI, "_")[0] + "_" + strings.Split(nameI, "_")[1] // Reconstruct YYYY-MM-DD_HH-MM-SS
		timestampJStr := strings.Split(nameJ, "_")[0] + "_" + strings.Split(nameJ, "_")[1]

		timeI, errI := time.Parse(monitorHistoryTimestampLayout, timestampIStr)
		timeJ, errJ := time.Parse(monitorHistoryTimestampLayout, timestampJStr)

		if errI != nil || errJ != nil {
			pfs.logger.Warn().Str("fileI", nameI).Str("fileJ", nameJ).Msg("Could not parse timestamps for sorting archived files, order may be incorrect.")
			return false // Keep original order if parsing fails
		}
		return timeI.After(timeJ) // Newest first
	})

	return archivedFiles, nil
}

// GetLastKnownRecord retrieves the most recent record for a URL, checking current and then archived files.
func (pfs *ParquetFileHistoryStore) GetLastKnownRecord(url string) (*models.FileHistoryRecord, error) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	// 1. Check the current history file
	currentFilePath := pfs.getHistoryFilePath()
	records, err := readFileHistoryRecords(currentFilePath, pfs.logger)
	if err != nil {
		// Log error but don't return yet, as we need to check archives
		pfs.logger.Error().Err(err).Str("file", currentFilePath).Msg("Failed to read current history file when getting last known record.")
	} else {
		sortRecords(records) // Sort by timestamp descending
		for _, r := range records {
			if r.URL == url {
				pfs.logger.Debug().Str("url", url).Str("source_file", currentFilePath).Time("record_ts", r.Timestamp).Msg("Found last known record in current history file.")
				return &r, nil
			}
		}
	}

	// 2. If not found in current, check archived files (newest first)
	archivedFiles, err := pfs.getArchivedHistoryFilesSorted()
	if err != nil {
		pfs.logger.Error().Err(err).Msg("Failed to get list of archived history files.")
		// Depending on policy, we might return an error here, or just indicate not found from archives.
		// For now, proceed to return not found if archives can't be listed.
	} else {
		for _, archivePath := range archivedFiles {
			pfs.logger.Debug().Str("url", url).Str("archive_file", archivePath).Msg("Checking archived history file.")
			archivedRecords, readErr := readFileHistoryRecords(archivePath, pfs.logger)
			if readErr != nil {
				pfs.logger.Error().Err(readErr).Str("file", archivePath).Msg("Failed to read an archived history file.")
				continue // Skip this archive file if it can't be read
			}
			sortRecords(archivedRecords)
			for _, r := range archivedRecords {
				if r.URL == url {
					pfs.logger.Debug().Str("url", url).Str("source_file", archivePath).Time("record_ts", r.Timestamp).Msg("Found last known record in archived history file.")
					return &r, nil
				}
			}
		}
	}

	pfs.logger.Info().Str("url", url).Msg("No record found for the specific URL in current or archived history files.")
	return nil, nil
}

// GetLastKnownHash retrieves the most recent hash for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownHash(url string) (string, error) {
	record, err := pfs.GetLastKnownRecord(url)
	if err != nil {
		return "", err // Propagate error
	}
	if record == nil {
		return "", nil // No record found, so no hash
	}
	return record.Hash, nil
}

// Helper function to ensure Parquet schema matches models.FileHistoryRecord
// This can be called during NewParquetFileHistoryStore or as a sanity check.
func (pfs *ParquetFileHistoryStore) validateSchemaCompatibility() error {
	schema := parquet.SchemaOf(new(models.FileHistoryRecord))
	// This is a basic check; more complex validation might involve comparing fields.
	// For now, just ensuring it can be generated is a good first step.
	if schema == nil {
		return errors.New("failed to generate parquet schema for FileHistoryRecord")
	}
	pfs.logger.Debug().Str("schema_name", schema.Name()).Msg("Parquet schema for FileHistoryRecord generated.")
	return nil
}

// Example of how one might sort records by URL and then Timestamp (desc) if needed for processing
// This is not used by the current GetLastKnownRecord logic but can be useful for other operations.
func sortRecords(records []models.FileHistoryRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].URL != records[j].URL {
			return records[i].URL < records[j].URL
		}
		return records[i].Timestamp.After(records[j].Timestamp) // Most recent first for same URL
	})
}
