package datastore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	// Needed for GetLastKnownRecord sorting
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

const (
	fileHistoryArchiveSubDir            = "archive"
	fileHistoryCurrentFile              = "current_history.parquet"
	monitorDataDir                      = "monitor" // New constant for monitor data subdirectory
	currentMonitorHistoryFile           = "all_monitor_history.parquet"
	archivedMonitorHistoryFormat        = "%s_%s_monitor_history.parquet" // timestamp, original_basename_no_ext
	monitorHistoryTimestampLayout       = "2006-01-02_15-04-05"
	maxMonitorHistoryFileSize     int64 = 100 * 1024 * 1024 // 100MB
	monitorHistoryFileGlobPattern       = "*_monitor_history.parquet"
)

// ParquetFileHistoryStore implements the models.FileHistoryStore interface using Parquet files.
// Each monitored URL will have its history stored in a separate Parquet file.
type ParquetFileHistoryStore struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	// Add mutex map for per-URL locking
	urlMutexes   map[string]*sync.Mutex
	mutexMapLock sync.RWMutex
}

// NewParquetFileHistoryStore creates a new ParquetFileHistoryStore.
func NewParquetFileHistoryStore(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetFileHistoryStore, error) {
	store := &ParquetFileHistoryStore{
		storageConfig: cfg,
		logger:        logger.With().Str("component", "ParquetFileHistoryStore").Logger(),
		urlMutexes:    make(map[string]*sync.Mutex),
	}

	basePath := cfg.ParquetBasePath
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure monitor history base directory '%s': %w", basePath, err)
	}
	store.logger.Info().Str("path", basePath).Msg("Monitor history base directory ensured.")
	return store, nil
}

// getURLMutex returns a mutex for the specific URL to ensure thread-safety
func (pfs *ParquetFileHistoryStore) getURLMutex(url string) *sync.Mutex {
	pfs.mutexMapLock.RLock()
	mutex, exists := pfs.urlMutexes[url]
	pfs.mutexMapLock.RUnlock()

	if exists {
		return mutex
	}

	pfs.mutexMapLock.Lock()
	defer pfs.mutexMapLock.Unlock()

	// Double-check after acquiring write lock
	if mutex, exists := pfs.urlMutexes[url]; exists {
		return mutex
	}

	mutex = &sync.Mutex{}
	pfs.urlMutexes[url] = mutex
	return mutex
}

// generateURLHash creates a unique hash for the URL to use as filename
func (pfs *ParquetFileHistoryStore) generateURLHash(url string) string {
	hasher := sha256.New()
	hasher.Write([]byte(url))
	return hex.EncodeToString(hasher.Sum(nil))[:16] // Use first 16 chars for shorter filename
}

// getHistoryFilePath returns the path to the Parquet file for a specific URL.
// Now creates a unique file for each URL using URL hash.
func (pfs *ParquetFileHistoryStore) getHistoryFilePath(recordURL string) (string, error) {
	hostnameWithPort, err := urlhandler.ExtractHostnameWithPort(recordURL)
	if err != nil {
		pfs.logger.Error().Err(err).Str("url", recordURL).Msg("Failed to extract hostname:port for history file path")
		return "", fmt.Errorf("extracting hostname:port from URL '%s': %w", recordURL, err)
	}

	// Sanitize hostname:port for directory name using specialized function
	sanitizedHostPort := urlhandler.SanitizeHostnamePort(hostnameWithPort)

	// Generate unique hash for this specific URL
	urlHash := pfs.generateURLHash(recordURL)

	// Create filename: <url_hash>_history.parquet
	fileName := fmt.Sprintf("%s_history.parquet", urlHash)

	// Base path for all monitor data: <storageConfig.ParquetBasePath>/monitor/<sanitizedHostPort>/<url_hash>_history.parquet
	urlSpecificDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir, sanitizedHostPort)
	if err := os.MkdirAll(urlSpecificDir, 0755); err != nil {
		pfs.logger.Error().Err(err).Str("directory", urlSpecificDir).Msg("Failed to create URL-specific directory for history file")
		return "", fmt.Errorf("creating directory '%s': %w", urlSpecificDir, err)
	}

	filePath := filepath.Join(urlSpecificDir, fileName)
	pfs.logger.Debug().Str("url", recordURL).Str("file_path", filePath).Str("url_hash", urlHash).Msg("Generated history file path")

	return filePath, nil
}

