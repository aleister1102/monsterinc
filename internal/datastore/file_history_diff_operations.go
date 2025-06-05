package datastore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

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

// GetAllDiffResults retrieves all diff results from all history files.
// This is a potentially expensive operation.
func (pfs *ParquetFileHistoryStore) GetAllDiffResults() ([]models.ContentDiffResult, error) {
	// Implementation of GetAllDiffResults method
	// This is a placeholder and should be implemented based on the actual requirements
	return []models.ContentDiffResult{}, nil
}
