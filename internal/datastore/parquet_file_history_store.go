package datastore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// getHistoryFilePath returns the path to the Parquet file for a specific URL.
// It now creates a directory structure based on the URL's host.
func (pfs *ParquetFileHistoryStore) getHistoryFilePath(recordURL string) (string, error) {
	parsedURL, err := url.Parse(recordURL)
	if err != nil {
		pfs.logger.Error().Err(err).Str("url", recordURL).Msg("Failed to parse URL for history file path")
		return "", fmt.Errorf("parsing URL '%s': %w", recordURL, err)
	}
	host := parsedURL.Host
	sanitizedHost := strings.ReplaceAll(host, ":", "_") // Sanitize host for directory name
	sanitizedHost = strings.ReplaceAll(sanitizedHost, " ", "_")

	// Base path for all monitor data: <storageConfig.ParquetBasePath>/monitor/<sanitizedHost>/current_history.parquet
	urlSpecificDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir, sanitizedHost)
	if err := os.MkdirAll(urlSpecificDir, 0755); err != nil {
		pfs.logger.Error().Err(err).Str("directory", urlSpecificDir).Msg("Failed to create URL-specific directory for history file")
		return "", fmt.Errorf("creating directory '%s': %w", urlSpecificDir, err)
	}
	return filepath.Join(urlSpecificDir, fileHistoryCurrentFile), nil
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

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistoryStore) StoreFileRecord(record models.FileHistoryRecord) error {
	historyFilePath, err := pfs.getHistoryFilePath(record.URL)
	if err != nil {
		return err // Error already logged in getHistoryFilePath
	}

	pfs.logger.Info().Str("url", record.URL).Str("path", historyFilePath).Msg("Storing file record")

	existingRecords, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil && !os.IsNotExist(err) {
		// Log error but attempt to overwrite if it's not a simple "not found" error
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Error reading existing history file, will attempt to overwrite")
		existingRecords = []models.FileHistoryRecord{} // Reset to ensure we write a new file
	} else if os.IsNotExist(err) {
		pfs.logger.Info().Str("path", historyFilePath).Msg("History file does not exist, creating new one.")
		existingRecords = []models.FileHistoryRecord{}
	}

	// Always append the new record
	allRecords := append(existingRecords, record)

	// Write all records back (effectively appending by rewriting)
	file, err := os.OpenFile(historyFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to open/create history file for writing")
		return fmt.Errorf("opening/creating history file '%s': %w", historyFilePath, err)
	}
	defer file.Close()

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

	writer := parquet.NewWriter(file, parquet.SchemaOf(models.FileHistoryRecord{}), compressionOption)
	for _, rec := range allRecords {
		if err := writer.Write(rec); err != nil {
			pfs.logger.Error().Err(err).Str("url", rec.URL).Msg("Failed to write record to Parquet file")
			// Decide if we should continue or return an error. For now, log and continue.
		}
	}

	if err := writer.Close(); err != nil {
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to close Parquet writer")
		return fmt.Errorf("closing Parquet writer for '%s': %w", historyFilePath, err)
	}
	pfs.logger.Info().Str("url", record.URL).Int("total_records", len(allRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// getArchivedHistoryFilesSorted is no longer directly applicable with per-URL files.
// This function would need to be re-thought if global archiving is needed.
// For now, let's assume GetLastKnownRecord will operate on the single current_history.parquet per URL.

// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownRecord(recordURL string) (*models.FileHistoryRecord, error) {
	pfs.logger.Debug().Str("url", recordURL).Msg("Attempting to get last known record using internal helper")

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		// Errors (like file not found, or critical read errors) are logged by getAndSortRecordsForURL or its callees.
		// If it's a 'file not found' scenario, getAndSortRecordsForURL will return (nil, nil) or an empty slice from readFileHistoryRecords.
		// For other errors, it will return (nil, actualError).
		// If os.IsNotExist was the root cause in readFileHistoryRecords, it would have returned an empty slice and nil error.
		// Let's refine this for clarity. readFileHistoryRecords returns empty slice + nil for NotExist.
		// So, if err is not nil here, it's a more significant error.
		pfs.logger.Error().Err(err).Str("url", recordURL).Msg("Failed to get and sort records for GetLastKnownRecord")
		return nil, err
		// If err is nil, but records might be empty (e.g. file not found or empty file case handled by readFileHistoryRecords)
		// This is handled by len(records) == 0 check below.
	}

	if len(records) == 0 {
		pfs.logger.Info().Str("url", recordURL).Msg("History file is empty, no last known record.")
		return nil, nil
	}

	// Filter records to find the one matching the specific recordURL
	for _, record := range records { // records are sorted newest first for the host
		if record.URL == recordURL {
			pfs.logger.Debug().Str("url", recordURL).Int64("timestamp_unix_ms", record.Timestamp).Str("hash", record.Hash).Msg("Retrieved last known record for specific URL")
			return &record, nil
		}
	}

	pfs.logger.Info().Str("url", recordURL).Msg("No record found for the specific URL within the host's history file.")
	return nil, nil // No record found for the specific URL
}

// GetLatestRecord retrieves the most recent FileHistoryRecord for a given URL.
// This is functionally an alias for GetLastKnownRecord in the current implementation.
func (pfs *ParquetFileHistoryStore) GetLatestRecord(recordURL string) (*models.FileHistoryRecord, error) {
	return pfs.GetLastKnownRecord(recordURL)
}

// GetRecordsForURL retrieves historical records for a given URL, up to a specified limit.
// Records are sorted by timestamp in descending order (newest first).
func (pfs *ParquetFileHistoryStore) GetRecordsForURL(recordURL string, limit int) ([]*models.FileHistoryRecord, error) {
	pfs.logger.Debug().Str("url", recordURL).Int("limit", limit).Msg("Attempting to get records for URL using internal helper")

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		// Similar error handling as in GetLastKnownRecord
		pfs.logger.Error().Err(err).Str("url", recordURL).Msg("Failed to get and sort records for GetRecordsForURL")
		return nil, err
	}

	// Filter records for the specific URL first
	urlSpecificRecords := make([]models.FileHistoryRecord, 0)
	for _, record := range records { // records are sorted newest first for the host
		if record.URL == recordURL {
			urlSpecificRecords = append(urlSpecificRecords, record)
		}
	}

	if len(urlSpecificRecords) == 0 {
		pfs.logger.Info().Str("url", recordURL).Msg("History file for host is empty or no records match specific URL, returning empty list.")
		return []*models.FileHistoryRecord{}, nil
	}

	// Now apply limit to the filtered and sorted (newest first) records
	numToReturn := len(urlSpecificRecords)
	if limit > 0 && limit < numToReturn {
		numToReturn = limit
	}

	resultRecords := make([]*models.FileHistoryRecord, 0, numToReturn)
	for i := 0; i < numToReturn; i++ {
		rec := urlSpecificRecords[i] // Create a new variable in the loop scope for the pointer
		resultRecords = append(resultRecords, &rec)
	}

	pfs.logger.Debug().Str("url", recordURL).Int("count", len(resultRecords)).Msg("Retrieved records for URL")
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
	schema := parquet.SchemaOf(new(models.FileHistoryRecord))
	// This is a basic check; more complex validation might involve comparing fields.
	// For now, just ensuring it can be generated is a good first step.
	if schema == nil {
		return errors.New("failed to generate parquet schema for FileHistoryRecord")
	}
	pfs.logger.Debug().Str("schema_name", schema.Name()).Msg("Parquet schema for FileHistoryRecord generated.")
	return nil
}