// getAndSortRecordsForURL is an internal helper to get the file path for a URL,
// read its records using readFileHistoryRecords, which also sorts them (newest first).
func (pfs *ParquetFileHistoryStore) getAndSortRecordsForURL(recordURL string) ([]models.FileHistoryRecord, error) {
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

// createParquetFile creates a new parquet file for writing with proper compression settings
func (pfs *ParquetFileHistoryStore) createParquetFile(filePath string) (*os.File, parquet.WriterOption, error) {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", filePath).Msg("Failed to open/create history file for writing")
		return nil, nil, fmt.Errorf("opening/creating history file '%s': %w", filePath, err)
	}

	// Get the compression codec from config string
	var compressionOption parquet.WriterOption = parquet.Compression(&parquet.Uncompressed) // Default to Uncompressed

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

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistoryStore) StoreFileRecord(record models.FileHistoryRecord) error {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(record.URL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	historyFilePath, err := pfs.getHistoryFilePath(record.URL)
	if err != nil {
		return err // Error already logged in getHistoryFilePath
	}

	pfs.logger.Info().Str("url", record.URL).Str("path", historyFilePath).Msg("Storing file record")

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
	defer file.Close()

	// Write all records to the file
	if err := pfs.writeParquetData(file, compressionOption, allRecords); err != nil {
		return fmt.Errorf("writing parquet data to '%s': %w", historyFilePath, err)
	}

	pfs.logger.Info().Str("url", record.URL).Int("total_records", len(allRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownRecord(recordURL string) (*models.FileHistoryRecord, error) {
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
func (pfs *ParquetFileHistoryStore) GetLatestRecord(recordURL string) (*models.FileHistoryRecord, error) {
	return pfs.GetLastKnownRecord(recordURL)
}

// GetRecordsForURL retrieves a limited number of records for a URL, for potential future use.
func (pfs *ParquetFileHistoryStore) GetRecordsForURL(recordURL string, limit int) ([]*models.FileHistoryRecord, error) {
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
	schema := parquet.SchemaOf(models.FileHistoryRecord{})
	if schema == nil {
		return fmt.Errorf("failed to generate parquet schema for FileHistoryRecord")
	}
	return nil
}

// scanHistoryFile reads a history file and returns records that have diff data.
func (pfs *ParquetFileHistoryStore) scanHistoryFile(filePath string) ([]*models.FileHistoryRecord, error) {
	if strings.Contains(filepath.Base(filePath), "archived") {
		return []*models.FileHistoryRecord{}, nil
	}

	records, err := pfs.readRecordsFromFile(filePath)
	if err != nil {
		return nil, err
	}

	var diffRecords []*models.FileHistoryRecord
	for _, record := range records {
		if record.DiffResultJSON != nil && *record.DiffResultJSON != "" && *record.DiffResultJSON != "null" {
			diffRecords = append(diffRecords, record)
		}
	}

	return diffRecords, nil
}

// walkDirectoryForDiffs walks through the monitor directory to find history files with diffs
func (pfs *ParquetFileHistoryStore) walkDirectoryForDiffs(monitorBaseDir string) ([]*models.FileHistoryRecord, error) {
	allDiffRecords := make([]*models.FileHistoryRecord, 0)

	// Walk through the monitorBaseDir to find all host-specific directories
	// then look for *_history.parquet files in each.
	err := filepath.WalkDir(monitorBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log the error but try to continue if possible, unless it's a critical error like permission denied to the root
			pfs.logger.Error().Err(err).Str("path", path).Msg("Error accessing path during walk for diff records")
			// If the error is related to the root directory itself, we probably can't proceed.
			if path == monitorBaseDir {
				return fmt.Errorf("error walking root monitor directory %s: %w", monitorBaseDir, err) // Stop walking
			}
			return nil // Skip this entry and continue
		}

		// We are looking for *_history.parquet files (new pattern)
		if !d.IsDir() && strings.HasSuffix(d.Name(), "_history.parquet") {
			diffRecords, scanErr := pfs.scanHistoryFile(path)
			if scanErr != nil {
				pfs.logger.Error().Err(scanErr).Str("file", path).Msg("Error scanning history file for diffs")
				return nil // Continue walking despite error
			}
			if diffRecords != nil {
				allDiffRecords = append(allDiffRecords, diffRecords...)
			}
		}
		return nil
	})

	if err != nil {
		// This error is from filepath.WalkDir itself (e.g., root dir not accessible)
		pfs.logger.Error().Err(err).Str("base_dir", monitorBaseDir).Msg("Failed to walk history directories to get records with diffs")
		return nil, fmt.Errorf("failed to walk history directories: %w", err)
	}

	return allDiffRecords, nil
}

// GetAllRecordsWithDiff retrieves all stored file history records that contain diff data.
func (pfs *ParquetFileHistoryStore) GetAllRecordsWithDiff() ([]*models.FileHistoryRecord, error) {
	monitorBaseDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir)

	allDiffRecords, err := pfs.walkDirectoryForDiffs(monitorBaseDir)
	if err != nil {
		return nil, err
	}

	pfs.logger.Info().Int("count", len(allDiffRecords)).Msg("Successfully retrieved all records with diffs.")
	return allDiffRecords, nil
}

// Helper to read all records from a single parquet file.
func (pfs *ParquetFileHistoryStore) readRecordsFromFile(filePath string) ([]*models.FileHistoryRecord, error) {
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

// GetHostnamesWithHistory retrieves a list of unique hostname:port combinations that have history records.
// This method scans the base monitor directory for subdirectories (each representing a hostname:port)
// and checks if they contain any *_history.parquet files.
func (pfs *ParquetFileHistoryStore) GetHostnamesWithHistory() ([]string, error) {
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

// groupURLsByHost groups URLs by their hostname:port for optimized processing
func (pfs *ParquetFileHistoryStore) groupURLsByHost(urls []string) map[string]string {
	urlHostMap := make(map[string]string) // To optimize file reads, group URLs by host:port

	for _, u := range urls {
		hostnameWithPort, err := urlhandler.ExtractHostnameWithPort(u)
		if err != nil {
			pfs.logger.Warn().Err(err).Str("url", u).Msg("Failed to extract hostname:port from URL, skipping for latest diff result.")
			continue
		}
		urlHostMap[u] = hostnameWithPort
	}

	return urlHostMap
}

// processHostRecordsForDiffs processes host records to find the latest diff for each URL
func (pfs *ParquetFileHistoryStore) processHostRecordsForDiffs(hostRecords []models.FileHistoryRecord, targetURLs []string) map[string]*models.ContentDiffResult {
	results := make(map[string]*models.ContentDiffResult)

	// For each target URL, find the latest record with diff
	for _, targetURL := range targetURLs {
		var latestDiffRecord *models.FileHistoryRecord
		var latestTimestamp int64 = 0

		// Search through all records for this URL to find the latest one with diff
		for _, record := range hostRecords {
			if record.URL == targetURL && record.DiffResultJSON != nil && *record.DiffResultJSON != "" && *record.DiffResultJSON != "null" {
				// Found a record with diff for this URL, check if it's the latest
				if record.Timestamp > latestTimestamp {
					latestTimestamp = record.Timestamp
					recordCopy := record // Create a copy to avoid pointer issues
					latestDiffRecord = &recordCopy
				}
			}
		}

		// If we found a latest diff record, unmarshal it
		if latestDiffRecord != nil {
			var diffResult models.ContentDiffResult
			if err := json.Unmarshal([]byte(*latestDiffRecord.DiffResultJSON), &diffResult); err != nil {
				pfs.logger.Error().Err(err).Str("url", targetURL).Int64("timestamp", latestDiffRecord.Timestamp).Msg("Failed to unmarshal DiffResultJSON for latest diff.")
				continue // Skip this URL
			}

			// Unmarshal ExtractedPathsJSON if available
			if latestDiffRecord.ExtractedPathsJSON != nil && *latestDiffRecord.ExtractedPathsJSON != "" {
				var extractedPaths []models.ExtractedPath
				if err := json.Unmarshal([]byte(*latestDiffRecord.ExtractedPathsJSON), &extractedPaths); err != nil {
					pfs.logger.Error().Err(err).Str("url", targetURL).Msg("Failed to unmarshal ExtractedPathsJSON for latest diff.")
					// Do not assign to diffResult.ExtractedPaths if unmarshaling fails, it will remain nil or empty
				} else {
					diffResult.ExtractedPaths = extractedPaths
				}
			}

			results[targetURL] = &diffResult
		}
	}

	return results
}

// GetAllLatestDiffResultsForURLs retrieves the latest diff result for each of the specified URLs.
func (pfs *ParquetFileHistoryStore) GetAllLatestDiffResultsForURLs(urls []string) (map[string]*models.ContentDiffResult, error) {
	results := make(map[string]*models.ContentDiffResult)
	urlHostMap := pfs.groupURLsByHost(urls)

	// Process URLs grouped by host to minimize file reads
	processedHosts := make(map[string]struct{})
	hostURLMap := make(map[string][]string) // Group URLs by host for processing

	// Build reverse mapping: host -> URLs
	for u, host := range urlHostMap {
		hostURLMap[host] = append(hostURLMap[host], u)
	}

	for host, urlsForHost := range hostURLMap {
		if _, processed := processedHosts[host]; processed {
			continue // Already processed this host
		}

		// Get all records for the hostname:port, which are sorted newest first by readFileHistoryRecords
		hostRecords, err := pfs.getAndSortRecordsForHost(host) // host is now hostname:port
		if err != nil {
			pfs.logger.Warn().Err(err).Str("host_with_port", host).Msg("Could not get records for hostname:port when fetching latest diffs.")
			continue
		}

		processedHosts[host] = struct{}{}

		// Process records for all URLs of this hostname:port
		hostResults := pfs.processHostRecordsForDiffs(hostRecords, urlsForHost)
		for url, diffResult := range hostResults {
			results[url] = diffResult
		}
	}

	return results, nil
}

// getAndSortRecordsForHost is a helper to get all records for a hostname:port, sorted newest first.
// This now reads from all *_history.parquet files in the host directory.
func (pfs *ParquetFileHistoryStore) getAndSortRecordsForHost(hostWithPort string) ([]models.FileHistoryRecord, error) {
	sanitizedHostPort := urlhandler.SanitizeHostnamePort(hostWithPort)
	hostSpecificDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir, sanitizedHostPort)

	if _, err := os.Stat(hostSpecificDir); os.IsNotExist(err) {
		return []models.FileHistoryRecord{}, nil // Return empty slice, not an error
	}

	// Read all *_history.parquet files in the directory
	dirEntries, err := os.ReadDir(hostSpecificDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read host directory '%s': %w", hostSpecificDir, err)
	}

	var allRecords []models.FileHistoryRecord
	for _, entry := range dirEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_history.parquet") {
			filePath := filepath.Join(hostSpecificDir, entry.Name())
			records, err := readFileHistoryRecords(filePath, pfs.logger)
			if err != nil {
				pfs.logger.Error().Err(err).Str("file", filePath).Msg("Error reading history file for host")
				continue // Skip this file but continue with others
			}
			allRecords = append(allRecords, records...)
		}
	}

	// Sort all records by timestamp descending (newest first)
	sort.SliceStable(allRecords, func(i, j int) bool {
		return allRecords[i].Timestamp > allRecords[j].Timestamp
	})

	return allRecords, nil
}

// GetAllDiffResults retrieves all diff results from all history files.
// This is a potentially expensive operation.
func (pfs *ParquetFileHistoryStore) GetAllDiffResults() ([]models.ContentDiffResult, error) {
	// Implementation of GetAllDiffResults method
	// This is a placeholder and should be implemented based on the actual requirements
	return []models.ContentDiffResult{}, nil
}