// GetAllRecordsWithDiff retrieves all records that have a non-empty DiffResultJSON field
// from all current_history.parquet files within the monitor data directory.
func (pfs *ParquetFileHistoryStore) GetAllRecordsWithDiff() ([]*models.FileHistoryRecord, error) {
	pfs.logger.Debug().Msg("Attempting to get all records with diffs.")
	allDiffRecords := make([]*models.FileHistoryRecord, 0)

	monitorBaseDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir)
	pfs.logger.Info().Str("monitor_dir", monitorBaseDir).Msg("Scanning for history files with diffs.")

	// Walk through the monitorBaseDir to find all host-specific directories
	// then look for current_history.parquet in each.
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

		// We are looking for current_history.parquet files
		if !d.IsDir() && d.Name() == fileHistoryCurrentFile {
			pfs.logger.Debug().Str("file_path", path).Msg("Found history file, attempting to read records with diffs.")
			// Check if this path is directly inside a host directory (e.g., monitor/example.com/current_history.parquet)
			// or deeper (archive), we only want the "current" ones outside archive for this specific method.
			parentDir := filepath.Dir(path)
			if filepath.Base(parentDir) == fileHistoryArchiveSubDir {
				pfs.logger.Debug().Str("file_path", path).Msg("Skipping archived history file.")
				return nil
			}

			records, readErr := pfs.readRecordsFromFile(path)
			if readErr != nil {
				pfs.logger.Error().Err(readErr).Str("file", path).Msg("Failed to read records from history file")
				return nil // Continue to the next file
			}

			for _, rec := range records {
				if rec.DiffResultJSON != nil && *rec.DiffResultJSON != "" && *rec.DiffResultJSON != "null" {
					allDiffRecords = append(allDiffRecords, rec)
				}
			}
		}
		return nil
	})

	if err != nil {
		// This error is from filepath.WalkDir itself (e.g., root dir not accessible)
		pfs.logger.Error().Err(err).Str("base_dir", monitorBaseDir).Msg("Failed to walk history directories to get records with diffs")
		return nil, fmt.Errorf("failed to walk history directories: %w", err)
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

// GetHostnamesWithHistory retrieves a list of unique hostnames that have history records.
// This method scans the base monitor directory for subdirectories (each representing a host)
// and checks if they contain a current_history.parquet file.
func (pfs *ParquetFileHistoryStore) GetHostnamesWithHistory() ([]string, error) {
	pfs.logger.Debug().Msg("Attempting to get all hostnames with history.")
	hostnames := make([]string, 0)
	seenHosts := make(map[string]bool)

	monitorBaseDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir)
	pfs.logger.Info().Str("monitor_dir", monitorBaseDir).Msg("Scanning for host directories with history.")

	entries, err := os.ReadDir(monitorBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			pfs.logger.Info().Str("directory", monitorBaseDir).Msg("Monitor base directory does not exist. No hosts with history.")
			return hostnames, nil
		}
		return nil, fmt.Errorf("failed to read monitor base directory '%s': %w", monitorBaseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			hostDirName := entry.Name()
			historyFilePath := filepath.Join(monitorBaseDir, hostDirName, fileHistoryCurrentFile)
			if _, err := os.Stat(historyFilePath); err == nil {
				// File exists, so this host has history
				if !seenHosts[hostDirName] {
					hostnames = append(hostnames, hostDirName)
					seenHosts[hostDirName] = true
				}
			} else if !os.IsNotExist(err) {
				// Some other error stating the file, log it but continue
				pfs.logger.Warn().Err(err).Str("file", historyFilePath).Msg("Error checking history file for host, skipping host.")
			}
		}
	}

	pfs.logger.Info().Int("count", len(hostnames)).Msg("Successfully retrieved hostnames with history.")
	return hostnames, nil
}

// GetAllLatestDiffResultsForURLs retrieves the latest diff result for each of the specified URLs.
func (pfs *ParquetFileHistoryStore) GetAllLatestDiffResultsForURLs(urls []string) (map[string]*models.ContentDiffResult, error) {
	results := make(map[string]*models.ContentDiffResult)
	urlHostMap := make(map[string]string) // To optimize file reads, group URLs by host

	for _, u := range urls {
		parsedURL, err := url.Parse(u)
		if err != nil {
			pfs.logger.Warn().Err(err).Str("url", u).Msg("Failed to parse URL, skipping for latest diff result.")
			continue
		}
		host := parsedURL.Host
		urlHostMap[u] = host
	}

	// Process URLs grouped by host to minimize file reads
	processedHosts := make(map[string]struct{})
	for u, host := range urlHostMap {
		if _, processed := processedHosts[host]; processed {
			// Already read this host's file, try to find the URL in existing results if logic was different
			// For this specific function, we fetch per URL, so this check is more for future optimization awareness
		}

		// Get all records for the host, which are sorted newest first by readFileHistoryRecords
		hostRecords, err := pfs.getAndSortRecordsForHost(host) // new helper or adapt existing
		if err != nil {
			pfs.logger.Warn().Err(err).Str("host", host).Msg("Could not get records for host when fetching latest diffs.")
			continue
		}

		processedHosts[host] = struct{}{}

		// Find the latest record for the specific URL that has a diff
		for _, record := range hostRecords {
			if record.URL == u && record.DiffResultJSON != nil {
				var diffResult models.ContentDiffResult
				if err := json.Unmarshal([]byte(*record.DiffResultJSON), &diffResult); err != nil {
					pfs.logger.Error().Err(err).Str("url", u).Msg("Failed to unmarshal DiffResultJSON for latest diff.")
					// Store an error or skip? For now, skip.
					break // Move to the next URL for this host
				}

				// Unmarshal ExtractedPathsJSON if available
				if record.ExtractedPathsJSON != nil && *record.ExtractedPathsJSON != "" {
					var extractedPaths []models.ExtractedPath
					if err := json.Unmarshal([]byte(*record.ExtractedPathsJSON), &extractedPaths); err != nil {
						pfs.logger.Error().Err(err).Str("url", u).Msg("Failed to unmarshal ExtractedPathsJSON for latest diff.")
						// Do not assign to diffResult.ExtractedPaths if unmarshaling fails, it will remain nil or empty
					} else {
						diffResult.ExtractedPaths = extractedPaths
					}
				}

				results[u] = &diffResult
				break // Found the latest for this URL
			}
		}
	}
	return results, nil
}

// getAndSortRecordsForHost is a new helper or adaptation of existing logic
// to get all records for a host, sorted.
func (pfs *ParquetFileHistoryStore) getAndSortRecordsForHost(host string) ([]models.FileHistoryRecord, error) {
	sanitizedHost := urlhandler.SanitizeFilename(host) // Changed SanitizeHost to SanitizeFilename
	// Construct the full path to the history file for the host.
	// It should be <storageConfig.ParquetBasePath>/monitor/<sanitizedHost>/current_history.parquet
	hostSpecificDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir, sanitizedHost)
	filePath := filepath.Join(hostSpecificDir, fileHistoryCurrentFile)

	pfs.logger.Debug().Str("host", host).Str("sanitized_host", sanitizedHost).Str("file_path", filePath).Msg("Constructed file path for getAndSortRecordsForHost")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		pfs.logger.Debug().Str("host", host).Str("file_path", filePath).Msg("History file does not exist for host in getAndSortRecordsForHost.")
		return []models.FileHistoryRecord{}, nil // Return empty slice, not an error
	}

	// Call readFileHistoryRecords which is defined in the same file and uses pfs.logger implicitly if needed,
	// or takes it as an argument. Assuming the existing readFileHistoryRecords(filePath, logger) definition.
	// If readFileHistoryRecords is a method of ParquetFileHistoryStore, it would be pfs.readFileHistoryRecords(filePath)
	// Based on the definition of readFileHistoryRecords(filePath string, logger zerolog.Logger), we pass pfs.logger.
	return readFileHistoryRecords(filePath, pfs.logger)
}

// GetAllDiffResults retrieves all diff results from all history files.
// This is a potentially expensive operation.
func (pfs *ParquetFileHistoryStore) GetAllDiffResults() ([]models.ContentDiffResult, error) {
	// Implementation of GetAllDiffResults method
	// This is a placeholder and should be implemented based on the actual requirements
	return []models.ContentDiffResult{}, nil
}
